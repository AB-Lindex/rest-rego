package types

import "errors"

// Authentication errors
var (
	ErrAuthenticationFailed      = errors.New("authentication failed")
	ErrAuthenticationUnavailable = errors.New("authentication service unavailable")
)

// IsAuthenticationError checks if an error is an authentication error
func IsAuthenticationError(err error) bool {
	return errors.Is(err, ErrAuthenticationFailed) ||
		errors.Is(err, ErrAuthenticationUnavailable)
}
