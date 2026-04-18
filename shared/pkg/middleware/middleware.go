package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ride-hailing/shared/pkg/jwt"
	redispkg "github.com/ride-hailing/shared/pkg/redis"
	"github.com/rs/zerolog/log"
)

type contextKey string

const ClaimsKey contextKey = "claims"

// Logger logs method, path, status, and duration for every request.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.status).
			Dur("duration", time.Since(start)).
			Msg("request")
	})
}

// Auth validates the Bearer JWT and stores claims in context.
func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, found := strings.CutPrefix(header, "Bearer ")
			if !found || token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			claims, err := jwt.Validate(secret, token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFrom extracts JWT claims from context.
func ClaimsFrom(ctx context.Context) *jwt.Claims {
	c, _ := ctx.Value(ClaimsKey).(*jwt.Claims)
	return c
}

// Idempotency checks X-Idempotency-Key; returns cached response if key seen.
func Idempotency(rc *redispkg.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}
			key := r.Header.Get("X-Idempotency-Key")
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			cacheKey := "idem:" + key
			var cached string
			if err := rc.Get(r.Context(), cacheKey, &cached); err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotency-Replayed", "true")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(cached))
				return
			}
			rw := &capturingWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r)
			if rw.status < 300 {
				_ = rc.SetRaw(r.Context(), cacheKey, string(rw.body), 24*time.Hour)
			}
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

type capturingWriter struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (cw *capturingWriter) WriteHeader(code int) {
	cw.status = code
	cw.ResponseWriter.WriteHeader(code)
}

func (cw *capturingWriter) Write(b []byte) (int, error) {
	cw.body = append(cw.body, b...)
	return cw.ResponseWriter.Write(b)
}
