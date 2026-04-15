package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"visto-easy/internal/model"
)

func TestForgotAndResetPasswordFlow(t *testing.T) {
	s, st, _ := newSecurityHTTPTestServer(t)
	pwd, _ := bcrypt.GenerateFromPassword([]byte("OldPass123!"), 12)
	u, err := st.CreateUser(model.Utente{Email: "reset@example.com", PasswordHash: string(pwd), Ruolo: model.RoleRichiedente, Nome: "Reset", Cognome: "User", Attivo: true, EmailVerificata: true})
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	reqForgot := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", strings.NewReader(`{"email":"reset@example.com"}`))
	reqForgot.Header.Set("Content-Type", "application/json")
	rrForgot := httptest.NewRecorder()
	s.Router().ServeHTTP(rrForgot, reqForgot)
	if rrForgot.Code != http.StatusOK {
		t.Fatalf("forgot password should return 200: got=%d body=%s", rrForgot.Code, rrForgot.Body.String())
	}

	if len(st.passwordResetTokens) != 1 {
		t.Fatalf("expected one reset token stored, got=%d", len(st.passwordResetTokens))
	}
	var resetToken string
	for token, rec := range st.passwordResetTokens {
		if rec.UserID == u.ID {
			resetToken = token
			break
		}
	}
	if resetToken == "" {
		t.Fatalf("reset token for created user not found")
	}

	reqReset := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", strings.NewReader(`{"token":"`+resetToken+`","new_password":"NewPass456!"}`))
	reqReset.Header.Set("Content-Type", "application/json")
	rrReset := httptest.NewRecorder()
	s.Router().ServeHTTP(rrReset, reqReset)
	if rrReset.Code != http.StatusOK {
		t.Fatalf("reset password should return 200: got=%d body=%s", rrReset.Code, rrReset.Body.String())
	}

	updatedUser, err := st.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("get updated user failed: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(updatedUser.PasswordHash), []byte("NewPass456!")) != nil {
		t.Fatalf("updated password hash does not match new password")
	}

	reqLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"reset@example.com","password":"NewPass456!"}`))
	reqLogin.Header.Set("Content-Type", "application/json")
	rrLogin := httptest.NewRecorder()
	s.Router().ServeHTTP(rrLogin, reqLogin)
	if rrLogin.Code != http.StatusOK {
		t.Fatalf("login with new password should succeed: got=%d body=%s", rrLogin.Code, rrLogin.Body.String())
	}

	found := false
	for _, evt := range st.ListAuditEvents() {
		if evt.Action == "AUTH_PASSWORD_RESET" && evt.ActorID == u.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected AUTH_PASSWORD_RESET audit event")
	}
}

func TestResetPasswordTokenIsOneTime(t *testing.T) {
	s, st, _ := newSecurityHTTPTestServer(t)
	pwd, _ := bcrypt.GenerateFromPassword([]byte("OldPass123!"), 12)
	u, err := st.CreateUser(model.Utente{Email: "one-time@example.com", PasswordHash: string(pwd), Ruolo: model.RoleRichiedente, Nome: "One", Cognome: "Time", Attivo: true, EmailVerificata: true})
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	_, err = st.CreatePasswordResetToken(model.PasswordResetToken{Token: "tok-one-time", Purpose: "password_reset", UserID: u.ID, Email: u.Email, ExpiresAt: time.Now().UTC().Add(30 * time.Minute)})
	if err != nil {
		t.Fatalf("create reset token failed: %v", err)
	}

	reqFirst := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", strings.NewReader(`{"token":"tok-one-time","new_password":"NewPass456!"}`))
	reqFirst.Header.Set("Content-Type", "application/json")
	rrFirst := httptest.NewRecorder()
	s.Router().ServeHTTP(rrFirst, reqFirst)
	if rrFirst.Code != http.StatusOK {
		t.Fatalf("first reset should succeed: got=%d body=%s", rrFirst.Code, rrFirst.Body.String())
	}

	reqSecond := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", strings.NewReader(`{"token":"tok-one-time","new_password":"Another789!"}`))
	reqSecond.Header.Set("Content-Type", "application/json")
	rrSecond := httptest.NewRecorder()
	s.Router().ServeHTTP(rrSecond, reqSecond)
	if rrSecond.Code != http.StatusUnauthorized {
		t.Fatalf("second reset with same token should fail: got=%d body=%s", rrSecond.Code, rrSecond.Body.String())
	}

	found := false
	for _, evt := range st.ListAuditEvents() {
		if evt.Action == "AUTH_PASSWORD_RESET_REJECTED" {
			if reason, ok := evt.Details["reason"].(string); ok && reason == "invalid_or_expired_token" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected AUTH_PASSWORD_RESET_REJECTED audit event")
	}
}
