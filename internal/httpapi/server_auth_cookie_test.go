package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"visto-easy/internal/model"
)

func refreshCookieOf(rr *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}
	return nil
}

func postAuth(s *Server, path, body string, cookie *http.Cookie, https bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if https {
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	if cookie != nil {
		req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	return rr
}

func TestLoginRefreshLogoutWithHttpOnlyCookie(t *testing.T) {
	s, st, _ := newSecurityHTTPTestServer(t)
	pwd, _ := bcrypt.GenerateFromPassword([]byte("Cookie#Pass123"), bcrypt.MinCost)
	if _, err := st.CreateUser(model.Utente{Email: "cookie@example.com", PasswordHash: string(pwd), Ruolo: model.RoleRichiedente, Nome: "C", Cognome: "K", Attivo: true, EmailVerificata: true}); err != nil {
		t.Fatal(err)
	}

	// Login: refresh token solo nel cookie HttpOnly.
	rr := postAuth(s, "/api/auth/login", `{"email":"cookie@example.com","password":"Cookie#Pass123"}`, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "refresh_token") || !strings.Contains(rr.Body.String(), "access_token") {
		t.Fatalf("il JSON deve avere solo l'access token: %s", rr.Body.String())
	}
	c1 := refreshCookieOf(rr)
	if c1 == nil || !c1.HttpOnly || !c1.Secure || c1.SameSite != http.SameSiteStrictMode || c1.Path != "/api/auth" || c1.MaxAge != 7*24*3600 {
		t.Fatalf("cookie del refresh errato: %+v", c1)
	}

	// Refresh senza corpo, solo con il cookie: ruota il cookie.
	rr = postAuth(s, "/api/auth/refresh", "", c1, true)
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "refresh_token") {
		t.Fatalf("refresh con cookie: %d %s", rr.Code, rr.Body.String())
	}
	c2 := refreshCookieOf(rr)
	if c2 == nil || c2.Value == "" || c2.Value == c1.Value {
		t.Fatalf("il cookie deve essere ruotato: %+v", c2)
	}
	// Il vecchio cookie non vale più (rotazione con sessione revocata).
	if rr := postAuth(s, "/api/auth/refresh", "", c1, true); rr.Code != http.StatusUnauthorized {
		t.Fatalf("riuso del vecchio refresh: atteso 401, ottenuto %d", rr.Code)
	}

	// Logout con il cookie: revoca la sessione e cancella il cookie.
	rr = postAuth(s, "/api/auth/logout", "", c2, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("logout: %d %s", rr.Code, rr.Body.String())
	}
	if c := refreshCookieOf(rr); c == nil || c.MaxAge != -1 {
		t.Fatalf("il logout deve cancellare il cookie: %+v", c)
	}
	if rr := postAuth(s, "/api/auth/refresh", "", c2, true); rr.Code != http.StatusUnauthorized {
		t.Fatalf("dopo il logout il refresh deve fallire, ottenuto %d", rr.Code)
	}
}

func TestRefreshWithoutCookieOrBody(t *testing.T) {
	s, _, _ := newSecurityHTTPTestServer(t)
	if rr := postAuth(s, "/api/auth/refresh", "", nil, false); rr.Code != http.StatusUnauthorized {
		t.Fatalf("senza cookie né corpo atteso 401, ottenuto %d", rr.Code)
	}
	if rr := postAuth(s, "/api/auth/logout", "", nil, false); rr.Code != http.StatusOK {
		t.Fatalf("logout senza sessione deve rispondere 200, ottenuto %d", rr.Code)
	}
}
