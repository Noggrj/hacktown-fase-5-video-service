package httpauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/httpauth"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/jwt"
)

const testSecret = "test-secret-at-least-16-bytes"

func protectedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-User-Id", httpauth.UserID(r.Context()))
		w.Header().Set("X-Email", httpauth.Email(r.Context()))
		w.WriteHeader(http.StatusOK)
	})
}

func TestMiddleware_RejectsMissingHeader(t *testing.T) {
	verifier, _ := jwt.NewVerifier(testSecret)
	h := httpauth.Middleware(verifier)(protectedHandler())

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_RejectsInvalidToken(t *testing.T) {
	verifier, _ := jwt.NewVerifier(testSecret)
	h := httpauth.Middleware(verifier)(protectedHandler())

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_AcceptsValidToken_InjectsClaims(t *testing.T) {
	issuer, _ := jwt.NewIssuer(testSecret)
	verifier, _ := jwt.NewVerifier(testSecret)
	h := httpauth.Middleware(verifier)(protectedHandler())

	token, _ := issuer.Issue("user-1", "a@b.com", time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-User-Id") != "user-1" || rec.Header().Get("X-Email") != "a@b.com" {
		t.Fatalf("claims not propagated: %+v", rec.Header())
	}
}
