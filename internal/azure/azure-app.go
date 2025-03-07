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
	tenant string
	header string
}

// New creates a new instance of the AzureAuthProvider
func New(tenant, authHeader string) *AzureAuthProvider {
	slog.Info("azure: creating auth provider", "tenant", tenant)
	return &AzureAuthProvider{
		tenant: tenant,
		header: authHeader,
	}
}

// Authenticate authenticates the request
func (az *AzureAuthProvider) Authenticate(info *types.Info, r *http.Request) error {
	bearerToken, ok := info.GetBearerToken(r, az.header)
	if len(bearerToken) == 0 || !ok {
		return nil
	}

	token, err := jwt.Parse(bearerToken, jwt.WithVerify(false))
	if err != nil {
		// http.Error(w, "invalid token", http.StatusUnauthorized)
		return nil
	}

	appid := getTokenString(token, "appid")
	tid := getTokenString(token, "tid")

	info.Request.ID = appid

	if appid == "" || !strings.EqualFold(tid, az.tenant) {
		// http.Error(w, "token not regonized", http.StatusUnauthorized)
		return nil
	}

	// tokenMap, err := token.AsMap(context.Background())
	// if err != nil {
	// 	// http.Error(w, "internal error", http.StatusInternalServerError)
	// 	return nil
	// }

	// info.JWT = tokenMap

	info.User = getApp(appid, string(bearerToken))

	return nil
}
