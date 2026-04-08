package httpapi

import (
	"context"
	"net/http"
	"os"
	"strings"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
)

type ctxKey string

const claimsKey ctxKey = "claims"

func (s *Server) authMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := strings.TrimSpace(r.Header.Get("Authorization"))
			if h == "" || !strings.HasPrefix(strings.ToLower(h), "bearer ") {
				writeErr(w, http.StatusUnauthorized, "token mancante")
				return
			}
			parts := strings.SplitN(h, " ", 2)
			if len(parts) != 2 {
				writeErr(w, http.StatusUnauthorized, "token non valido")
				return
			}
			token := strings.TrimSpace(parts[1])
			claims, err := s.tokens.Parse(token)
			if err != nil || claims.Type != "access" {
				writeErr(w, http.StatusUnauthorized, "token non valido")
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
				writeErr(w, http.StatusUnauthorized, "non autenticato")
				return
			}
			if !allowed[claims.Role] {
				writeErr(w, http.StatusForbidden, "ruolo non autorizzato")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.EqualFold(strings.TrimSpace(os.Getenv("SECURITY_HEADERS_HSTS")), "true") {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	allowCredentials := strings.EqualFold(strings.TrimSpace(os.Getenv("CORS_ALLOW_CREDENTIALS")), "true")
	allowedMethods := strings.TrimSpace(os.Getenv("CORS_ALLOWED_METHODS"))
	if allowedMethods == "" {
		allowedMethods = "GET,POST,PATCH,DELETE,OPTIONS"
	}
	allowedHeaders := strings.TrimSpace(os.Getenv("CORS_ALLOWED_HEADERS"))
	if allowedHeaders == "" {
		allowedHeaders = "Authorization,Content-Type"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			w.Header().Add("Vary", "Origin")
			if isOriginAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
				if allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			} else if r.Method == http.MethodOptions {
				writeErr(w, http.StatusForbidden, "origine non consentita")
				return
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseAllowedOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{"*"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func isOriginAllowed(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == "*" || strings.EqualFold(origin, candidate) {
			return true
		}
	}
	return false
}
