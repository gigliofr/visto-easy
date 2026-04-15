package httpapi

import (
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"
)

func TestRegisterRejectsWeakPassword(t *testing.T) {
  s, _, _ := newSecurityHTTPTestServer(t)

  req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"email":"weak@example.com","password":"weakpass","nome":"Weak","cognome":"User"}`))
  req.Header.Set("Content-Type", "application/json")
  rr := httptest.NewRecorder()

  s.Router().ServeHTTP(rr, req)
  if rr.Code != http.StatusBadRequest {
    t.Fatalf("register with weak password should fail: got=%d body=%s", rr.Code, rr.Body.String())
  }
  if !strings.Contains(strings.ToLower(rr.Body.String()), "password") {
    t.Fatalf("expected password validation message: body=%s", rr.Body.String())
  }
}

func TestRegisterCreatesPendingVerificationAccount(t *testing.T) {
  s, st, _ := newSecurityHTTPTestServer(t)

  req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"email":"new.user@example.com","password":"Strong#Pass123","nome":"New","cognome":"User"}`))
  req.Header.Set("Content-Type", "application/json")
  rr := httptest.NewRecorder()

  s.Router().ServeHTTP(rr, req)
  if rr.Code != http.StatusCreated {
    t.Fatalf("register should return 201: got=%d body=%s", rr.Code, rr.Body.String())
  }

  var payload map[string]any
  if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
    t.Fatalf("invalid register response json: %v", err)
  }
  if _, hasLink := payload["verification_link"]; hasLink {
    t.Fatalf("register response must not expose verification link token")
  }

  if len(st.passwordResetTokens) != 1 {
    t.Fatalf("expected one verification token, got=%d", len(st.passwordResetTokens))
  }

  var verificationToken string
  for token, rec := range st.passwordResetTokens {
    if rec.Purpose == "email_verification" {
      verificationToken = token
      break
    }
  }
  if verificationToken == "" {
    t.Fatalf("verification token not found")
  }

  user, err := st.GetUserByEmail("new.user@example.com")
  if err != nil {
    t.Fatalf("user should exist after register: %v", err)
  }
  if user.Attivo || user.EmailVerificata {
    t.Fatalf("newly registered user should be pending verification")
  }

  loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"new.user@example.com","password":"Strong#Pass123"}`))
  loginReq.Header.Set("Content-Type", "application/json")
  loginRR := httptest.NewRecorder()

  s.Router().ServeHTTP(loginRR, loginReq)
  if loginRR.Code != http.StatusForbidden {
    t.Fatalf("login before verification should be forbidden: got=%d body=%s", loginRR.Code, loginRR.Body.String())
  }
}

func TestRegisterRejectsInvalidEmail(t *testing.T) {
  s, _, _ := newSecurityHTTPTestServer(t)

  req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"email":"not-an-email","password":"Strong#Pass123","nome":"Bad","cognome":"Mail"}`))
  req.Header.Set("Content-Type", "application/json")
  rr := httptest.NewRecorder()

  s.Router().ServeHTTP(rr, req)
  if rr.Code != http.StatusBadRequest {
    t.Fatalf("register with invalid email should fail: got=%d body=%s", rr.Code, rr.Body.String())
  }
}
