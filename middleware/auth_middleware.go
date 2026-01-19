package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/SaiNageswarS/go-api-boot/logger"
	"go.uber.org/zap"
)

// APIKeyAuthMiddleware validates API key from Authorization header or X-API-Key header
func APIKeyAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := os.Getenv("API_KEY")
		if apiKey == "" {
			logger.Error("API_KEY environment variable is not set")
			http.Error(w, "Server configuration error", http.StatusInternalServerError)
			return
		}

		// Check for API key in Authorization header (Bearer token) or X-API-Key header
		authHeader := r.Header.Get("Authorization")
		apiKeyHeader := r.Header.Get("X-API-Key")

		var providedKey string
		if authHeader != "" {
			// Extract token from "Bearer <token>" format
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				providedKey = parts[1]
			} else if len(parts) == 1 {
				// If no Bearer prefix, use the whole header value
				providedKey = parts[0]
			}
		} else if apiKeyHeader != "" {
			providedKey = apiKeyHeader
		}

		if providedKey == "" {
			logger.Error("API key missing from request", zap.String("path", r.URL.Path))
			http.Error(w, "API key required. Provide it in Authorization header (Bearer <key>) or X-API-Key header", http.StatusUnauthorized)
			return
		}

		if providedKey != apiKey {
			logger.Error("Invalid API key provided", zap.String("path", r.URL.Path))
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// API key is valid, proceed to next handler
		next(w, r)
	}
}
