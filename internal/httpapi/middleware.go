package httpapi

import (
	"context"
	"net/http"
	"strings"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"

	"github.com/gigliofr/gokit/envx"
	"github.com/gigliofr/gokit/httpx"
	"github.com/gigliofr/gokit/mw"
)

type ctxKey string

const claimsKey ctxKey = "claims"

func (s *Server) authMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := httpx.BearerToken(r)
			if token == "" {
				httpx.Error(w, http.StatusUnauthorized, "token mancante")
				return
			}
			claims, err := s.tokens.Parse(token)
			if err != nil || claims.Type != "access" {
				httpx.Error(w, http.StatusUnauthorized, "token non valido")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func claimsFromCtx(ctx context.Context) *auth.Claims {
	v := ctx.Value(claimsKey)
	if c, ok := v.(*auth.Claims); ok {
		return c
	}
	return nil
}

func (s *Server) requireRoles(roles ...model.Role) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[string(r)] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFromCtx(r.Context())
			if claims == nil {
				httpx.Error(w, http.StatusUnauthorized, "non autenticato")
				return
			}
			if !allowed[claims.Role] {
				httpx.Error(w, http.StatusForbidden, "ruolo non autorizzato")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// securityHeadersMiddleware applica gli header di sicurezza (gokit/mw).
// HSTS si attiva con SECURITY_HEADERS_HSTS=true ed è inviato solo su HTTPS.
func securityHeadersMiddleware() func(http.Handler) http.Handler {
	hsts := 0
	if envx.Bool("SECURITY_HEADERS_HSTS", false) {
		hsts = 63072000
	}
	return mw.SecurityHeaders(mw.SecurityOptions{
		FrameOptions:      "DENY",
		ReferrerPolicy:    "no-referrer",
		PermissionsPolicy: "geolocation=(), microphone=(), camera=()",
		ContentSecurityPolicy: strings.Join([]string{
			"default-src 'self'",
			"script-src 'self'",
			"style-src 'self' https://fonts.googleapis.com",
			"font-src 'self' https://fonts.gstatic.com data:",
			"img-src 'self' data:",
			"connect-src 'self' http: https:",
			"frame-ancestors 'none'",
			"base-uri 'none'",
			"form-action 'self'",
		}, "; "),
		HSTSMaxAge: hsts,
	})
}

// corsMiddleware legge la configurazione CORS dall'ambiente (gokit/mw):
// CORS_ALLOWED_ORIGINS (default "*"), CORS_ALLOW_CREDENTIALS,
// CORS_ALLOWED_METHODS, CORS_ALLOWED_HEADERS. I preflight da origini non
// ammesse ricevono 403.
func corsMiddleware() func(http.Handler) http.Handler {
	return mw.CORS(mw.CORSOptions{
		AllowedOrigins:            envx.List("CORS_ALLOWED_ORIGINS", []string{"*"}),
		AllowCredentials:          envx.Bool("CORS_ALLOW_CREDENTIALS", false),
		AllowedMethods:            envx.List("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"}),
		AllowedHeaders:            envx.List("CORS_ALLOWED_HEADERS", []string{"Authorization", "Content-Type"}),
		RejectDisallowedPreflight: true,
	})
}
