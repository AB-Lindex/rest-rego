package types

import (
	"context"
	"encoding/base64"
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
	Method  string                 `json:"method"`
	Path    []string               `json:"path"`
	Headers map[string]interface{} `json:"headers"`
	Auth    *RequestAuth           `json:"auth"`
	Size    int64                  `json:"size"`
	ID      string                 `json:"id,omitempty"`
}

type RequestAuth struct {
	Kind     string `json:"kind,omitempty"`
	Token    string `json:"token,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

// type JWTInfo struct {
// 	Header  interface{} `json:"header,omitempty"`
// 	Payload interface{} `json:"payload,omitempty"`
// }

// NewInfo creates a new instance of the Info based on the request
func NewInfo(r *http.Request, authKey string) *Info {
	i := new(Info)
	i.Request.Method = r.Method
	i.Request.Path = strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	i.Request.Size = r.ContentLength
	i.URL = r.URL.String()

	i.Request.Headers = make(map[string]interface{})
	for k, v := range r.Header {
		if len(v) == 1 {
			i.Request.Headers[k] = v[0]
		} else {
			i.Request.Headers[k] = v
		}
	}

	if authHdr, ok := i.Request.Headers[authKey]; ok {
		a := &RequestAuth{}
		parts := strings.SplitN(authHdr.(string), " ", 2)
		if len(parts) == 2 {
			a.Kind = parts[0]
			a.Token = strings.TrimSpace(parts[1])
		} else {
			a.Token = parts[0]
		}
		if strings.EqualFold(a.Kind, "basic") {
			if userpwd, err := base64.StdEncoding.DecodeString(a.Token); err == nil {
				parts = strings.SplitN(string(userpwd), ":", 2)
				if len(parts) >= 1 {
					a.User = parts[0]
				}
				if len(parts) >= 2 {
					a.Password = parts[1]
				}
			}
		}
		i.Request.Auth = a
	}

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
