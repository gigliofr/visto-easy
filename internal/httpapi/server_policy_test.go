package httpapi

import (
	"testing"
	"time"

	"visto-easy/internal/model"
	"visto-easy/internal/store"
)

type fakePolicyStore struct {
	allowed []model.AllowedIP
	blocked []model.BlockedIP
	securityEvents []model.SecurityEvent
}

func newPolicyTestServer() *Server {
	return &Server{store: &fakePolicyStore{}}
}

func (f *fakePolicyStore) CreateUser(u model.Utente) (model.Utente, error) { return model.Utente{}, store.ErrNotFound }
func (f *fakePolicyStore) ListUsers() []model.Utente { return nil }
func (f *fakePolicyStore) GetUserByEmail(email string) (model.Utente, error) {
	return model.Utente{}, store.ErrNotFound
}
func (f *fakePolicyStore) GetUserByID(id string) (model.Utente, error) { return model.Utente{}, store.ErrNotFound }
func (f *fakePolicyStore) CreatePratica(p model.Pratica, actorID string) (model.Pratica, error) {
	return model.Pratica{}, store.ErrNotFound
}
func (f *fakePolicyStore) ListPraticheByUser(userID string) []model.Pratica { return nil }
func (f *fakePolicyStore) ListAllPratiche() []model.Pratica { return nil }
func (f *fakePolicyStore) GetPratica(id string) (model.Pratica, error) { return model.Pratica{}, store.ErrNotFound }
func (f *fakePolicyStore) UpdatePraticaAsDraft(id, userID string, data map[string]any) (model.Pratica, error) {
	return model.Pratica{}, store.ErrNotFound
}
func (f *fakePolicyStore) DeletePraticaAsDraft(id, userID string) error { return store.ErrNotFound }
func (f *fakePolicyStore) SubmitPratica(id, userID string) (model.Pratica, error) {
	return model.Pratica{}, store.ErrNotFound
}
func (f *fakePolicyStore) ChangePraticaState(id string, fromActor string, next model.StatoPratica, note string) (model.Pratica, error) {
	return model.Pratica{}, store.ErrNotFound
}
func (f *fakePolicyStore) AssignOperatore(praticaID, operatoreID, actorID string) (model.Pratica, error) {
	return model.Pratica{}, store.ErrNotFound
}
func (f *fakePolicyStore) AddNota(praticaID, actorID, message string, internal bool) (model.Pratica, error) {
	return model.Pratica{}, store.ErrNotFound
}
func (f *fakePolicyStore) RequestDocumento(praticaID, actorID, documento, note string) (model.Pratica, error) {
	return model.Pratica{}, store.ErrNotFound
}
func (f *fakePolicyStore) AddDocumento(praticaID string, d model.Documento) (model.Documento, error) {
	return model.Documento{}, store.ErrNotFound
}
func (f *fakePolicyStore) ListDocumenti(praticaID string) ([]model.Documento, error) {
	return nil, store.ErrNotFound
}
func (f *fakePolicyStore) CreatePayment(praticaID, provider string, amount float64) (model.Pagamento, error) {
	return model.Pagamento{}, store.ErrNotFound
}
func (f *fakePolicyStore) GetPaymentByToken(token string) (model.Pagamento, error) {
	return model.Pagamento{}, store.ErrNotFound
}
func (f *fakePolicyStore) ConfirmPaymentByToken(token string) (model.Pagamento, error) {
	return model.Pagamento{}, store.ErrNotFound
}
func (f *fakePolicyStore) MarkWebhookEventProcessed(provider, eventID, paymentID string) (bool, error) {
	return false, nil
}
func (f *fakePolicyStore) AddSecurityEvent(evt model.SecurityEvent) (model.SecurityEvent, error) {
	f.securityEvents = append(f.securityEvents, evt)
	return evt, nil
}
func (f *fakePolicyStore) ListSecurityEvents() []model.SecurityEvent {
	out := make([]model.SecurityEvent, len(f.securityEvents))
	copy(out, f.securityEvents)
	return out
}
func (f *fakePolicyStore) GetSecurityEventByID(id string) (model.SecurityEvent, error) {
	return model.SecurityEvent{}, store.ErrNotFound
}
func (f *fakePolicyStore) UpsertBlockedIP(entry model.BlockedIP) (model.BlockedIP, error) {
	for i := range f.blocked {
		if f.blocked[i].IP == entry.IP {
			f.blocked[i] = entry
			return entry, nil
		}
	}
	f.blocked = append(f.blocked, entry)
	return entry, nil
}
func (f *fakePolicyStore) RemoveBlockedIP(ip string) (bool, error) {
	for i := range f.blocked {
		if f.blocked[i].IP == ip {
			f.blocked = append(f.blocked[:i], f.blocked[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (f *fakePolicyStore) ListBlockedIPs() []model.BlockedIP {
	now := time.Now().UTC()
	filtered := make([]model.BlockedIP, 0, len(f.blocked))
	for _, item := range f.blocked {
		if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			continue
		}
		filtered = append(filtered, item)
	}
	f.blocked = filtered
	out := make([]model.BlockedIP, len(filtered))
	copy(out, filtered)
	return out
}
func (f *fakePolicyStore) GetBlockedIP(ip string) (model.BlockedIP, error) {
	now := time.Now().UTC()
	for _, item := range f.blocked {
		if item.IP == ip {
			if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
				return model.BlockedIP{}, store.ErrNotFound
			}
			return item, nil
		}
	}
	return model.BlockedIP{}, store.ErrNotFound
}
func (f *fakePolicyStore) UpsertAllowedIP(entry model.AllowedIP) (model.AllowedIP, error) {
	for i := range f.allowed {
		if f.allowed[i].IP == entry.IP {
			f.allowed[i] = entry
			return entry, nil
		}
	}
	f.allowed = append(f.allowed, entry)
	return entry, nil
}
func (f *fakePolicyStore) RemoveAllowedIP(ip string) (bool, error) {
	for i := range f.allowed {
		if f.allowed[i].IP == ip {
			f.allowed = append(f.allowed[:i], f.allowed[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (f *fakePolicyStore) ListAllowedIPs() []model.AllowedIP {
	now := time.Now().UTC()
	filtered := make([]model.AllowedIP, 0, len(f.allowed))
	for _, item := range f.allowed {
		if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			continue
		}
		filtered = append(filtered, item)
	}
	f.allowed = filtered
	out := make([]model.AllowedIP, len(filtered))
	copy(out, filtered)
	return out
}
func (f *fakePolicyStore) GetAllowedIP(ip string) (model.AllowedIP, error) {
	now := time.Now().UTC()
	for _, item := range f.allowed {
		if item.IP == ip {
			if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
				return model.AllowedIP{}, store.ErrNotFound
			}
			return item, nil
		}
	}
	return model.AllowedIP{}, store.ErrNotFound
}

func TestNormalizeBlockTarget(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "ipv4", input: "203.0.113.7", want: "203.0.113.7"},
		{name: "ipv4 with port", input: "203.0.113.7:443", want: "203.0.113.7"},
		{name: "cidr", input: "203.0.113.0/24", want: "203.0.113.0/24"},
		{name: "invalid", input: "not-an-ip", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeBlockTarget(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("normalizeBlockTarget(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestEvaluatePolicyExactBlockWinsOverExactAllow(t *testing.T) {
	s := newPolicyTestServer()
	now := time.Now().UTC()

	_, _ = s.store.UpsertAllowedIP(model.AllowedIP{IP: "203.0.113.7", Reason: "trusted", AllowedBy: "admin", AllowedAt: now})
	_, _ = s.store.UpsertBlockedIP(model.BlockedIP{IP: "203.0.113.7", Reason: "incident", BlockedBy: "admin", BlockedAt: now.Add(time.Second)})

	decision := s.evaluateClientIPPolicy("203.0.113.7")
	if decision.Action != "block" || decision.Reason != "exact_block_rule" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestEvaluatePolicyExactAllowWinsOverCIDRBlock(t *testing.T) {
	s := newPolicyTestServer()
	now := time.Now().UTC()

	_, _ = s.store.UpsertBlockedIP(model.BlockedIP{IP: "10.10.0.0/16", Reason: "broad block", BlockedBy: "admin", BlockedAt: now})
	_, _ = s.store.UpsertAllowedIP(model.AllowedIP{IP: "10.10.1.55", Reason: "exception", AllowedBy: "admin", AllowedAt: now.Add(time.Second)})

	decision := s.evaluateClientIPPolicy("10.10.1.55")
	if decision.Action != "allow" || decision.Reason != "exact_allow_rule" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestEvaluatePolicyCIDRSpecificity(t *testing.T) {
	s := newPolicyTestServer()
	now := time.Now().UTC()

	_, _ = s.store.UpsertBlockedIP(model.BlockedIP{IP: "10.0.0.0/16", Reason: "network risk", BlockedBy: "admin", BlockedAt: now})
	_, _ = s.store.UpsertAllowedIP(model.AllowedIP{IP: "10.0.1.0/24", Reason: "office subnet", AllowedBy: "admin", AllowedAt: now})

	decision := s.evaluateClientIPPolicy("10.0.1.50")
	if decision.Action != "allow" || decision.Reason != "cidr_allow_precedence" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestEvaluatePolicyCIDRTieBreakBlock(t *testing.T) {
	s := newPolicyTestServer()
	now := time.Now().UTC()

	_, _ = s.store.UpsertAllowedIP(model.AllowedIP{IP: "192.168.10.0/24", Reason: "allow temp", AllowedBy: "admin", AllowedAt: now})
	_, _ = s.store.UpsertBlockedIP(model.BlockedIP{IP: "192.168.10.0/24", Reason: "abuse", BlockedBy: "admin", BlockedAt: now})

	decision := s.evaluateClientIPPolicy("192.168.10.20")
	if decision.Action != "block" || decision.Reason != "cidr_block_precedence" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestEvaluatePolicyNoRule(t *testing.T) {
	s := newPolicyTestServer()

	decision := s.evaluateClientIPPolicy("198.51.100.42")
	if decision.Action != "allow" || decision.Reason != "no_matching_rule" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}
