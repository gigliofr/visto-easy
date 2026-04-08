package store

import "visto-easy/internal/model"

type DataStore interface {
	CreateUser(u model.Utente) (model.Utente, error)
	ListUsers() []model.Utente
	GetUserByEmail(email string) (model.Utente, error)
	GetUserByID(id string) (model.Utente, error)
	CreatePratica(p model.Pratica, actorID string) (model.Pratica, error)
	ListPraticheByUser(userID string) []model.Pratica
	ListAllPratiche() []model.Pratica
	GetPratica(id string) (model.Pratica, error)
	UpdatePraticaAsDraft(id, userID string, data map[string]any) (model.Pratica, error)
	DeletePraticaAsDraft(id, userID string) error
	SubmitPratica(id, userID string) (model.Pratica, error)
	ChangePraticaState(id string, fromActor string, next model.StatoPratica, note string) (model.Pratica, error)
	AssignOperatore(praticaID, operatoreID, actorID string) (model.Pratica, error)
	AddNota(praticaID, actorID, message string, internal bool) (model.Pratica, error)
	RequestDocumento(praticaID, actorID, documento, note string) (model.Pratica, error)
	AddDocumento(praticaID string, d model.Documento) (model.Documento, error)
	ListDocumenti(praticaID string) ([]model.Documento, error)
	GetDocumento(praticaID, documentoID string) (model.Documento, error)
	DeleteDocumento(praticaID, documentoID string) (bool, error)
	CreatePayment(praticaID, provider string, amount float64) (model.Pagamento, error)
	GetPaymentByToken(token string) (model.Pagamento, error)
	ConfirmPaymentByToken(token string) (model.Pagamento, error)
	MarkWebhookEventProcessed(provider, eventID, paymentID string) (bool, error)
	AddSecurityEvent(evt model.SecurityEvent) (model.SecurityEvent, error)
	ListSecurityEvents() []model.SecurityEvent
	GetSecurityEventByID(id string) (model.SecurityEvent, error)
	UpsertBlockedIP(entry model.BlockedIP) (model.BlockedIP, error)
	RemoveBlockedIP(ip string) (bool, error)
	ListBlockedIPs() []model.BlockedIP
	GetBlockedIP(ip string) (model.BlockedIP, error)
	UpsertAllowedIP(entry model.AllowedIP) (model.AllowedIP, error)
	RemoveAllowedIP(ip string) (bool, error)
	ListAllowedIPs() []model.AllowedIP
	GetAllowedIP(ip string) (model.AllowedIP, error)
}
