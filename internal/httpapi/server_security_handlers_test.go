package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
)

func newSecurityHTTPTestServer(t *testing.T) (*Server, *fakePolicyStore, string) {
	t.Helper()
	st := &fakePolicyStore{}
	tm, err := auth.NewTokenManager("this-is-a-long-test-secret-with-32-plus")
	if err != nil {
		t.Fatalf("token manager init failed: %v", err)
	}
	tok, err := tm.SignAccess("bo-admin-1", string(model.RoleAdmin))
	if err != nil {
		t.Fatalf("access token sign failed: %v", err)
	}
	s := &Server{store: st, tokens: tm}
	return s, st, tok
}

func mustJSONBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	return bytes.NewReader(b)
}

func decodeMap(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("json decode failed: %v; body=%s", err, rr.Body.String())
	}
	return m
}

func TestBOEvaluateIPEndpoint(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()

	_, _ = st.UpsertBlockedIP(model.BlockedIP{IP: "10.10.0.0/16", Reason: "broad block", BlockedBy: "admin", BlockedAt: now})
	_, _ = st.UpsertAllowedIP(model.AllowedIP{IP: "10.10.1.55", Reason: "exception", AllowedBy: "admin", AllowedAt: now.Add(time.Second)})

	req := httptest.NewRequest(http.MethodGet, "/api/bo/security/evaluate-ip?ip=10.10.1.55", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}

	resp := decodeMap(t, rr)
	decision, ok := resp["decision"].(map[string]any)
	if !ok {
		t.Fatalf("missing decision object: %#v", resp)
	}
	if decision["action"] != "allow" {
		t.Fatalf("unexpected action: %#v", decision)
	}
	if decision["reason"] != "exact_allow_rule" {
		t.Fatalf("unexpected reason: %#v", decision)
	}
}

func TestBORevokeAllowedIPBulkEndpoint(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()

	_, _ = st.UpsertAllowedIP(model.AllowedIP{IP: "10.0.0.0/24", Reason: "office", AllowedBy: "admin", AllowedAt: now})

	body := mustJSONBody(t, map[string]any{
		"targets": []string{"10.0.0.0/24", "10.0.0.5", "bad-target"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/bo/security/allowed-ips/revoke-bulk", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}

	resp := decodeMap(t, rr)
	count, ok := resp["count"].(map[string]any)
	if !ok {
		t.Fatalf("missing count object: %#v", resp)
	}
	if count["removed"] != float64(1) || count["not_found"] != float64(1) || count["invalid"] != float64(1) {
		t.Fatalf("unexpected counters: %#v", count)
	}
}

func TestBOUnblockIPBulkAllEndpoint(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()

	_, _ = st.UpsertBlockedIP(model.BlockedIP{IP: "203.0.113.0/24", Reason: "abuse", BlockedBy: "admin", BlockedAt: now})
	_, _ = st.UpsertBlockedIP(model.BlockedIP{IP: "198.51.100.1", Reason: "incident", BlockedBy: "admin", BlockedAt: now.Add(time.Second)})

	body := mustJSONBody(t, map[string]any{"unblock_all": true})
	req := httptest.NewRequest(http.MethodPost, "/api/bo/security/blocked-ips/unblock-bulk", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}

	resp := decodeMap(t, rr)
	count, ok := resp["count"].(map[string]any)
	if !ok {
		t.Fatalf("missing count object: %#v", resp)
	}
	if count["removed"] != float64(2) {
		t.Fatalf("unexpected removed count: %#v", count)
	}
	if got := len(st.ListBlockedIPs()); got != 0 {
		t.Fatalf("expected blocked list empty after unblock_all, got=%d", got)
	}
}
