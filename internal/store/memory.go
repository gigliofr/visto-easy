package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"visto-easy/internal/model"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrAlreadyExists  = errors.New("already exists")
	ErrInvalidState   = errors.New("invalid state transition")
	ErrForbiddenState = errors.New("forbidden state change")
)

type MemoryStore struct {
	mu        sync.RWMutex
	users     map[string]model.Utente
	pratiche  map[string]model.Pratica
	payments  map[string]model.Pagamento
	webhooks  map[string]time.Time
	securityEvents []model.SecurityEvent
	byEmail   map[string]string
	counters  map[string]*atomic.Uint64
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		users:    make(map[string]model.Utente),
		pratiche: make(map[string]model.Pratica),
		payments: make(map[string]model.Pagamento),
		webhooks: make(map[string]time.Time),
		securityEvents: make([]model.SecurityEvent, 0, 128),
		byEmail:  make(map[string]string),
		counters: map[string]*atomic.Uint64{"pratica": {}},
	}
	s.seedBackofficeUsers()
	return s
}

func (s *MemoryStore) seedBackofficeUsers() {
	now := time.Now().UTC()
	for _, u := range []model.Utente{
		{ID: uuid.NewString(), Email: "operatore@vistoeasy.local", Nome: "Mario", Cognome: "Operatore", Ruolo: model.RoleOperatore, Attivo: true, EmailVerificata: true, CreatoIl: now, AggiornatoIl: now},
		{ID: uuid.NewString(), Email: "supervisore@vistoeasy.local", Nome: "Luca", Cognome: "Supervisore", Ruolo: model.RoleSupervisore, Attivo: true, EmailVerificata: true, CreatoIl: now, AggiornatoIl: now},
		{ID: uuid.NewString(), Email: "admin@vistoeasy.local", Nome: "Anna", Cognome: "Admin", Ruolo: model.RoleAdmin, Attivo: true, EmailVerificata: true, CreatoIl: now, AggiornatoIl: now},
	} {
		u.PasswordHash = ""
		s.users[u.ID] = u
		s.byEmail[strings.ToLower(u.Email)] = u.ID
	}
}

func (s *MemoryStore) CreateUser(u model.Utente) (model.Utente, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	emailKey := strings.ToLower(strings.TrimSpace(u.Email))
	if _, exists := s.byEmail[emailKey]; exists {
		return model.Utente{}, ErrAlreadyExists
	}
	now := time.Now().UTC()
	u.ID = uuid.NewString()
	u.Email = emailKey
	u.Attivo = true
	u.EmailVerificata = true
	u.CreatoIl = now
	u.AggiornatoIl = now
	s.users[u.ID] = u
	s.byEmail[emailKey] = u.ID
	return u, nil
}

func (s *MemoryStore) GetUserByEmail(email string) (model.Utente, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return model.Utente{}, ErrNotFound
	}
	return s.users[id], nil
}

func (s *MemoryStore) GetUserByID(id string) (model.Utente, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return model.Utente{}, ErrNotFound
	}
	return u, nil
}

func (s *MemoryStore) CreatePratica(p model.Pratica, actorID string) (model.Pratica, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	seq := s.counters["pratica"].Add(1)
	p.ID = uuid.NewString()
	p.Codice = fmt.Sprintf("VST-%d-%06d", now.Year(), seq)
	p.Stato = model.StatoBozza
	p.Priorita = model.PrioritaNormale
	p.Valuta = "EUR"
	p.CreatoIl = now
	p.AggiornatoIl = now
	p.Eventi = []model.EventoPratica{{
		ID: uuid.NewString(), PraticaID: p.ID, AttoreID: actorID,
		TipoEvento: "PRATICA_CREATA", AStato: model.StatoBozza, CreatoIl: now,
	}}
	s.pratiche[p.ID] = p
	return p, nil
}

func (s *MemoryStore) ListPraticheByUser(userID string) []model.Pratica {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Pratica, 0)
	for _, p := range s.pratiche {
		if p.UtenteID == userID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatoIl.After(out[j].CreatoIl) })
	return out
}

func (s *MemoryStore) ListAllPratiche() []model.Pratica {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Pratica, 0, len(s.pratiche))
	for _, p := range s.pratiche {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatoIl.After(out[j].CreatoIl) })
	return out
}

func (s *MemoryStore) GetPratica(id string) (model.Pratica, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pratiche[id]
	if !ok {
		return model.Pratica{}, ErrNotFound
	}
	return p, nil
}

func (s *MemoryStore) UpdatePraticaAsDraft(id, userID string, data map[string]any) (model.Pratica, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pratiche[id]
	if !ok {
		return model.Pratica{}, ErrNotFound
	}
	if p.UtenteID != userID {
		return model.Pratica{}, ErrForbiddenState
	}
	if p.Stato != model.StatoBozza {
		return model.Pratica{}, ErrForbiddenState
	}
	if v, ok := data["tipo_visto"].(string); ok && strings.TrimSpace(v) != "" { p.TipoVisto = v }
	if v, ok := data["paese_dest"].(string); ok && strings.TrimSpace(v) != "" { p.PaeseDest = v }
	if v, ok := data["dati_anagrafici"].(map[string]any); ok { p.DatiAnagrafici = v }
	if v, ok := data["dati_passaporto"].(map[string]any); ok { p.DatiPassaporto = v }
	p.AggiornatoIl = time.Now().UTC()
	s.pratiche[id] = p
	return p, nil
}

func (s *MemoryStore) DeletePraticaAsDraft(id, userID string) error {
	s.mu.Lock(); defer s.mu.Unlock()
	p, ok := s.pratiche[id]
	if !ok { return ErrNotFound }
	if p.UtenteID != userID || p.Stato != model.StatoBozza { return ErrForbiddenState }
	delete(s.pratiche, id)
	return nil
}

var allowedTransitions = map[model.StatoPratica]map[model.StatoPratica]bool{
	model.StatoBozza: {model.StatoAnnullata: true, model.StatoInviata: true},
	model.StatoInviata: {model.StatoInLavorazione: true, model.StatoRifiutata: true},
	model.StatoInLavorazione: {model.StatoSospesa: true, model.StatoIntegrazioneRichiesta: true, model.StatoApprovata: true, model.StatoRifiutata: true},
	model.StatoIntegrazioneRichiesta: {model.StatoApprovata: true, model.StatoInLavorazione: true},
	model.StatoSospesa: {model.StatoInLavorazione: true, model.StatoRifiutata: true},
	model.StatoApprovata: {model.StatoAttendePagamento: true},
	model.StatoAttendePagamento: {model.StatoPagamentoRicevuto: true},
	model.StatoPagamentoRicevuto: {model.StatoVistoInElaborazione: true},
	model.StatoVistoInElaborazione: {model.StatoVistoEmesso: true},
	model.StatoVistoEmesso: {model.StatoCompletata: true},
}

func (s *MemoryStore) ChangePraticaState(id string, fromActor string, next model.StatoPratica, note string) (model.Pratica, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	p, ok := s.pratiche[id]
	if !ok { return model.Pratica{}, ErrNotFound }
	if !allowedTransitions[p.Stato][next] { return model.Pratica{}, ErrInvalidState }
	now := time.Now().UTC()
	evt := model.EventoPratica{ID: uuid.NewString(), PraticaID: id, AttoreID: fromActor, TipoEvento: "STATO_CAMBIATO", DaStato: p.Stato, AStato: next, Messaggio: note, CreatoIl: now}
	p.Stato = next
	p.AggiornatoIl = now
	if next == model.StatoInviata { p.InviatoIl = &now }
	if next == model.StatoCompletata { p.CompletatoIl = &now }
	p.Eventi = append(p.Eventi, evt)
	s.pratiche[id] = p
	return p, nil
}

func (s *MemoryStore) AddDocumento(praticaID string, d model.Documento) (model.Documento, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	p, ok := s.pratiche[praticaID]
	if !ok { return model.Documento{}, ErrNotFound }
	d.ID = uuid.NewString()
	d.PraticaID = praticaID
	d.CaricatoIl = time.Now().UTC()
	d.StatoValidazione = "PENDING"
	d.S3Key = fmt.Sprintf("pratiche/%s/documenti/%s_%s", praticaID, d.ID, d.NomeFile)
	p.Documenti = append(p.Documenti, d)
	p.AggiornatoIl = time.Now().UTC()
	s.pratiche[praticaID] = p
	return d, nil
}

func (s *MemoryStore) ListDocumenti(praticaID string) ([]model.Documento, error) {
	s.mu.RLock(); defer s.mu.RUnlock()
	p, ok := s.pratiche[praticaID]
	if !ok { return nil, ErrNotFound }
	return p.Documenti, nil
}

func (s *MemoryStore) CreatePayment(praticaID, provider string, amount float64) (model.Pagamento, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	if _, ok := s.pratiche[praticaID]; !ok { return model.Pagamento{}, ErrNotFound }
	now := time.Now().UTC()
	token := strings.ReplaceAll(uuid.NewString(), "-", "")
	pay := model.Pagamento{
		ID: uuid.NewString(), PraticaID: praticaID, Provider: provider,
		ProviderSessionID: "sess_" + token[:12], Token: token,
		Importo: amount, Valuta: "EUR", Stato: model.PagamentoPendente,
		LinkPagamento: "/pagamento/" + token, Scadenza: now.Add(48 * time.Hour), CreatoIl: now,
	}
	s.payments[pay.ID] = pay
	return pay, nil
}

func (s *MemoryStore) GetPaymentByToken(token string) (model.Pagamento, error) {
	s.mu.RLock(); defer s.mu.RUnlock()
	for _, p := range s.payments {
		if p.Token == token { return p, nil }
	}
	return model.Pagamento{}, ErrNotFound
}

func (s *MemoryStore) ConfirmPaymentByToken(token string) (model.Pagamento, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	for id, p := range s.payments {
		if p.Token != token { continue }
		now := time.Now().UTC()
		p.Stato = model.PagamentoCompletato
		p.PagatoIl = &now
		s.payments[id] = p
		return p, nil
	}
	return model.Pagamento{}, ErrNotFound
}

func (s *MemoryStore) MarkWebhookEventProcessed(provider, eventID, paymentID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(provider)) + ":" + strings.TrimSpace(eventID)
	if key == ":" {
		return false, ErrForbiddenState
	}
	if _, ok := s.webhooks[key]; ok {
		return true, nil
	}
	s.webhooks[key] = time.Now().UTC()
	return false, nil
}

func (s *MemoryStore) AddSecurityEvent(evt model.SecurityEvent) (model.SecurityEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(evt.Type) == "" {
		return model.SecurityEvent{}, ErrForbiddenState
	}
	if strings.TrimSpace(evt.ID) == "" {
		evt.ID = uuid.NewString()
	}
	if evt.CreatoIl.IsZero() {
		evt.CreatoIl = time.Now().UTC()
	}
	s.securityEvents = append(s.securityEvents, evt)
	return evt, nil
}

func (s *MemoryStore) ListSecurityEvents() []model.SecurityEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.SecurityEvent, len(s.securityEvents))
	copy(out, s.securityEvents)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatoIl.After(out[j].CreatoIl) })
	return out
}

func (s *MemoryStore) GetSecurityEventByID(id string) (model.SecurityEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, evt := range s.securityEvents {
		if evt.ID == id {
			return evt, nil
		}
	}
	return model.SecurityEvent{}, ErrNotFound
}
