package httpapi

import (
	"context"
	"net/http"
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
