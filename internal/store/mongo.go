package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"visto-easy/internal/model"
)

type MongoStore struct {
	client     *mongo.Client
	db         *mongo.Database
	users      *mongo.Collection
	pratiche   *mongo.Collection
	pagamenti  *mongo.Collection
	counters   *mongo.Collection
}

func NewMongoStore(uri, dbName string) (*MongoStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect failed: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping failed: %w", err)
	}

	db := client.Database(dbName)
	s := &MongoStore{
		client:    client,
		db:        db,
		users:     db.Collection("utenti"),
		pratiche:  db.Collection("pratiche"),
		pagamenti: db.Collection("pagamenti"),
		counters:  db.Collection("counters"),
	}
	if err := s.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}
	if err := s.seedBackofficeUsers(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *MongoStore) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.client.Disconnect(ctx)
}

func (s *MongoStore) ensureIndexes(ctx context.Context) error {
	_, err := s.users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("users index creation failed: %w", err)
	}
	_, err = s.pratiche.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "utenteid", Value: 1}, {Key: "creatoil", Value: -1}}},
		{Keys: bson.D{{Key: "codice", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("pratiche index creation failed: %w", err)
	}
	_, err = s.pagamenti.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "token", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "praticaid", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("pagamenti index creation failed: %w", err)
	}
	return nil
}

func (s *MongoStore) seedBackofficeUsers(ctx context.Context) error {
	now := time.Now().UTC()
	pwd, err := bcrypt.GenerateFromPassword([]byte("ChangeMe123!"), 12)
	if err != nil {
		return err
	}
	seed := []model.Utente{
		{ID: uuid.NewString(), Email: "operatore@vistoeasy.local", PasswordHash: string(pwd), Nome: "Mario", Cognome: "Operatore", Ruolo: model.RoleOperatore, Attivo: true, EmailVerificata: true, CreatoIl: now, AggiornatoIl: now},
		{ID: uuid.NewString(), Email: "supervisore@vistoeasy.local", PasswordHash: string(pwd), Nome: "Luca", Cognome: "Supervisore", Ruolo: model.RoleSupervisore, Attivo: true, EmailVerificata: true, CreatoIl: now, AggiornatoIl: now},
		{ID: uuid.NewString(), Email: "admin@vistoeasy.local", PasswordHash: string(pwd), Nome: "Anna", Cognome: "Admin", Ruolo: model.RoleAdmin, Attivo: true, EmailVerificata: true, CreatoIl: now, AggiornatoIl: now},
	}
	for _, u := range seed {
		filter := bson.M{"email": u.Email}
		update := bson.M{"$setOnInsert": u}
		_, err := s.users.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *MongoStore) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 8*time.Second)
}

func (s *MongoStore) CreateUser(u model.Utente) (model.Utente, error) {
	ctx, cancel := s.ctx()
	defer cancel()
	email := strings.ToLower(strings.TrimSpace(u.Email))
	if email == "" {
		return model.Utente{}, ErrForbiddenState
	}
	if _, err := s.GetUserByEmail(email); err == nil {
		return model.Utente{}, ErrAlreadyExists
	}
	now := time.Now().UTC()
	u.ID = uuid.NewString()
	u.Email = email
	u.Attivo = true
	u.EmailVerificata = true
	u.CreatoIl = now
	u.AggiornatoIl = now
	if _, err := s.users.InsertOne(ctx, u); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.Utente{}, ErrAlreadyExists
		}
		return model.Utente{}, err
	}
	return u, nil
}

func (s *MongoStore) ListUsers() []model.Utente {
	ctx, cancel := s.ctx()
	defer cancel()
	cur, err := s.users.Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"creatoil": -1}))
	if err != nil {
		return []model.Utente{}
	}
	defer cur.Close(ctx)
	var out []model.Utente
	for cur.Next(ctx) {
		var u model.Utente
		if cur.Decode(&u) == nil {
			out = append(out, u)
		}
	}
	return out
}

func (s *MongoStore) GetUserByEmail(email string) (model.Utente, error) {
	ctx, cancel := s.ctx()
	defer cancel()
	var u model.Utente
	err := s.users.FindOne(ctx, bson.M{"email": strings.ToLower(strings.TrimSpace(email))}).Decode(&u)
	if errorsIsNotFound(err) {
		return model.Utente{}, ErrNotFound
	}
	return u, err
}

func (s *MongoStore) GetUserByID(id string) (model.Utente, error) {
	ctx, cancel := s.ctx()
	defer cancel()
	var u model.Utente
	err := s.users.FindOne(ctx, bson.M{"id": id}).Decode(&u)
	if errorsIsNotFound(err) {
		return model.Utente{}, ErrNotFound
	}
	return u, err
}

func (s *MongoStore) nextPraticaSeq(ctx context.Context) (int64, error) {
	opt := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	update := bson.M{"$inc": bson.M{"seq": 1}}
	var out struct {
		Seq int64 `bson:"seq"`
	}
	err := s.counters.FindOneAndUpdate(ctx, bson.M{"_id": "pratica"}, update, opt).Decode(&out)
	if err != nil {
		return 0, err
	}
	return out.Seq, nil
}

func (s *MongoStore) CreatePratica(p model.Pratica, actorID string) (model.Pratica, error) {
	ctx, cancel := s.ctx()
	defer cancel()
	now := time.Now().UTC()
	seq, err := s.nextPraticaSeq(ctx)
	if err != nil {
		return model.Pratica{}, err
	}
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
	if _, err := s.pratiche.InsertOne(ctx, p); err != nil {
		return model.Pratica{}, err
	}
	return p, nil
}

func (s *MongoStore) ListPraticheByUser(userID string) []model.Pratica {
	ctx, cancel := s.ctx()
	defer cancel()
	cur, err := s.pratiche.Find(ctx, bson.M{"utenteid": userID}, options.Find().SetSort(bson.M{"creatoil": -1}))
	if err != nil {
		return []model.Pratica{}
	}
	defer cur.Close(ctx)
	var out []model.Pratica
	for cur.Next(ctx) {
		var p model.Pratica
		if cur.Decode(&p) == nil {
			out = append(out, p)
		}
	}
	return out
}

func (s *MongoStore) ListAllPratiche() []model.Pratica {
	ctx, cancel := s.ctx()
	defer cancel()
	cur, err := s.pratiche.Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"creatoil": -1}))
	if err != nil {
		return []model.Pratica{}
	}
	defer cur.Close(ctx)
	var out []model.Pratica
	for cur.Next(ctx) {
		var p model.Pratica
		if cur.Decode(&p) == nil {
			out = append(out, p)
		}
	}
	return out
}

func (s *MongoStore) GetPratica(id string) (model.Pratica, error) {
	ctx, cancel := s.ctx()
	defer cancel()
	var p model.Pratica
	err := s.pratiche.FindOne(ctx, bson.M{"id": id}).Decode(&p)
	if errorsIsNotFound(err) {
		return model.Pratica{}, ErrNotFound
	}
	return p, err
}

func (s *MongoStore) UpdatePraticaAsDraft(id, userID string, data map[string]any) (model.Pratica, error) {
	p, err := s.GetPratica(id)
	if err != nil {
		return model.Pratica{}, err
	}
	if p.UtenteID != userID || p.Stato != model.StatoBozza {
		return model.Pratica{}, ErrForbiddenState
	}
	if v, ok := data["tipo_visto"].(string); ok && strings.TrimSpace(v) != "" { p.TipoVisto = v }
	if v, ok := data["paese_dest"].(string); ok && strings.TrimSpace(v) != "" { p.PaeseDest = v }
	if v, ok := data["dati_anagrafici"].(map[string]any); ok { p.DatiAnagrafici = v }
	if v, ok := data["dati_passaporto"].(map[string]any); ok { p.DatiPassaporto = v }
	p.AggiornatoIl = time.Now().UTC()

	ctx, cancel := s.ctx()
	defer cancel()
	_, err = s.pratiche.ReplaceOne(ctx, bson.M{"id": id}, p)
	if err != nil {
		return model.Pratica{}, err
	}
	return p, nil
}

func (s *MongoStore) DeletePraticaAsDraft(id, userID string) error {
	ctx, cancel := s.ctx()
	defer cancel()
	res, err := s.pratiche.DeleteOne(ctx, bson.M{"id": id, "utenteid": userID, "stato": model.StatoBozza})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrForbiddenState
	}
	return nil
}

func (s *MongoStore) SubmitPratica(id, userID string) (model.Pratica, error) {
	p, err := s.GetPratica(id)
	if err != nil {
		return model.Pratica{}, err
	}
	if p.UtenteID != userID || p.Stato != model.StatoBozza {
		return model.Pratica{}, ErrForbiddenState
	}
	return s.ChangePraticaState(id, userID, model.StatoInviata, "pratica inviata dal richiedente")
}

func (s *MongoStore) ChangePraticaState(id string, fromActor string, next model.StatoPratica, note string) (model.Pratica, error) {
	p, err := s.GetPratica(id)
	if err != nil {
		return model.Pratica{}, err
	}
	if !allowedTransitions[p.Stato][next] {
		return model.Pratica{}, ErrInvalidState
	}
	now := time.Now().UTC()
	evt := model.EventoPratica{ID: uuid.NewString(), PraticaID: id, AttoreID: fromActor, TipoEvento: "STATO_CAMBIATO", DaStato: p.Stato, AStato: next, Messaggio: note, CreatoIl: now}
	p.Stato = next
	p.AggiornatoIl = now
	if next == model.StatoInviata { p.InviatoIl = &now }
	if next == model.StatoCompletata { p.CompletatoIl = &now }
	p.Eventi = append(p.Eventi, evt)

	ctx, cancel := s.ctx()
	defer cancel()
	_, err = s.pratiche.ReplaceOne(ctx, bson.M{"id": id}, p)
	if err != nil {
		return model.Pratica{}, err
	}
	return p, nil
}

func (s *MongoStore) AssignOperatore(praticaID, operatoreID, actorID string) (model.Pratica, error) {
	p, err := s.GetPratica(praticaID)
	if err != nil {
		return model.Pratica{}, err
	}
	if _, err := s.GetUserByID(operatoreID); err != nil {
		return model.Pratica{}, ErrNotFound
	}
	now := time.Now().UTC()
	p.OperatoreID = operatoreID
	p.AggiornatoIl = now
	p.Eventi = append(p.Eventi, model.EventoPratica{
		ID:         uuid.NewString(),
		PraticaID:  p.ID,
		AttoreID:   actorID,
		TipoEvento: "ASSEGNAZIONE_OPERATORE",
		Messaggio:  "operatore assegnato",
		Metadata: map[string]any{
			"operatore_id": operatoreID,
		},
		CreatoIl: now,
	})

	ctx, cancel := s.ctx()
	defer cancel()
	_, err = s.pratiche.ReplaceOne(ctx, bson.M{"id": praticaID}, p)
	if err != nil {
		return model.Pratica{}, err
	}
	return p, nil
}

func (s *MongoStore) AddNota(praticaID, actorID, message string, internal bool) (model.Pratica, error) {
	p, err := s.GetPratica(praticaID)
	if err != nil {
		return model.Pratica{}, err
	}
	now := time.Now().UTC()
	nota := strings.TrimSpace(message)
	if nota == "" {
		return model.Pratica{}, ErrForbiddenState
	}
	if internal {
		if p.NoteInterne != "" {
			p.NoteInterne += "\n"
		}
		p.NoteInterne += now.Format(time.RFC3339) + " | " + nota
	} else {
		if p.NoteRichiedente != "" {
			p.NoteRichiedente += "\n"
		}
		p.NoteRichiedente += now.Format(time.RFC3339) + " | " + nota
	}
	p.AggiornatoIl = now
	p.Eventi = append(p.Eventi, model.EventoPratica{
		ID:         uuid.NewString(),
		PraticaID:  p.ID,
		AttoreID:   actorID,
		TipoEvento: "NOTA_AGGIUNTA",
		Messaggio:  nota,
		Metadata: map[string]any{
			"scope": map[bool]string{true: "interna", false: "richiedente"}[internal],
		},
		CreatoIl: now,
	})

	ctx, cancel := s.ctx()
	defer cancel()
	_, err = s.pratiche.ReplaceOne(ctx, bson.M{"id": praticaID}, p)
	if err != nil {
		return model.Pratica{}, err
	}
	return p, nil
}

func (s *MongoStore) RequestDocumento(praticaID, actorID, documento, note string) (model.Pratica, error) {
	p, err := s.GetPratica(praticaID)
	if err != nil {
		return model.Pratica{}, err
	}
	if p.Stato != model.StatoInLavorazione && p.Stato != model.StatoSospesa {
		return model.Pratica{}, ErrInvalidState
	}
	p, err = s.ChangePraticaState(praticaID, actorID, model.StatoIntegrazioneRichiesta, "richiesta integrazione documentale")
	if err != nil {
		return model.Pratica{}, err
	}
	msg := strings.TrimSpace(documento)
	if strings.TrimSpace(note) != "" {
		msg += " - " + strings.TrimSpace(note)
	}
	if msg == "" {
		msg = "documento aggiuntivo richiesto"
	}
	return s.AddNota(praticaID, actorID, msg, false)
}

func (s *MongoStore) AddDocumento(praticaID string, d model.Documento) (model.Documento, error) {
	p, err := s.GetPratica(praticaID)
	if err != nil {
		return model.Documento{}, err
	}
	d.ID = uuid.NewString()
	d.PraticaID = praticaID
	d.CaricatoIl = time.Now().UTC()
	d.StatoValidazione = "PENDING"
	d.S3Key = fmt.Sprintf("pratiche/%s/documenti/%s_%s", praticaID, d.ID, d.NomeFile)
	p.Documenti = append(p.Documenti, d)
	p.AggiornatoIl = time.Now().UTC()

	ctx, cancel := s.ctx()
	defer cancel()
	_, err = s.pratiche.ReplaceOne(ctx, bson.M{"id": praticaID}, p)
	if err != nil {
		return model.Documento{}, err
	}
	return d, nil
}

func (s *MongoStore) ListDocumenti(praticaID string) ([]model.Documento, error) {
	p, err := s.GetPratica(praticaID)
	if err != nil {
		return nil, err
	}
	return p.Documenti, nil
}

func (s *MongoStore) CreatePayment(praticaID, provider string, amount float64) (model.Pagamento, error) {
	if _, err := s.GetPratica(praticaID); err != nil {
		return model.Pagamento{}, err
	}
	now := time.Now().UTC()
	token := strings.ReplaceAll(uuid.NewString(), "-", "")
	pay := model.Pagamento{
		ID: uuid.NewString(), PraticaID: praticaID, Provider: provider,
		ProviderSessionID: "sess_" + token[:12], Token: token,
		Importo: amount, Valuta: "EUR", Stato: model.PagamentoPendente,
		LinkPagamento: "/pagamento/" + token, Scadenza: now.Add(48 * time.Hour), CreatoIl: now,
	}
	ctx, cancel := s.ctx()
	defer cancel()
	if _, err := s.pagamenti.InsertOne(ctx, pay); err != nil {
		return model.Pagamento{}, err
	}
	return pay, nil
}

func (s *MongoStore) GetPaymentByToken(token string) (model.Pagamento, error) {
	ctx, cancel := s.ctx()
	defer cancel()
	var p model.Pagamento
	err := s.pagamenti.FindOne(ctx, bson.M{"token": token}).Decode(&p)
	if errorsIsNotFound(err) {
		return model.Pagamento{}, ErrNotFound
	}
	return p, err
}

func (s *MongoStore) ConfirmPaymentByToken(token string) (model.Pagamento, error) {
	ctx, cancel := s.ctx()
	defer cancel()
	now := time.Now().UTC()
	res, err := s.pagamenti.UpdateOne(ctx, bson.M{"token": token}, bson.M{"$set": bson.M{"stato": model.PagamentoCompletato, "pagatoil": now}})
	if err != nil {
		return model.Pagamento{}, err
	}
	if res.MatchedCount == 0 {
		return model.Pagamento{}, ErrNotFound
	}
	return s.GetPaymentByToken(token)
}

func errorsIsNotFound(err error) bool {
	return err == mongo.ErrNoDocuments
}
