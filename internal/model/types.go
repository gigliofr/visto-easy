package model

import "time"

type Role string

const (
	RoleRichiedente Role = "RICHIEDENTE"
	RoleOperatore   Role = "OPERATORE"
	RoleSupervisore Role = "SUPERVISORE"
	RoleAdmin       Role = "ADMIN"
)

type StatoPratica string

const (
	StatoBozza                  StatoPratica = "BOZZA"
	StatoInviata                StatoPratica = "INVIATA"
	StatoInLavorazione          StatoPratica = "IN_LAVORAZIONE"
	StatoIntegrazioneRichiesta  StatoPratica = "INTEGRAZIONE_RICHIESTA"
	StatoSospesa                StatoPratica = "SOSPESA"
	StatoApprovata              StatoPratica = "APPROVATA"
	StatoRifiutata              StatoPratica = "RIFIUTATA"
	StatoAttendePagamento       StatoPratica = "ATTENDE_PAGAMENTO"
	StatoPagamentoRicevuto      StatoPratica = "PAGAMENTO_RICEVUTO"
	StatoVistoInElaborazione    StatoPratica = "VISTO_IN_ELABORAZIONE"
	StatoVistoEmesso            StatoPratica = "VISTO_EMESSO"
	StatoCompletata             StatoPratica = "COMPLETATA"
	StatoAnnullata              StatoPratica = "ANNULLATA"
)

type Priorita string

const (
	PrioritaNormale Priorita = "NORMALE"
	PrioritaAlta    Priorita = "ALTA"
	PrioritaUrgente Priorita = "URGENTE"
)

type Utente struct {
	ID              string    `json:"id"`
	Email           string    `json:"email"`
	PasswordHash    string    `json:"-"`
	TOTPSecret      string    `json:"-"`
	TOTPEnabled     bool      `json:"totp_enabled"`
	Nome            string    `json:"nome"`
	Cognome         string    `json:"cognome"`
	Ruolo           Role      `json:"ruolo"`
	Telefono        string    `json:"telefono,omitempty"`
	EmailVerificata bool      `json:"email_verificata"`
	Attivo          bool      `json:"attivo"`
	CreatoIl        time.Time `json:"creato_il"`
	AggiornatoIl    time.Time `json:"aggiornato_il"`
}

type Documento struct {
	ID               string    `json:"id"`
	PraticaID        string    `json:"pratica_id"`
	Tipo             string    `json:"tipo"`
	NomeFile         string    `json:"nome_file"`
	MimeType         string    `json:"mime_type"`
	Dimensione       int64     `json:"dimensione"`
	StatoValidazione string    `json:"stato_validazione"`
	NoteValidazione  string    `json:"note_validazione,omitempty"`
	S3Key            string    `json:"s3_key"`
	CaricatoIl       time.Time `json:"caricato_il"`
}

type EventoPratica struct {
	ID         string         `json:"id"`
	PraticaID  string         `json:"pratica_id"`
	AttoreID   string         `json:"attore_id"`
	TipoEvento string         `json:"tipo_evento"`
	DaStato    StatoPratica   `json:"da_stato,omitempty"`
	AStato     StatoPratica   `json:"a_stato,omitempty"`
	Messaggio  string         `json:"messaggio,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatoIl   time.Time      `json:"creato_il"`
}

type Pratica struct {
	ID               string         `json:"id"`
	Codice           string         `json:"codice"`
	UtenteID         string         `json:"utente_id"`
	OperatoreID      string         `json:"operatore_id,omitempty"`
	TipoVisto        string         `json:"tipo_visto"`
	PaeseDest        string         `json:"paese_dest"`
	Stato            StatoPratica   `json:"stato"`
	Priorita         Priorita       `json:"priorita"`
	NoteInterne      string         `json:"note_interne,omitempty"`
	NoteRichiedente  string         `json:"note_richiedente,omitempty"`
	DatiAnagrafici   map[string]any `json:"dati_anagrafici,omitempty"`
	DatiPassaporto   map[string]any `json:"dati_passaporto,omitempty"`
	ImportoDovuto    float64        `json:"importo_dovuto"`
	Valuta           string         `json:"valuta"`
	CreatoIl         time.Time      `json:"creato_il"`
	AggiornatoIl     time.Time      `json:"aggiornato_il"`
	InviatoIl        *time.Time     `json:"inviato_il,omitempty"`
	CompletatoIl     *time.Time     `json:"completato_il,omitempty"`
	Documenti        []Documento    `json:"documenti,omitempty"`
	Eventi           []EventoPratica `json:"eventi,omitempty"`
}

type PagamentoStato string

const (
	PagamentoPendente   PagamentoStato = "PENDENTE"
	PagamentoCompletato PagamentoStato = "COMPLETATO"
	PagamentoFallito    PagamentoStato = "FALLITO"
	PagamentoRimborsato PagamentoStato = "RIMBORSATO"
)

type Pagamento struct {
	ID                string         `json:"id"`
	PraticaID         string         `json:"pratica_id"`
	Provider          string         `json:"provider"`
	ProviderSessionID string         `json:"provider_session_id"`
	Token             string         `json:"token"`
	Importo           float64        `json:"importo"`
	Valuta            string         `json:"valuta"`
	Stato             PagamentoStato `json:"stato"`
	LinkPagamento     string         `json:"link_pagamento"`
	Scadenza          time.Time      `json:"scadenza"`
	PagatoIl          *time.Time     `json:"pagato_il,omitempty"`
	CreatoIl          time.Time      `json:"creato_il"`
}

type RefreshSession struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Role       string     `json:"role"`
	Revoked    bool       `json:"revoked"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	ReplacedBy string     `json:"replaced_by,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatoIl   time.Time  `json:"creato_il"`
	AggiornatoIl time.Time `json:"aggiornato_il"`
}

type PasswordResetToken struct {
	Token       string     `json:"token"`
	Purpose     string     `json:"purpose,omitempty"`
	UserID      string     `json:"user_id"`
	Email       string     `json:"email"`
	ExpiresAt   time.Time  `json:"expires_at"`
	UsedAt      *time.Time `json:"used_at,omitempty"`
	CreatoIl    time.Time  `json:"creato_il"`
	AggiornatoIl time.Time `json:"aggiornato_il"`
}

type SecurityEvent struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Outcome   string         `json:"outcome"`
	Email     string         `json:"email,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	IP        string         `json:"ip,omitempty"`
	UserAgent string         `json:"user_agent,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatoIl  time.Time      `json:"creato_il"`
}

type AuditEvent struct {
	ID        string         `json:"id"`
	ActorID   string         `json:"actor_id"`
	ActorRole string         `json:"actor_role"`
	Action    string         `json:"action"`
	Resource  string         `json:"resource"`
	ResourceID string        `json:"resource_id,omitempty"`
	IP        string         `json:"ip,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	CreatoIl  time.Time      `json:"creato_il"`
}

type BlockedIP struct {
	IP        string     `json:"ip"`
	Reason    string     `json:"reason"`
	BlockedBy string     `json:"blocked_by"`
	BlockedAt time.Time  `json:"blocked_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type AllowedIP struct {
	IP        string     `json:"ip"`
	Reason    string     `json:"reason"`
	AllowedBy string     `json:"allowed_by"`
	AllowedAt time.Time  `json:"allowed_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
