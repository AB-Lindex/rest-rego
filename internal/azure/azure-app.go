package azure

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/AB-Lindex/rest-rego/internal/types"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

// AzureAuthProvider is an implementation of the AuthProvider interface for Azure
type AzureAuthProvider struct {
	tenant     string
	header     string
	permissive bool // true = treat auth failures as anonymous
}

// New creates a new instance of the AzureAuthProvider
func New(tenant, authHeader string, permissive bool) *AzureAuthProvider {
	slog.Info("azure: creating auth provider", "tenant", tenant)
	return &AzureAuthProvider{
		tenant:     tenant,
		header:     authHeader,
		permissive: permissive,
	}
}

// Authenticate authenticates the request
func (az *AzureAuthProvider) Authenticate(info *types.Info, r *http.Request) error {
	// Case 1: No bearer token → Always allow as anonymous
	bearerToken, ok := info.GetBearerToken(r, az.header)
	if len(bearerToken) == 0 || !ok {
		slog.Debug("azure: no bearer token, treating as anonymous")
		return nil
	}

	// Case 2: Token malformed
	token, err := jwt.Parse(bearerToken, jwt.WithVerify(false))
	if err != nil {
		slog.Warn("azure: failed to parse JWT", "error", err)

		if !az.permissive {
			return types.ErrAuthenticationFailed
		}

		slog.Debug("azure: treating malformed token as anonymous (permissive mode)")
		return nil
	}

	appid := getTokenString(token, "appid")
	tid := getTokenString(token, "tid")

	// Case 3: Missing required claims
	if appid == "" {
		slog.Warn("azure: missing appid claim")

		if !az.permissive {
			return types.ErrAuthenticationFailed
		}

		slog.Debug("azure: treating token without appid as anonymous (permissive mode)")
		return nil
	}

	// Case 4: Wrong tenant
	if !strings.EqualFold(tid, az.tenant) {
		slog.Warn("azure: tenant mismatch", "expected", az.tenant, "got", tid)

		if !az.permissive {
			return types.ErrAuthenticationFailed
		}

		slog.Debug("azure: treating wrong tenant as anonymous (permissive mode)")
		return nil
	}

	// Case 5: Fetch app from Graph API
	info.Request.ID = appid
	user := getApp(appid, string(bearerToken))

	if user == nil {
		slog.Error("azure: failed to fetch app from Graph API", "appid", appid)

		// Always fail on Graph API errors (system unavailable)
		// Don't fail open even in permissive mode
		return types.ErrAuthenticationUnavailable
	}

	info.User = user
	slog.Info("azure: authentication successful", "appid", appid, "tenant", tid)
	return nil
}
