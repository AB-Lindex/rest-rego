package router

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

func (proxy *Proxy) policyHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := types.GetInfo(r)
		if info == nil {
			slog.Error("router: missing request context")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Explicit error handling - fail closed on evaluation errors
		result, err := proxy.validator.Validate(proxy.requestName, info)
		if err != nil {
			slog.Error("router: policy evaluation failed",
				"error", err,
				"path", r.URL.Path,
				"method", r.Method,
				"id", info.Request.ID)
			http.Error(w, "policy evaluation error", http.StatusInternalServerError)
			return // EXPLICIT FAIL CLOSED
		}

		// An undefined result (nil) is treated as deny, like an undefined 'allow' rule
		var resultMap map[string]interface{}
		if result != nil {
			var ok bool
			resultMap, ok = result.(map[string]interface{})
			if !ok {
				slog.Error("router: invalid policy result type",
					"type", fmt.Sprintf("%T", result),
					"path", r.URL.Path)
				http.Error(w, "invalid policy result", http.StatusInternalServerError)
				return
			}
		}

		// Undefined 'allow' means deny; a defined non-boolean is a policy error - PREVENTS FAIL-OPEN
		allowBool := false
		if allowValue, allowExists := resultMap["allow"]; !allowExists {
			slog.Warn("router: policy 'allow' is undefined - denying", "path", r.URL.Path)
		} else if b, isBool := allowValue.(bool); !isBool {
			slog.Error("router: policy 'allow' field is not boolean",
				"type", fmt.Sprintf("%T", allowValue),
				"path", r.URL.Path)
			http.Error(w, "invalid policy result", http.StatusInternalServerError)
			return
		} else {
			allowBool = b
		}

		info.Result = result

		// Explicit deny check - fail closed by default
		if !allowBool {
			if info.Request.Auth == nil && proxy.resourceMetadataURL != "" {
				slog.Info("router: anonymous request denied by policy",
					"path", r.URL.Path,
					"method", r.Method,
					"id", info.Request.ID)
				w.Header().Set("WWW-Authenticate", proxy.wwwAuthenticateChallenge())
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			slog.Info("router: access denied by policy",
				"path", r.URL.Path,
				"method", r.Method,
				"id", info.Request.ID)
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}

		// Handle optional URL rewriting
		if url, ok := resultMap["url"].(string); ok && url != "" {
			info.URL = url
			slog.Debug("router: rewriting URL", "original", r.URL.Path, "new", url)
		}

		// Handle optional custom metric label values
		if labels, ok := resultMap["labels"].(map[string]interface{}); ok {
			result := make(map[string]string, len(labels))
			for name, value := range labels {
				if s, ok := value.(string); ok {
					result[name] = s
				}
			}
			info.Labels = result
		}

		next.ServeHTTP(w, r)
	})
}
