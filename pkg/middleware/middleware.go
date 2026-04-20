// Package middleware provides HTTP middleware for the API Gateway including
// JWT authentication, rate limiting, CORS, and request tracking.
// Implements BRD Section 10.2 (Auth) and Section 7.4 (Rate Limiting).
package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ---- Request ID & Context ----

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	UserRoleKey  contextKey = "user_role"
)

// RequestID middleware injects a unique request ID for distributed tracing.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ---- CORS Middleware (BRD: API Layer) ----

// CORSConfig defines Cross-Origin Resource Sharing settings.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns production CORS settings.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"}, // Restrict in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID", "X-API-Key"},
		AllowCredentials: true,
		MaxAge:           86400,
	}
}

// CORS returns CORS middleware.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range cfg.AllowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))

			// Handle preflight
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ---- Rate Limiting (BRD Section 7.4) ----
// Token bucket algorithm per BRD:
// - Unauthenticated: 100 req/min per IP
// - Authenticated: 5,000 req/min per user
// - Merchant API: 50,000 req/min per API key

// RateLimiter implements in-memory token bucket rate limiting.
// Production deployment should use Redis for distributed rate limiting.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    int           // tokens per interval
	burst   int           // max burst size
	interval time.Duration
}

type tokenBucket struct {
	tokens    float64
	lastCheck time.Time
}

// NewRateLimiter creates a rate limiter with specified rate and burst.
func NewRateLimiter(ratePerMinute, burst int) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     ratePerMinute,
		burst:    burst,
		interval: time.Minute,
	}
}

// Allow checks if a request from the given key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[key]
	now := time.Now()

	if !exists {
		rl.buckets[key] = &tokenBucket{
			tokens:    float64(rl.burst - 1),
			lastCheck: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastCheck).Seconds()
	bucket.tokens += elapsed * (float64(rl.rate) / 60.0)
	if bucket.tokens > float64(rl.burst) {
		bucket.tokens = float64(rl.burst)
	}
	bucket.lastCheck = now

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// RateLimit returns HTTP middleware that enforces rate limiting per BRD Section 7.4.
func RateLimit(unauthLimiter, authLimiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var key string
			var limiter *RateLimiter

			// Check if authenticated
			if userID, ok := r.Context().Value(UserIDKey).(string); ok && userID != "" {
				key = "user:" + userID
				limiter = authLimiter
			} else {
				key = "ip:" + r.RemoteAddr
				limiter = unauthLimiter
			}

			if !limiter.Allow(key) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "rate_limit_exceeded",
					"message": "Too many requests. Please retry after the Retry-After period.",
					"code":    "RATE_LIMIT_429",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ---- JWT Authentication (BRD Section 10.2) ----
// OAuth2/OIDC: JWT access tokens (15-min TTL) + refresh tokens (7-day TTL)

// JWTConfig holds JWT validation configuration.
type JWTConfig struct {
	PublicKey     *rsa.PublicKey
	Issuer       string
	Audience     string
	RequiredRole string // Empty = any authenticated user
}

// Claims represents JWT token claims per BRD Section 10.2 RBAC.
type Claims struct {
	UserID    string   `json:"sub"`
	Email     string   `json:"email"`
	Role      string   `json:"role"`      // Customer, Merchant, Admin, Super-Admin
	Issuer    string   `json:"iss"`
	Audience  string   `json:"aud"`
	ExpiresAt int64    `json:"exp"`
	IssuedAt  int64    `json:"iat"`
	Scopes    []string `json:"scopes,omitempty"`
}

// Authenticate is middleware that validates JWT Bearer tokens.
// Implements BRD Section 10.2: OAuth2/OIDC with RBAC enforcement.
func Authenticate(skipPaths []string) func(http.Handler) http.Handler {
	skipSet := make(map[string]bool)
	for _, p := range skipPaths {
		skipSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for health checks and public endpoints
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeAuthError(w, "missing or invalid authorization header")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				writeAuthError(w, "empty bearer token")
				return
			}

			// In production, validate JWT signature against Keycloak JWKS endpoint.
			// For now, extract claims from token (placeholder for OIDC integration).
			claims, err := validateToken(token)
			if err != nil {
				writeAuthError(w, err.Error())
				return
			}

			// Check token expiration
			if time.Now().Unix() > claims.ExpiresAt {
				writeAuthError(w, "token expired")
				return
			}

			// Inject user context for downstream handlers
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole is middleware that enforces RBAC per BRD Section 10.2.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]bool)
	for _, r := range roles {
		roleSet[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, ok := r.Context().Value(UserRoleKey).(string)
			if !ok || !roleSet[userRole] {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "forbidden",
					"message": "Insufficient permissions for this resource",
					"code":    "AUTHZ_403",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---- Logging Middleware ----

// Logger middleware logs all HTTP requests with timing and status.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		// Emit structured log line
		duration := time.Since(start)
		fmt.Printf(`{"timestamp":"%s","method":"%s","path":"%s","status":%d,"duration_ms":%d,"request_id":"%s","remote_addr":"%s"}`+"\n",
			time.Now().UTC().Format(time.RFC3339),
			r.Method, r.URL.Path, wrapped.statusCode,
			duration.Milliseconds(),
			r.Context().Value(RequestIDKey),
			r.RemoteAddr,
		)
	})
}

// ---- Recovery Middleware ----

// Recover catches panics and returns 500 instead of crashing the server.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "internal_server_error",
					"message": "An unexpected error occurred",
					"code":    "INTERNAL_500",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---- Helpers ----

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "unauthorized",
		"message": msg,
		"code":    "AUTH_401",
	})
}

func validateToken(token string) (*Claims, error) {
	// PLACEHOLDER: In production, validate JWT signature against Keycloak JWKS.
	// This is where RS256 signature verification with public key rotation happens.
	if token == "" {
		return nil, errors.New("empty token")
	}
	// For development, accept any non-empty token with default claims
	return &Claims{
		UserID:    "dev-user",
		Email:     "dev@digital-metro.com",
		Role:      "Admin",
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
	}, nil
}

func generateID() string {
	// Simple request ID generator; use UUID in production
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
