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
	wellKnowns    []string
	wellknownList []*wellKnownData
	cache         *jwk.Cache
	JWKS          []jwk.Set
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

func New(wellKnowns, audList []string) *JWTSupport {
	j := &JWTSupport{
		wellKnowns: wellKnowns,
		audiences:  audList,
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
	slog.Debug("jwtsupport: authenticating request", "url", r.URL.Path)
	if info.Request.Auth == nil {
		slog.Debug("jwtsupport: info.Request.Auth == nil")
		return nil
		//return types.ErrNoAuth
	}
	if !strings.EqualFold(info.Request.Auth.Kind, "bearer") {
		slog.Debug("jwtsupport: not a bearer token")
		return nil
		//return types.ErrNoAuth
	}
	request := []byte(info.Request.Auth.Token)
	for _, wc := range j.wellknownList {
		ks, err := j.cache.Get(context.Background(), wc.JwksURI)
		if err != nil {
			slog.Debug("jwtsupport: failed to get jwks", "url", wc.JwksURI, "error", err)
			continue
		}
		for _, aud := range j.audiences {
			slog.Debug("jwtsupport: parsing token", "aud", aud, "keys", ks.Len())
			token, err := jwt.Parse(request,
				jwt.WithKeySet(ks),
				jwt.WithValidate(true),
				jwt.WithVerify(true),
				jwt.WithAudience(aud),
			)
			if err != nil {
				slog.Debug("jwtsupport: failed to parse token", "error", err)
				continue
			}
			fields, _ := token.AsMap(context.Background())
			info.JWT = fields
			return nil
		}
	}
	return nil
}
