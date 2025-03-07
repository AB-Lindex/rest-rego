package types

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey int

const ctxInfoKey ctxKey = 0

// Info is the request information
type Info struct {
	Request RequestInfo `json:"request"`
	JWT     interface{} `json:"jwt,omitempty"`
	User    interface{} `json:"user,omitempty,omitdefault"`
	Result  interface{} `json:"result,omitempty,omitdefault"`

	URL string `json:"-"`
}

// RequestInfo is the request information for the rego-policy
type RequestInfo struct {
	Method string   `json:"method"`
	Path   []string `json:"path"`
	Size   int64    `json:"size"`
	ID     string   `json:"id,omitempty"`
}

// type JWTInfo struct {
// 	Header  interface{} `json:"header,omitempty"`
// 	Payload interface{} `json:"payload,omitempty"`
// }

// NewInfo creates a new instance of the Info based on the request
func NewInfo(r *http.Request) *Info {
	i := new(Info)
	i.Request.Method = r.Method
	i.Request.Path = strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	i.Request.Size = r.ContentLength
	i.URL = r.URL.String()

	return i
}

// RequestWithInfo adds the Info to the request context
func (info *Info) RequestWithInfo(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), ctxInfoKey, info)
	return r.WithContext(ctx)
}

// GetInfo returns the Info from the request context
func GetInfo(r *http.Request) *Info {
	info, _ := r.Context().Value(ctxInfoKey).(*Info)
	return info
}

// GetBearerToken returns the bearer token from the request
func (info *Info) GetBearerToken(r *http.Request, authHeader string) ([]byte, bool) {
	authValue := r.Header.Get(authHeader)
	authParts := strings.SplitN(authValue, " ", 2)
	if len(authParts) != 2 {
		// http.Error(w, "invalid authorization header", http.StatusBadRequest)
		return nil, false
	}
	if !strings.EqualFold(authParts[0], "bearer") {
		// http.Error(w, "only bearer-authorization allowed", http.StatusBadRequest)
		return nil, false
	}
	return []byte(authParts[1]), true
}
