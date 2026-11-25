// Package auth adatta github.com/gigliofr/gokit/authx ai token di visto-easy.
package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/gigliofr/gokit/authx"
)

// Claims dei token di visto-easy. Il formato JSON (uid, role, typ, jti) è
// invariato, quindi i token emessi prima della migrazione restano validi.
type Claims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	authx.BaseClaims
}

type TokenManager struct {
	tm *authx.TokenManager[*Claims]
}

func NewTokenManager(secret string) (*TokenManager, error) {
	tm, err := authx.NewTokenManager(secret, func() *Claims { return &Claims{} },
		authx.WithTTL(15*time.Minute, 7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	return &TokenManager{tm: tm}, nil
}

func (m *TokenManager) SignAccess(userID, role string) (string, error) {
	return m.tm.Issue(&Claims{UserID: userID, Role: role}, authx.Access)
}

func (m *TokenManager) SignRefresh(userID, role string) (string, error) {
	return m.SignRefreshWithJTI(userID, role, "")
}

// SignRefreshWithJTI emette un refresh token con l'identificativo indicato
// (generato se vuoto), usato per la revoca delle sessioni.
func (m *TokenManager) SignRefreshWithJTI(userID, role, jti string) (string, error) {
	c := &Claims{UserID: userID, Role: role}
	c.ID = strings.TrimSpace(jti)
	return m.tm.Issue(c, authx.Refresh)
}

// Parse verifica firma (solo HS256) e scadenza e accetta sia access sia
// refresh token: il chiamante controlla claims.Type.
func (m *TokenManager) Parse(tokenStr string) (*Claims, error) {
	c, err := m.tm.Verify(tokenStr, authx.Access)
	if errors.Is(err, authx.ErrWrongTokenType) {
		c, err = m.tm.Verify(tokenStr, authx.Refresh)
	}
	return c, err
}
