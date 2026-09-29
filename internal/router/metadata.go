package router

import (
	"net/http"
	"net/url"
	"strings"
)

// protectedResourceMetadata is the RFC 9728 OAuth 2.0 Protected Resource Metadata
// document body.
type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
}

// newProtectedResourceMetadata builds the RFC 9728 metadata document for the given
// resource URL and de-duplicated issuer list.
func newProtectedResourceMetadata(resourceURL string, issuers []string) *protectedResourceMetadata {
	return &protectedResourceMetadata{
		Resource:               resourceURL,
		AuthorizationServers:   issuers,
		BearerMethodsSupported: []string{"header"},
	}
}

// buildWellKnownURL builds the RFC 9728/8414-convention well-known discovery URL by
// inserting metadataPath before the path component of resourceURL, e.g.
// resourceURL=https://host/mcp, metadataPath=/.well-known/oauth-protected-resource
// -> https://host/.well-known/oauth-protected-resource/mcp.
func buildWellKnownURL(resourceURL, metadataPath string) (string, error) {
	u, err := url.Parse(resourceURL)
	if err != nil {
		return "", err
	}
	mp := "/" + strings.Trim(metadataPath, "/")
	path := strings.TrimSuffix(u.Path, "/")
	return u.Scheme + "://" + u.Host + mp + path, nil
}

// protectedResourceHandler serves the pre-marshaled RFC 9728 metadata document.
// This route is intentionally unauthenticated (per RFC 9728); Proxy.Handler
// intercepts it ahead of proxy.mux so it never passes through
// authHandler/policyHandler.
func (proxy *Proxy) protectedResourceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(proxy.resourceMetadataJSON)
}
