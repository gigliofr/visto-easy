package httpapi

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"visto-easy/internal/model"
	"visto-easy/internal/store"
)

type fakePolicyStore struct {
	allowed        []model.AllowedIP
	blocked        []model.BlockedIP
	securityEvents []model.SecurityEvent
	auditEvents    []model.AuditEvent
	users          map[string]model.Utente
	usersByEmail   map[string]string
	pratiche       map[string]model.Pratica
	refreshSessions map[string]model.RefreshSession
	passwordResetTokens map[string]model.PasswordResetToken
	payments       map[string]model.Pagamento
	seq            int
}

func newPolicyTestServer() *Server {
	return &Server{store: &fakePolicyStore{users: map[string]model.Utente{}, usersByEmail: map[string]string{}, pratiche: map[string]model.Pratica{}, refreshSessions: map[string]model.RefreshSession{}, passwordResetTokens: map[string]model.PasswordResetToken{}, payments: map[string]model.Pagamento{}}}
}

func (f *fakePolicyStore) nextID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%d", prefix, f.seq)
}

func (f *fakePolicyStore) CreateUser(u model.Utente) (model.Utente, error) {
	if f.users == nil {
		f.users = map[string]model.Utente{}
	}
	if f.usersByEmail == nil {
		f.usersByEmail = map[string]string{}
	}
	email := strings.ToLower(strings.TrimSpace(u.Email))
	if email == "" {
		return model.Utente{}, store.ErrForbiddenState
	}
	if _, ok := f.usersByEmail[email]; ok {
		return model.Utente{}, store.ErrAlreadyExists
	}
	now := time.Now().UTC()
	u.ID = f.nextID("usr")
	u.Email = email
	u.CreatoIl = now
	u.AggiornatoIl = now
	f.users[u.ID] = u
	f.usersByEmail[email] = u.ID
	return u, nil
}
func (f *fakePolicyStore) SetUserVerificationState(userID string, attivo, emailVerificata bool) (bool, error) {
	u, ok := f.users[strings.TrimSpace(userID)]
	if !ok {
		return false, nil
	}
	u.Attivo = attivo
	u.EmailVerificata = emailVerificata
	u.AggiornatoIl = time.Now().UTC()
	f.users[u.ID] = u
	return true, nil
}
func (f *fakePolicyStore) ListUsers() []model.Utente {
	out := make([]model.Utente, 0, len(f.users))
	for _, u := range f.users {
		out = append(out, u)
	}
	return out
}
func (f *fakePolicyStore) GetUserByEmail(email string) (model.Utente, error) {
	id, ok := f.usersByEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return model.Utente{}, store.ErrNotFound
	}
	u, ok := f.users[id]
	if !ok {
		return model.Utente{}, store.ErrNotFound
	}
	return u, nil
}
func (f *fakePolicyStore) GetUserByID(id string) (model.Utente, error) {
	u, ok := f.users[id]
	if !ok {
		return model.Utente{}, store.ErrNotFound
	}
	return u, nil
}
func (f *fakePolicyStore) SetUserTOTPSecret(userID, secret string) (bool, error) {
	u, ok := f.users[strings.TrimSpace(userID)]
	if !ok {
		return false, nil
	}
	u.TOTPSecret = strings.TrimSpace(secret)
	u.AggiornatoIl = time.Now().UTC()
	f.users[u.ID] = u
	return true, nil
}
func (f *fakePolicyStore) SetUserTOTPEnabled(userID string, enabled bool) (bool, error) {
	u, ok := f.users[strings.TrimSpace(userID)]
	if !ok {
		return false, nil
	}
	u.TOTPEnabled = enabled
	u.AggiornatoIl = time.Now().UTC()
	f.users[u.ID] = u
	return true, nil
}
func (f *fakePolicyStore) CreatePratica(p model.Pratica, actorID string) (model.Pratica, error) {
	if f.pratiche == nil {
		f.pratiche = map[string]model.Pratica{}
	}
	now := time.Now().UTC()
	p.ID = f.nextID("pra")
	p.Codice = f.nextID("VST")
	p.Stato = model.StatoBozza
	p.Priorita = model.PrioritaNormale
	p.Valuta = "EUR"
	p.CreatoIl = now
	p.AggiornatoIl = now
	f.pratiche[p.ID] = p
	return p, nil
}
func (f *fakePolicyStore) ListPraticheByUser(userID string) []model.Pratica { return nil }
func (f *fakePolicyStore) ListAllPratiche() []model.Pratica {
	out := make([]model.Pratica, 0, len(f.pratiche))
	for _, p := range f.pratiche {
		out = append(out, p)
	}
	return out
}
func (f *fakePolicyStore) GetPratica(id string) (model.Pratica, error) {
	p, ok := f.pratiche[id]
	if !ok {
		return model.Pratica{}, store.ErrNotFound
	}
	return p, nil
}
func (f *fakePolicyStore) UpdatePraticaAsDraft(id, userID string, data map[string]any) (model.Pratica, error) {
	return model.Pratica{}, store.ErrNotFound
}
func (f *fakePolicyStore) DeletePraticaAsDraft(id, userID string) error { return store.ErrNotFound }
func (f *fakePolicyStore) SubmitPratica(id, userID string) (model.Pratica, error) {
	p, ok := f.pratiche[id]
	if !ok {
		return model.Pratica{}, store.ErrNotFound
	}
	if p.UtenteID != userID || p.Stato != model.StatoBozza {
		return model.Pratica{}, store.ErrForbiddenState
	}
	now := time.Now().UTC()
	p.Stato = model.StatoInviata
	p.InviatoIl = &now
	p.AggiornatoIl = now
	f.pratiche[id] = p
	return p, nil
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
	p, ok := f.pratiche[praticaID]
	if !ok {
		return model.Documento{}, store.ErrNotFound
	}
	d.ID = f.nextID("doc")
	d.PraticaID = praticaID
	d.CaricatoIl = time.Now().UTC()
	d.StatoValidazione = "PENDING"
	d.S3Key = fmt.Sprintf("pratiche/%s/documenti/%s_%s", praticaID, d.ID, d.NomeFile)
	p.Documenti = append(p.Documenti, d)
	f.pratiche[praticaID] = p
	return d, nil
}
func (f *fakePolicyStore) ListDocumenti(praticaID string) ([]model.Documento, error) {
	p, ok := f.pratiche[praticaID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return p.Documenti, nil
}
func (f *fakePolicyStore) GetDocumento(praticaID, documentoID string) (model.Documento, error) {
	p, ok := f.pratiche[praticaID]
	if !ok {
		return model.Documento{}, store.ErrNotFound
	}
	for _, d := range p.Documenti {
		if d.ID == documentoID {
			return d, nil
		}
	}
	return model.Documento{}, store.ErrNotFound
}
func (f *fakePolicyStore) DeleteDocumento(praticaID, documentoID string) (bool, error) {
	p, ok := f.pratiche[praticaID]
	if !ok {
		return false, store.ErrNotFound
	}
	for i := range p.Documenti {
		if p.Documenti[i].ID == documentoID {
			p.Documenti = append(p.Documenti[:i], p.Documenti[i+1:]...)
			f.pratiche[praticaID] = p
			return true, nil
		}
	}
	return false, nil
}
func (f *fakePolicyStore) CreatePayment(praticaID, provider string, amount float64) (model.Pagamento, error) {
	if f.payments == nil {
		f.payments = map[string]model.Pagamento{}
	}
	if _, ok := f.pratiche[praticaID]; !ok {
		return model.Pagamento{}, store.ErrNotFound
	}
	now := time.Now().UTC()
	id := f.nextID("pay")
	token := strings.ReplaceAll(f.nextID("tok"), "-", "")
	p := model.Pagamento{
		ID: id, PraticaID: praticaID, Provider: provider,
		ProviderSessionID: "sess_" + id, Token: token,
		Importo: amount, Valuta: "EUR", Stato: model.PagamentoPendente,
		LinkPagamento: "/pagamento/" + token, Scadenza: now.Add(48 * time.Hour), CreatoIl: now,
	}
	f.payments[p.ID] = p
	return p, nil
}
func (f *fakePolicyStore) UpdatePaymentCheckout(paymentID, providerSessionID, linkPagamento string) (model.Pagamento, error) {
	p, ok := f.payments[strings.TrimSpace(paymentID)]
	if !ok {
		return model.Pagamento{}, store.ErrNotFound
	}
	if strings.TrimSpace(providerSessionID) != "" {
		p.ProviderSessionID = strings.TrimSpace(providerSessionID)
	}
	if strings.TrimSpace(linkPagamento) != "" {
		p.LinkPagamento = strings.TrimSpace(linkPagamento)
	}
	f.payments[p.ID] = p
	return p, nil
}
func (f *fakePolicyStore) GetPaymentByToken(token string) (model.Pagamento, error) {
	for _, p := range f.payments {
		if p.Token == token {
			return p, nil
		}
	}
	return model.Pagamento{}, store.ErrNotFound
}
func (f *fakePolicyStore) ConfirmPaymentByToken(token string) (model.Pagamento, error) {
	for id, p := range f.payments {
		if p.Token == token {
			now := time.Now().UTC()
			p.Stato = model.PagamentoCompletato
			p.PagatoIl = &now
			f.payments[id] = p
			return p, nil
		}
	}
	return model.Pagamento{}, store.ErrNotFound
}
func (f *fakePolicyStore) RefundPaymentByToken(token string) (model.Pagamento, error) {
	for id, p := range f.payments {
		if p.Token != token {
			continue
		}
		if p.Stato != model.PagamentoCompletato {
			return model.Pagamento{}, store.ErrInvalidState
		}
		p.Stato = model.PagamentoRimborsato
		f.payments[id] = p
		return p, nil
	}
	return model.Pagamento{}, store.ErrNotFound
}
func (f *fakePolicyStore) CreateRefreshSession(session model.RefreshSession) (model.RefreshSession, error) {
	if f.refreshSessions == nil {
		f.refreshSessions = map[string]model.RefreshSession{}
	}
	if strings.TrimSpace(session.ID) == "" {
		return model.RefreshSession{}, store.ErrForbiddenState
	}
	now := time.Now().UTC()
	if session.CreatoIl.IsZero() {
		session.CreatoIl = now
	}
	session.AggiornatoIl = now
	f.refreshSessions[session.ID] = session
	return session, nil
}
func (f *fakePolicyStore) GetRefreshSessionByID(id string) (model.RefreshSession, error) {
	session, ok := f.refreshSessions[strings.TrimSpace(id)]
	if !ok {
		return model.RefreshSession{}, store.ErrNotFound
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		delete(f.refreshSessions, strings.TrimSpace(id))
		return model.RefreshSession{}, store.ErrNotFound
	}
	return session, nil
}
func (f *fakePolicyStore) ListRefreshSessionsByUser(userID string) []model.RefreshSession {
	now := time.Now().UTC()
	items := make([]model.RefreshSession, 0)
	for id, session := range f.refreshSessions {
		if now.After(session.ExpiresAt) {
			delete(f.refreshSessions, id)
			continue
		}
		if session.UserID == strings.TrimSpace(userID) {
			items = append(items, session)
		}
	}
	return items
}
func (f *fakePolicyStore) RevokeRefreshSession(id, replacedBy string) (bool, error) {
	session, ok := f.refreshSessions[strings.TrimSpace(id)]
	if !ok || session.Revoked {
		return false, nil
	}
	now := time.Now().UTC()
	session.Revoked = true
	session.RevokedAt = &now
	session.ReplacedBy = strings.TrimSpace(replacedBy)
	session.AggiornatoIl = now
	f.refreshSessions[strings.TrimSpace(id)] = session
	return true, nil
}
func (f *fakePolicyStore) RevokeAllRefreshSessionsByUser(userID string) (int, error) {
	now := time.Now().UTC()
	revoked := 0
	for id, session := range f.refreshSessions {
		if session.UserID != strings.TrimSpace(userID) {
			continue
		}
		if now.After(session.ExpiresAt) {
			delete(f.refreshSessions, id)
			continue
		}
		if session.Revoked {
			continue
		}
		session.Revoked = true
		session.RevokedAt = &now
		session.ReplacedBy = "admin_bulk_revoke"
		session.AggiornatoIl = now
		f.refreshSessions[id] = session
		revoked++
	}
	return revoked, nil
}
func (f *fakePolicyStore) CreatePasswordResetToken(token model.PasswordResetToken) (model.PasswordResetToken, error) {
	if f.passwordResetTokens == nil {
		f.passwordResetTokens = map[string]model.PasswordResetToken{}
	}
	if strings.TrimSpace(token.Token) == "" || strings.TrimSpace(token.UserID) == "" {
		return model.PasswordResetToken{}, store.ErrForbiddenState
	}
	now := time.Now().UTC()
	if token.CreatoIl.IsZero() {
		token.CreatoIl = now
	}
	token.AggiornatoIl = now
	f.passwordResetTokens[token.Token] = token
	return token, nil
}
func (f *fakePolicyStore) ConsumePasswordResetToken(token string) (model.PasswordResetToken, error) {
	rec, ok := f.passwordResetTokens[strings.TrimSpace(token)]
	if !ok {
		return model.PasswordResetToken{}, store.ErrNotFound
	}
	now := time.Now().UTC()
	if rec.UsedAt != nil || now.After(rec.ExpiresAt) {
		delete(f.passwordResetTokens, strings.TrimSpace(token))
		return model.PasswordResetToken{}, store.ErrNotFound
	}
	rec.UsedAt = &now
	rec.AggiornatoIl = now
	f.passwordResetTokens[strings.TrimSpace(token)] = rec
	return rec, nil
}
func (f *fakePolicyStore) UpdateUserPassword(userID, passwordHash string) (bool, error) {
	u, ok := f.users[strings.TrimSpace(userID)]
	if !ok {
		return false, nil
	}
	if strings.TrimSpace(passwordHash) == "" {
		return false, store.ErrForbiddenState
	}
	u.PasswordHash = strings.TrimSpace(passwordHash)
	u.AggiornatoIl = time.Now().UTC()
	f.users[u.ID] = u
	return true, nil
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
func (f *fakePolicyStore) AddAuditEvent(evt model.AuditEvent) (model.AuditEvent, error) {
	if strings.TrimSpace(evt.Action) == "" || strings.TrimSpace(evt.Resource) == "" {
		return model.AuditEvent{}, store.ErrForbiddenState
	}
	if strings.TrimSpace(evt.ID) == "" {
		evt.ID = f.nextID("audit")
	}
	if evt.CreatoIl.IsZero() {
		evt.CreatoIl = time.Now().UTC()
	}
	f.auditEvents = append(f.auditEvents, evt)
	return evt, nil
}
func (f *fakePolicyStore) ListAuditEvents() []model.AuditEvent {
	out := make([]model.AuditEvent, len(f.auditEvents))
	copy(out, f.auditEvents)
	return out
}
func (f *fakePolicyStore) GetAuditEventByID(id string) (model.AuditEvent, error) {
	for _, evt := range f.auditEvents {
		if evt.ID == strings.TrimSpace(id) {
			return evt, nil
		}
	}
	return model.AuditEvent{}, store.ErrNotFound
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
