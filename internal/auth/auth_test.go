package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestLegacyTokensStillValid(t *testing.T) {
	type legacy struct {
		UserID string `json:"uid"`
		Role   string `json:"role"`
		Type   string `json:"typ"`
		jwt.RegisteredClaims
	}
	now := time.Now()
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy{UserID: "u1", Role: "ADMIN", Type: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{ID: "jti-1", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}}).SignedString([]byte(secret))
	tm, _ := NewTokenManager(secret)
	c, err := tm.Parse(tok)
	if err != nil || c.UserID != "u1" || c.Role != "ADMIN" || c.Type != "refresh" || c.ID != "jti-1" {
		t.Fatalf("token precedente: %v %+v", err, c)
	}
}

func TestSignAndParse(t *testing.T) {
	tm, _ := NewTokenManager(secret)
	a, _ := tm.SignAccess("u2", "RICHIEDENTE")
	c, err := tm.Parse(a)
	if err != nil || c.Type != "access" || c.ExpiresAt.Sub(c.IssuedAt.Time) != 15*time.Minute {
		t.Fatalf("access: %v %+v", err, c)
	}
	r, _ := tm.SignRefreshWithJTI("u2", "RICHIEDENTE", " sess-9 ")
	c, err = tm.Parse(r)
	if err != nil || c.Type != "refresh" || c.ID != "sess-9" {
		t.Fatalf("refresh: %v %+v", err, c)
	}
	if _, err := tm.Parse(a + "x"); err == nil {
		t.Fatal("firma alterata accettata")
	}
	if _, err := NewTokenManager("corto"); err == nil {
		t.Fatal("segreto corto accettato")
	}
}
