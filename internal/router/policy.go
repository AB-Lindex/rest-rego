package router

import (
	"log/slog"
	"net/http"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

func (proxy *Proxy) policyHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := types.GetInfo(r)
		if info == nil {
			http.Error(w, "internal error - missing context", http.StatusInternalServerError)
			return
		}

		result, err := proxy.validator.Validate(proxy.requestName, info)
		if err != nil {
			slog.Error("router: request validation error", "error", err)
			result = err.Error()
			// http.Error(w, "internal validator error", http.StatusInternalServerError)
			// return
		}

		info.Result = result

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			http.Error(w, "internal validator error", http.StatusInternalServerError)
			return
		}
		if resultMap["allow"] == false {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if url, ok := resultMap["url"].(string); ok && url != "" {
			info.URL = url
		}

		next.ServeHTTP(w, r)
	})
}
