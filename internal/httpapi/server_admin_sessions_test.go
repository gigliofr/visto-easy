package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
)

func TestAdminCanListAndRevokeUserSessions(t *testing.T) {
	s, st, adminToken := newSecurityHTTPTestServer(t)
	u, err := st.CreateUser(model.Utente{Email: "target-sessions@example.com", PasswordHash: "x", Ruolo: model.RoleRichiedente, Nome: "Target", Cognome: "User"})
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	_, err = st.CreateRefreshSession(model.RefreshSession{ID: "sess-a", UserID: u.ID, Role: string(model.RoleRichiedente), ExpiresAt: time.Now().UTC().Add(2 * time.Hour), Revoked: false})
	if err != nil {
		t.Fatalf("create refresh session a failed: %v", err)
	}
	_, err = st.CreateRefreshSession(model.RefreshSession{ID: "sess-b", UserID: u.ID, Role: string(model.RoleRichiedente), ExpiresAt: time.Now().UTC().Add(2 * time.Hour), Revoked: false})
	if err != nil {
		t.Fatalf("create refresh session b failed: %v", err)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/bo/utenti/"+u.ID+"/sessioni", nil)
	reqList.Header.Set("Authorization", "Bearer "+adminToken)
	rrList := httptest.NewRecorder()
	s.Router().ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("list sessions failed: got=%d body=%s", rrList.Code, rrList.Body.String())
	}

	reqRevoke := httptest.NewRequest(http.MethodPost, "/api/bo/utenti/"+u.ID+"/sessioni/revoca-all", strings.NewReader(`{}`))
	reqRevoke.Header.Set("Authorization", "Bearer "+adminToken)
	reqRevoke.Header.Set("Content-Type", "application/json")
	rrRevoke := httptest.NewRecorder()
	s.Router().ServeHTTP(rrRevoke, reqRevoke)
	if rrRevoke.Code != http.StatusOK {
		t.Fatalf("revoke all sessions failed: got=%d body=%s", rrRevoke.Code, rrRevoke.Body.String())
	}

	sessions := st.ListRefreshSessionsByUser(u.ID)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions after revoke, got=%d", len(sessions))
	}
	for _, sess := range sessions {
		if !sess.Revoked {
			t.Fatalf("expected session revoked, got=%+v", sess)
		}
	}
}

func TestNonAdminCannotManageSessions(t *testing.T) {
	s, st, _ := newSecurityHTTPTestServer(t)
	op, err := st.CreateUser(model.Utente{Email: "operatore-sessioni@example.com", PasswordHash: "x", Ruolo: model.RoleOperatore, Nome: "Operatore", Cognome: "NoAdmin"})
	if err != nil {
		t.Fatalf("create operator failed: %v", err)
	}
	tm, err := auth.NewTokenManager("this-is-a-long-test-secret-with-32-plus")
	if err != nil {
		t.Fatalf("token manager init failed: %v", err)
	}
	opToken, err := tm.SignAccess(op.ID, string(model.RoleOperatore))
	if err != nil {
		t.Fatalf("sign operator token failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/bo/utenti/any/sessioni", nil)
	req.Header.Set("Authorization", "Bearer "+opToken)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for non-admin, got=%d body=%s", rr.Code, rr.Body.String())
	}
}
