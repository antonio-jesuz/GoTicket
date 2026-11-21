package user

import (
	"context"
	"net/http"
	"strings"

	"goticket/internal/httpx"
)

type ctxKey struct{}

func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

// Authenticate valida o JWT. Aceita "Authorization: Bearer" e, como fallback para
// EventSource (que não envia headers), ?token= — veja ADR-004 em docs/ARCHITECTURE.md.
func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		claims, err := s.Parse(token)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, ErrUnauthorized.Error())
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
	})
}

// RequireAgent deve ser composto depois de Authenticate.
func RequireAgent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := FromContext(r.Context()); !ok || !c.IsAgent() {
			httpx.Error(w, http.StatusForbidden, "acesso restrito a agentes")
			return
		}
		next.ServeHTTP(w, r)
	})
}
