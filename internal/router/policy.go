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

		// Validate result structure
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			slog.Error("router: invalid policy result type",
				"type", fmt.Sprintf("%T", result),
				"path", r.URL.Path)
			http.Error(w, "invalid policy result", http.StatusInternalServerError)
			return
		}

		// Validate that 'allow' field exists and is boolean - PREVENTS FAIL-OPEN
		allowValue, allowExists := resultMap["allow"]
		if !allowExists {
			slog.Error("router: policy result missing 'allow' field", "path", r.URL.Path)
			http.Error(w, "invalid policy result", http.StatusInternalServerError)
			return
		}

		allowBool, isBool := allowValue.(bool)
		if !isBool {
			slog.Error("router: policy 'allow' field is not boolean",
				"type", fmt.Sprintf("%T", allowValue),
				"path", r.URL.Path)
			http.Error(w, "invalid policy result", http.StatusInternalServerError)
			return
		}

		info.Result = result

		// Explicit deny check - fail closed by default
		if !allowBool {
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

		next.ServeHTTP(w, r)
	})
}
