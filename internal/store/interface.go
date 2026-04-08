package store

import "visto-easy/internal/model"

type DataStore interface {
	CreateUser(u model.Utente) (model.Utente, error)
	GetUserByEmail(email string) (model.Utente, error)
	GetUserByID(id string) (model.Utente, error)
	CreatePratica(p model.Pratica, actorID string) (model.Pratica, error)
	ListPraticheByUser(userID string) []model.Pratica
	ListAllPratiche() []model.Pratica
	GetPratica(id string) (model.Pratica, error)
	UpdatePraticaAsDraft(id, userID string, data map[string]any) (model.Pratica, error)
	DeletePraticaAsDraft(id, userID string) error
	ChangePraticaState(id string, fromActor string, next model.StatoPratica, note string) (model.Pratica, error)
	AddDocumento(praticaID string, d model.Documento) (model.Documento, error)
	ListDocumenti(praticaID string) ([]model.Documento, error)
	CreatePayment(praticaID, provider string, amount float64) (model.Pagamento, error)
	GetPaymentByToken(token string) (model.Pagamento, error)
	ConfirmPaymentByToken(token string) (model.Pagamento, error)
}
