package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"visto-easy/internal/model"
)

func TestRefreshTokenRotationInvalidatesPreviousToken(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	_ = token

	oldID := "refresh-old-1"
	oldRefresh, err := s.tokens.SignRefreshWithJTI("u-1", string(model.RoleAdmin), oldID)
	if err != nil {
		t.Fatalf("sign refresh failed: %v", err)
	}
	_, err = st.CreateRefreshSession(model.RefreshSession{
		ID:        oldID,
		UserID:    "u-1",
		Role:      string(model.RoleAdmin),
		Revoked:   false,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create refresh session failed: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", strings.NewReader(`{"refresh_token":"`+oldRefresh+`"}`))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	s.Router().ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first refresh should succeed: got=%d body=%s", rr1.Code, rr1.Body.String())
	}

	sessOld, err := st.GetRefreshSessionByID(oldID)
	if err != nil {
		t.Fatalf("old session should still be retrievable: %v", err)
	}
	if !sessOld.Revoked {
		t.Fatalf("expected old refresh session revoked after rotation")
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", strings.NewReader(`{"refresh_token":"`+oldRefresh+`"}`))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	s.Router().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("reusing rotated refresh should be unauthorized: got=%d body=%s", rr2.Code, rr2.Body.String())
	}
}

func TestLogoutRevokesRefreshSession(t *testing.T) {
	s, st, _ := newSecurityHTTPTestServer(t)

	refreshID := "refresh-logout-1"
	refreshToken, err := s.tokens.SignRefreshWithJTI("u-2", string(model.RoleAdmin), refreshID)
	if err != nil {
		t.Fatalf("sign refresh failed: %v", err)
	}
	_, err = st.CreateRefreshSession(model.RefreshSession{
		ID:        refreshID,
		UserID:    "u-2",
		Role:      string(model.RoleAdmin),
		Revoked:   false,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create refresh session failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", strings.NewReader(`{"refresh_token":"`+refreshToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("logout should succeed: got=%d body=%s", rr.Code, rr.Body.String())
	}

	sess, err := st.GetRefreshSessionByID(refreshID)
	if err != nil {
		t.Fatalf("refresh session should exist: %v", err)
	}
	if !sess.Revoked {
		t.Fatalf("expected refresh session revoked by logout")
	}
}
