package types

import "net/http"

// AuthProvider is the interface for the authentication provider
type AuthProvider interface {
	Authenticate(*Info, *http.Request) error
}

// Validator is the interface for the policy validator
type Validator interface {
	Validate(name string, input interface{}) (interface{}, error)
}

// AuthChallenger is optionally implemented by AuthProviders that require
// a specific WWW-Authenticate challenge header on 401 responses.
type AuthChallenger interface {
	WWWAuthenticate() string
}

// IssuerProvider is optionally implemented by AuthProviders that can advertise
// one or more OAuth 2.0 / OIDC issuer URLs for RFC 9728 protected-resource
// metadata.
type IssuerProvider interface {
	Issuers() []string
}
