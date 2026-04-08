package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"visto-easy/internal/model"
)

func TestBOAuditEventsListWithFiltersAndPagination(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	now := time.Now().UTC()

	_, _ = st.AddAuditEvent(model.AuditEvent{
		ActorID:   "bo-admin-1",
		ActorRole: string(model.RoleAdmin),
		Action:    "PRATICA_CHANGE_STATE",
		Resource:  "pratica",
		ResourceID: "pra-1",
		IP:        "203.0.113.10",
		CreatoIl:  now,
	})
	_, _ = st.AddAuditEvent(model.AuditEvent{
		ActorID:   "bo-admin-1",
		ActorRole: string(model.RoleAdmin),
		Action:    "PAYMENT_REFUND",
		Resource:  "payment",
		ResourceID: "pay-1",
		IP:        "203.0.113.11",
		CreatoIl:  now.Add(1 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/bo/audit-events?action=payment_refund&page=1&page_size=1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Items []model.AuditEvent `json:"items"`
		Count int                `json:"count"`
		Total int                `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json decode failed: %v; body=%s", err, rr.Body.String())
	}
	if resp.Total != 1 || resp.Count != 1 || len(resp.Items) != 1 {
		t.Fatalf("unexpected counts: %+v", resp)
	}
	if resp.Items[0].Action != "PAYMENT_REFUND" {
		t.Fatalf("unexpected action: %+v", resp.Items[0])
	}
}

func TestBOGetAuditEventByID(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)

	evt, err := st.AddAuditEvent(model.AuditEvent{
		ActorID:   "bo-admin-1",
		ActorRole: string(model.RoleAdmin),
		Action:    "PRATICA_ASSIGN_OPERATOR",
		Resource:  "pratica",
		ResourceID: "pra-33",
	})
	if err != nil {
		t.Fatalf("seed audit event failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/bo/audit-events/"+evt.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}

	var got model.AuditEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if got.ID != evt.ID || got.Action != "PRATICA_ASSIGN_OPERATOR" {
		t.Fatalf("unexpected event payload: %+v", got)
	}
}

func TestBOAuditEventsCSV(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)

	_, _ = st.AddAuditEvent(model.AuditEvent{
		ActorID:   "bo-admin-1",
		ActorRole: string(model.RoleAdmin),
		Action:    "PRATICA_ADD_NOTE",
		Resource:  "pratica",
		ResourceID: "pra-77",
		IP:        "198.51.100.23",
		CreatoIl:  time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/bo/audit-events/report.csv?resource=pratica", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("expected csv content type, got=%q", ct)
	}
	if !strings.Contains(rr.Body.String(), "PRATICA_ADD_NOTE") {
		t.Fatalf("expected action in csv body, got=%s", rr.Body.String())
	}
}
