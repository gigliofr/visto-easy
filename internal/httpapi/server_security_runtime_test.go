package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"visto-easy/internal/model"
)

func TestBOSecurityEventsStatsEndpoint(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()

	st.securityEvents = []model.SecurityEvent{
		{ID: "e1", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.1", CreatoIl: now.Add(-2 * time.Minute)},
		{ID: "e2", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.1", CreatoIl: now.Add(-1 * time.Minute)},
		{ID: "e3", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.2", CreatoIl: now.Add(-30 * time.Second)},
		{ID: "e4", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.3", CreatoIl: now.Add(-20 * time.Second)},
		{ID: "e5", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.4", CreatoIl: now.Add(-10 * time.Second)},
		{ID: "e6", Type: "LOGIN_LOCKED", Outcome: "blocked", IP: "203.0.113.1", CreatoIl: now.Add(-5 * time.Second)},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/bo/security-events/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["total"] != float64(6) {
		t.Fatalf("unexpected total: %#v", resp)
	}
	if resp["recent_failed_logins"] != float64(5) {
		t.Fatalf("unexpected recent_failed_logins: %#v", resp)
	}
	if resp["recent_locked_logins"] != float64(1) {
		t.Fatalf("unexpected recent_locked_logins: %#v", resp)
	}
	if resp["high_risk_detected"] != true {
		t.Fatalf("expected high_risk_detected=true: %#v", resp)
	}
}

func TestAuthMiddlewareBlocksCIDRRule(t *testing.T) {
	s, st, _ := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()
	_, _ = st.UpsertBlockedIP(model.BlockedIP{IP: "203.0.113.0/24", Reason: "abuse", BlockedBy: "admin", BlockedAt: now})

	body := strings.NewReader(`{"email":"u@example.com","password":"wrongpass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("X-Forwarded-For", "203.0.113.55")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 due blocklist, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddlewareAllowlistBeatsCIDRBlock(t *testing.T) {
	s, st, _ := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()
	_, _ = st.UpsertBlockedIP(model.BlockedIP{IP: "203.0.113.0/24", Reason: "abuse", BlockedBy: "admin", BlockedAt: now})
	_, _ = st.UpsertAllowedIP(model.AllowedIP{IP: "203.0.113.55", Reason: "trusted host", AllowedBy: "admin", AllowedAt: now.Add(time.Second)})

	body := strings.NewReader(`{"email":"u@example.com","password":"wrongpass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("X-Forwarded-For", "203.0.113.55")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code == http.StatusForbidden {
		t.Fatalf("allowlist should bypass blocklist, got 403 body=%s", rr.Body.String())
	}
	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status after allowlist pass-through: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddlewareIgnoresExpiredBlockRule(t *testing.T) {
	s, st, _ := newSecurityHTTPTestServer(t)
	past := time.Now().UTC().Add(-1 * time.Minute)
	_, _ = st.UpsertBlockedIP(model.BlockedIP{IP: "198.51.100.0/24", Reason: "old block", BlockedBy: "admin", BlockedAt: past, ExpiresAt: &past})

	body := strings.NewReader(`{"email":"u@example.com","password":"wrongpass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("X-Forwarded-For", "198.51.100.42")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code == http.StatusForbidden {
		t.Fatalf("expired block rule should not return 403: body=%s", rr.Body.String())
	}
	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status after expired block rule: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSecurityAlertsStreamSendsReadyAndAlert(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()
	st.securityEvents = []model.SecurityEvent{
		{ID: "e1", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.2", CreatoIl: now.Add(-10 * time.Second)},
		{ID: "e2", Type: "LOGIN_LOCKED", Outcome: "blocked", IP: "203.0.113.2", CreatoIl: now.Add(-5 * time.Second)},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/bo/security-events/stream", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "event: ready") {
		t.Fatalf("missing ready SSE event: body=%s", body)
	}
	if !strings.Contains(body, "event: security_alert") {
		t.Fatalf("missing security_alert SSE event: body=%s", body)
	}
}
