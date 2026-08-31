package httpcors

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newNextOK() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestMiddleware_AllowAll_EchoesOrigin(t *testing.T) {
	h := Middleware([]string{"*"})(newNextOK())
	req := httptest.NewRequest(http.MethodGet, "/videos", nil)
	req.Header.Set("Origin", "http://localhost:8085")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (request must still reach next handler)", rec.Code)
	}
}

func TestMiddleware_AllowList_MatchingOrigin(t *testing.T) {
	h := Middleware([]string{"http://localhost:8085"})(newNextOK())
	req := httptest.NewRequest(http.MethodGet, "/videos", nil)
	req.Header.Set("Origin", "http://localhost:8085")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8085" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want http://localhost:8085", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, want Origin", got)
	}
}

func TestMiddleware_AllowList_NonMatchingOrigin_NoHeader(t *testing.T) {
	h := Middleware([]string{"http://localhost:8085"})(newNextOK())
	req := httptest.NewRequest(http.MethodGet, "/videos", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty for disallowed origin", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (CORS header omission, not a block — browser enforces it)", rec.Code)
	}
}

func TestMiddleware_Preflight_ShortCircuitsWithNoContent(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { nextCalled = true })
	h := Middleware([]string{"*"})(next)

	req := httptest.NewRequest(http.MethodOptions, "/videos", nil)
	req.Header.Set("Origin", "http://localhost:8085")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if nextCalled {
		t.Fatal("preflight OPTIONS must not reach the next handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("Access-Control-Allow-Methods must be set on preflight response")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("Access-Control-Allow-Headers must be set on preflight response")
	}
}

func TestMiddleware_NoOriginHeader_PassesThroughUnaffected(t *testing.T) {
	h := Middleware([]string{"http://localhost:8085"})(newNextOK())
	req := httptest.NewRequest(http.MethodGet, "/videos", nil) // no Origin header, e.g. curl/Postman
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty when no Origin header present", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
