package jwtsupport

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AB-Lindex/go-resthelp"
	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type JWTSupport struct {
	audiences     []string
	audienceKey   string
	authKind      string
	wellKnowns    []string
	wellknownList []*wellKnownData
	cache         *jwk.Cache
	JWKS          []jwk.Set
	permissive    bool // true = treat auth failures as anonymous
}

var algConverter = make(map[string]jwa.KeyAlgorithm)

func getAlgorithm(name string) jwa.KeyAlgorithm {
	if alg, ok := algConverter[name]; ok {
		return alg
	}
	alg := jwa.KeyAlgorithmFrom(name)
	algConverter[name] = alg
	return alg
}

type wellKnownData struct {
	JwksURI             string   `json:"jwks_uri"`
	SupportedAlgorithms []string `json:"id_token_signing_alg_values_supported"`
}

// PostFetch is a function that is called after the JWKS is fetched from the
// remote server. This is needed for those providers that do not include the
// algorithm in the JWKS, but instead provide a list of supported algorithms
// in the well-known document.
//
// This function will add the supported algorithms to the keys that do not
// have an algorithm set.
func (wkd *wellKnownData) PostFetch(url string, set jwk.Set) (jwk.Set, error) {
	newset := jwk.NewSet()
	// fmt.Println("--postfetch--start--", url)
	for i := range set.Len() {
		if key, ok := set.Key(i); ok {
			// kid, _ := key.Get(jwk.KeyIDKey)
			if key.Algorithm().String() != "" {
				// key already has an algorithm, just add it
				// fmt.Println("-- copy key", kid)
				newset.AddKey(key)
			} else {
				// key has no algorithm, convert it to all supported algorithms
				for _, alg := range wkd.SupportedAlgorithms {
					// fmt.Println("-- set alg on key", kid, "to", alg)
					key.Set(jwk.AlgorithmKey, getAlgorithm(alg))
					newset.AddKey(key)
				}
			}
		}
	}
	// fmt.Println("--postfetch--end--")
	// fmt.Println()
	return newset, nil
}

func New(wellKnowns []string, audKey string, audList []string, kind string, permissive bool) *JWTSupport {
	j := &JWTSupport{
		wellKnowns:  wellKnowns,
		audienceKey: audKey,
		audiences:   audList,
		authKind:    kind,
		permissive:  permissive,
	}

	j.LoadWellKnowns()
	j.LoadJWKS()

	if len(j.JWKS) == 0 {
		slog.Error("jwtsupport: no JWKS loaded")
		os.Exit(1)
	}
	if len(j.audiences) == 0 {
		slog.Error("jwtsupport: no audiences to match")
		os.Exit(1)
	}

	return j
}

func (j *JWTSupport) LoadWellKnowns() {
	for _, wellKnown := range j.wellKnowns {
		if wellKnown == "" {
			continue
		}
		slog.Debug("jwtsupport: loading well-known", "url", wellKnown)
		helper := resthelp.New()
		req, err := helper.Get(wellKnown)
		if err != nil {
			slog.Error("jwtsupport: failed to init well-known", "url", wellKnown, "error", err)
			continue
		}

		resp, err := req.Do()
		if err != nil {
			slog.Error("jwtsupport: failed to get well-known", "url", wellKnown, "error", err)
			continue
		}

		var wc wellKnownData
		if err = resp.ParseJSON(&wc); err != nil {
			slog.Error("jwtsupport: failed to parse well-known", "url", wellKnown, "error", err)
			continue
		}
		j.wellknownList = append(j.wellknownList, &wc)
	}
}

func (j *JWTSupport) LoadJWKS() {

	j.cache = jwk.NewCache(context.Background(),
		jwk.WithRefreshWindow(2*time.Minute),
		// jwk.WithRefreshWindow(24*time.Hour),
	)

	for _, wk := range j.wellknownList {
		err := j.cache.Register(wk.JwksURI, jwk.WithPostFetcher(wk)) //, jwk.WithPostFetcher(postfetch))
		if err != nil {
			slog.Error("jwtsupport: failed to register jwks", "url", wk.JwksURI, "error", err)
			continue
		}

		_, err = j.cache.Get(context.Background(), wk.JwksURI)
		if err != nil {
			slog.Error("jwtsupport: failed to get jwks", "url", wk.JwksURI, "error", err)
			continue
		}
		cachedset := jwk.NewCachedSet(j.cache, wk.JwksURI)
		slog.Info("jwtsupport: loaded jwks", "url", wk.JwksURI, "keys", cachedset.Len())
		j.JWKS = append(j.JWKS, cachedset)
	}
}

func (j *JWTSupport) Authenticate(info *types.Info, r *http.Request) error {
	// Case 1: No authentication header → Always allow as anonymous
	if info.Request.Auth == nil {
		slog.Debug("jwtsupport: no authentication header, treating as anonymous")
		return nil
	}

	// Case 2: Wrong authentication kind
	if !strings.EqualFold(info.Request.Auth.Kind, j.authKind) {
		slog.Debug("jwtsupport: incorrect auth kind", "expected", j.authKind, "got", info.Request.Auth.Kind)

		if !j.permissive {
			slog.Warn("jwtsupport: rejecting request with wrong auth kind (strict mode)")
			return types.ErrAuthenticationFailed
		}

		// Permissive mode: treat as anonymous
		slog.Debug("jwtsupport: treating wrong auth kind as anonymous (permissive mode)")
		return nil
	}

	request := []byte(info.Request.Auth.Token)
	lastError := error(nil)

	// Try to validate token against all configured issuers
	for _, wc := range j.wellknownList {
		ks, err := j.cache.Get(context.Background(), wc.JwksURI)
		if err != nil {
			slog.Warn("jwtsupport: failed to fetch JWKS", "url", wc.JwksURI, "error", err)
			lastError = err
			continue
		}

		for _, aud := range j.audiences {
			slog.Debug("jwtsupport: validating token", "issuer", wc.JwksURI, "aud", aud)

			var options []jwt.ParseOption
			if ks.Len() == 1 {
				if key, ok := ks.Key(0); ok {
					options = append(options, jwt.WithKey(key.Algorithm(), key))
				}
			} else {
				options = append(options, jwt.WithKeySet(ks))
			}

			options = append(options, jwt.WithValidate(true))
			options = append(options, jwt.WithVerify(true))

			if j.audienceKey == "aud" {
				options = append(options, jwt.WithAudience(aud))
			} else {
				options = append(options, jwt.WithClaimValue(j.audienceKey, aud))
			}

			token, err := jwt.Parse(request, options...)
			if err != nil {
				slog.Debug("jwtsupport: token validation failed", "aud", aud, "error", err)
				lastError = err
				continue
			}

			// SUCCESS: Valid token
			fields, _ := token.AsMap(context.Background())
			info.JWT = fields
			slog.Info("jwtsupport: authentication successful", "aud", aud)
			return nil
		}
	}

	// Case 3: Token validation failed for all issuers
	if lastError != nil {
		if !j.permissive {
			// Strict mode: check if it's a system error vs validation error
			errStr := lastError.Error()
			if strings.Contains(errStr, "failed to fetch") ||
				strings.Contains(errStr, "connection") ||
				strings.Contains(errStr, "timeout") {
				slog.Error("jwtsupport: authentication system unavailable (strict mode)", "error", lastError)
				return types.ErrAuthenticationUnavailable
			}

			slog.Warn("jwtsupport: token validation failed, rejecting (strict mode)", "error", lastError)
			return types.ErrAuthenticationFailed
		}

		// Permissive mode: treat validation failure as anonymous
		slog.Debug("jwtsupport: token validation failed, treating as anonymous (permissive mode)", "error", lastError)
		return nil
	}

	// Case 4: No well-known endpoints configured
	slog.Error("jwtsupport: no well-known endpoints configured")
	return types.ErrAuthenticationUnavailable
}
