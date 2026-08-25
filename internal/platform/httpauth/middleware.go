// Package httpauth is the chi-agnostic middleware every fiapx-* service
// uses to protect routes with the JWT issued by auth-service. It is
// intentionally small enough to duplicate across services rather than
// pulling in a shared module for a single ~40-line file.
package httpauth

import (
	"context"
	"net/http"
	"strings"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/jwt"
)

type ctxKey string

const (
	userIDKey ctxKey = "userID"
	emailKey  ctxKey = "email"
)

// Middleware rejects requests without a valid "Authorization: Bearer
// <token>" header, and otherwise injects the token's claims into the
// request context for downstream handlers.
func Middleware(verifier *jwt.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
				return
			}
			claims, err := verifier.Verify(token)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, emailKey, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserID reads the authenticated user's id from a request context
// previously passed through Middleware.
func UserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// Email reads the authenticated user's email from a request context
// previously passed through Middleware.
func Email(ctx context.Context) string {
	v, _ := ctx.Value(emailKey).(string)
	return v
}
