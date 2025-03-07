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
