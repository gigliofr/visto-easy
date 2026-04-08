package httpapi

import (
	"context"
	"encoding/csv"
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

func TestBOSecurityEventsStatsTopFailedIPsOrder(t *testing.T) {
	t.Setenv("SECURITY_ALERT_WINDOW_MINUTES", "15")
	t.Setenv("SECURITY_ALERT_FAILED_THRESHOLD", "3")

	s, st, token := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()
	st.securityEvents = []model.SecurityEvent{
		{ID: "e1", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.9", CreatoIl: now.Add(-2 * time.Minute)},
		{ID: "e2", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.8", CreatoIl: now.Add(-90 * time.Second)},
		{ID: "e3", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.9", CreatoIl: now.Add(-45 * time.Second)},
		{ID: "e4", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", IP: "203.0.113.7", CreatoIl: now.Add(-20 * time.Second)},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/bo/security-events/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}

	resp := decodeMap(t, rr)
	topRaw, ok := resp["top_failed_ips"].([]any)
	if !ok || len(topRaw) < 2 {
		t.Fatalf("unexpected top_failed_ips payload: %#v", resp)
	}
	first, ok := topRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("invalid first top entry: %#v", topRaw[0])
	}
	if first["key"] != "203.0.113.9" || first["count"] != float64(2) {
		t.Fatalf("unexpected first top entry: %#v", first)
	}
	if resp["high_risk_detected"] != true {
		t.Fatalf("expected high_risk_detected=true: %#v", resp)
	}
}

func TestBOSecurityEventsCSVFiltersAndSortsByRecent(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()
	st.securityEvents = []model.SecurityEvent{
		{ID: "e-old", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", Email: "a@example.com", IP: "203.0.113.10", CreatoIl: now.Add(-3 * time.Minute)},
		{ID: "e-mid", Type: "IP_BLOCKED", Outcome: "manual", UserID: "admin-1", IP: "203.0.113.11", CreatoIl: now.Add(-2 * time.Minute)},
		{ID: "e-new", Type: "LOGIN_FAILED", Outcome: "invalid_credentials", Email: "b@example.com", IP: "203.0.113.12", CreatoIl: now.Add(-1 * time.Minute)},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/bo/security-events/report.csv?type=login_failed", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(strings.ToLower(ct), "text/csv") {
		t.Fatalf("unexpected content-type: %s", ct)
	}

	rows, err := csv.NewReader(strings.NewReader(rr.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("invalid csv: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected header + 2 rows, got=%d body=%s", len(rows), rr.Body.String())
	}
	if rows[1][0] != "e-new" || rows[2][0] != "e-old" {
		t.Fatalf("expected rows sorted by recente desc, got row1=%v row2=%v", rows[1], rows[2])
	}
	if rows[1][1] != "LOGIN_FAILED" || rows[2][1] != "LOGIN_FAILED" {
		t.Fatalf("unexpected filtered types in csv: row1=%v row2=%v", rows[1], rows[2])
	}
}

func TestSecurityAlertsStreamIgnoresNonLoginAndOldLockedEvents(t *testing.T) {
	t.Setenv("SECURITY_ALERT_WINDOW_MINUTES", "1")
	t.Setenv("SECURITY_ALERT_FAILED_THRESHOLD", "2")

	s, st, token := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()
	st.securityEvents = []model.SecurityEvent{
		{ID: "e-old-locked", Type: "LOGIN_LOCKED", Outcome: "blocked", IP: "203.0.113.20", CreatoIl: now.Add(-3 * time.Minute)},
		{ID: "e-non-login", Type: "IP_BLOCKED", Outcome: "manual", IP: "203.0.113.21", CreatoIl: now.Add(-10 * time.Second)},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/bo/security-events/stream", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "event: security_alert") {
		t.Fatalf("missing security_alert SSE event: body=%s", body)
	}
	if !strings.Contains(body, `"severity":"ok"`) {
		t.Fatalf("expected severity ok snapshot: body=%s", body)
	}
	if !strings.Contains(body, `"recent_failed_logins":0`) || !strings.Contains(body, `"recent_locked_logins":0`) {
		t.Fatalf("expected zero recent login alerts in snapshot: body=%s", body)
	}
}
