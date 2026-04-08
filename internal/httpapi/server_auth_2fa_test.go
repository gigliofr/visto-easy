package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"visto-easy/internal/model"
)

func TestTwoFASetupEnableAndLogin(t *testing.T) {
	s, st, _ := newSecurityHTTPTestServer(t)
	pwd, _ := bcrypt.GenerateFromPassword([]byte("AdminPass123!"), 12)
	u, err := st.CreateUser(model.Utente{Email: "bo2fa@example.com", PasswordHash: string(pwd), Ruolo: model.RoleAdmin, Nome: "Bo", Cognome: "TwoFA"})
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	accessToken, err := s.tokens.SignAccess(u.ID, string(model.RoleAdmin))
	if err != nil {
		t.Fatalf("sign access failed: %v", err)
	}

	reqSetup := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", strings.NewReader(`{}`))
	reqSetup.Header.Set("Authorization", "Bearer "+accessToken)
	reqSetup.Header.Set("Content-Type", "application/json")
	rrSetup := httptest.NewRecorder()
	s.Router().ServeHTTP(rrSetup, reqSetup)
	if rrSetup.Code != http.StatusOK {
		t.Fatalf("2fa setup failed: got=%d body=%s", rrSetup.Code, rrSetup.Body.String())
	}

	updatedUser, err := st.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("get user after setup failed: %v", err)
	}
	if strings.TrimSpace(updatedUser.TOTPSecret) == "" {
		t.Fatalf("expected totp secret to be set after setup")
	}

	code, err := totp.GenerateCode(updatedUser.TOTPSecret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate totp code failed: %v", err)
	}

	reqEnable := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", strings.NewReader(`{"code":"`+code+`"}`))
	reqEnable.Header.Set("Authorization", "Bearer "+accessToken)
	reqEnable.Header.Set("Content-Type", "application/json")
	rrEnable := httptest.NewRecorder()
	s.Router().ServeHTTP(rrEnable, reqEnable)
	if rrEnable.Code != http.StatusOK {
		t.Fatalf("2fa enable failed: got=%d body=%s", rrEnable.Code, rrEnable.Body.String())
	}

	reqNoOTP := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"bo2fa@example.com","password":"AdminPass123!"}`))
	reqNoOTP.Header.Set("Content-Type", "application/json")
	rrNoOTP := httptest.NewRecorder()
	s.Router().ServeHTTP(rrNoOTP, reqNoOTP)
	if rrNoOTP.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized without otp: got=%d body=%s", rrNoOTP.Code, rrNoOTP.Body.String())
	}

	code2, err := totp.GenerateCode(updatedUser.TOTPSecret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate totp code for login failed: %v", err)
	}
	reqWithOTP := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"bo2fa@example.com","password":"AdminPass123!","otp":"`+code2+`"}`))
	reqWithOTP.Header.Set("Content-Type", "application/json")
	rrWithOTP := httptest.NewRecorder()
	s.Router().ServeHTTP(rrWithOTP, reqWithOTP)
	if rrWithOTP.Code != http.StatusOK {
		t.Fatalf("expected login success with otp: got=%d body=%s", rrWithOTP.Code, rrWithOTP.Body.String())
	}
}
