package middleware

import (
	"net/http"
	"os"
	"strings"
)

var apiKeys map[string]struct{}

func LoadAPIKeys() {
	raw := os.Getenv("REGISTRY_API_KEYS")
	apiKeys = make(map[string]struct{})
	for _, k := range strings.Split(raw, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			apiKeys[k] = struct{}{}
		}
	}
}

func RequireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		key, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || key == "" || len(apiKeys) == 0 {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if _, valid := apiKeys[key]; !valid {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
