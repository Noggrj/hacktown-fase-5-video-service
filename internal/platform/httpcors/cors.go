// Package httpcors provides a minimal CORS middleware. Hand-rolled instead
// of pulling in github.com/go-chi/cors — the same "too small to justify a
// new dependency" call already made for internal/platform/httpauth (see
// its package doc), and this service only needs to answer a browser
// preflight and echo one header, not the full CORS spec surface.
package httpcors

import "net/http"

// Middleware allows cross-origin requests from the given origins. Only
// relevant for browsers — server-to-server calls (curl, Postman, other
// services) ignore CORS entirely, so this never blocks them.
//
// allowedOrigins may contain "*" to allow any origin (the default in
// development). Authorization is sent as a bearer header, not a cookie,
// so "*" is safe here — there's no credentialed (cookie/TLS-client-cert)
// request to protect against.
func Middleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := false
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
			continue
		}
		origins[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := origins[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, traceparent")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
