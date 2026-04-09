package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"

	"visto-easy/internal/model"
)

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func pick[T any](items []T, idx int) T {
	return items[idx%len(items)]
}

func main() {
	uri := flag.String("uri", envOr("MONGODB_URI", envOr("DATABASE_URL", envOr("MONGO_URL", envOr("MONGODB_URL", "")))), "MongoDB connection string")
	dbName := flag.String("db", envOr("MONGODB_DB_NAME", envOr("DATABASE_NAME", "visto-easy")), "MongoDB database name")
	usersCount := flag.Int("users", 10, "Number of richiedente users to create")
	praticheCount := flag.Int("pratiche", 100, "Number of practices to create")
	password := flag.String("password", envOr("SEED_USER_PASSWORD", "User123!Demo"), "Plain-text password for generated users")
	prefix := flag.String("prefix", envOr("SEED_PREFIX", "seed"), "Prefix for generated emails and practice codes")
	flag.Parse()

	if strings.TrimSpace(*uri) == "" {
		log.Fatal("MongoDB URI mancante: passa -uri o imposta MONGODB_URI")
	}
	if *usersCount <= 0 {
		log.Fatal("-users deve essere > 0")
	}
	if *praticheCount <= 0 {
		log.Fatal("-pratiche deve essere > 0")
	}
	if len(strings.TrimSpace(*password)) < 8 {
		log.Fatal("la password deve avere almeno 8 caratteri")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(*password)), 12)
	if err != nil {
		log.Fatalf("errore hash password: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*uri))
	if err != nil {
		log.Fatalf("connessione Mongo fallita: %v", err)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("ping Mongo fallito: %v", err)
	}

	db := client.Database(*dbName)
	usersCol := db.Collection("utenti")
	praticheCol := db.Collection("pratiche")

	operatorIDs, err := loadActiveOperatorIDs(ctx, usersCol)
	if err != nil {
		log.Fatalf("errore lettura operatori: %v", err)
	}

	userIDs := make([]string, 0, *usersCount)
	now := time.Now().UTC()
	cleanPrefix := strings.ToLower(strings.TrimSpace(*prefix))
	if cleanPrefix == "" {
		cleanPrefix = "seed"
	}

	for i := 1; i <= *usersCount; i++ {
		email := fmt.Sprintf("%s.user%02d@vistoeasy.local", cleanPrefix, i)
		firstName := fmt.Sprintf("Seed%02d", i)
		lastName := "User"

		filter := bson.M{"email": email}
		update := bson.M{
			"$setOnInsert": bson.M{
				"id":       uuid.NewString(),
				"creatoil": now,
			},
			"$set": bson.M{
				"email":           email,
				"passwordhash":    string(hash),
				"nome":            firstName,
				"cognome":         lastName,
				"ruolo":           model.RoleRichiedente,
				"attivo":          true,
				"emailverificata": true,
				"aggiornatoil":    now,
			},
		}
		if _, err := usersCol.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
			log.Fatalf("errore upsert utente %s: %v", email, err)
		}

		var row struct {
			ID string `bson:"id"`
		}
		if err := usersCol.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"id": 1})).Decode(&row); err != nil {
			log.Fatalf("errore lettura id utente %s: %v", email, err)
		}
		userIDs = append(userIDs, row.ID)
	}

	states := []model.StatoPratica{
		model.StatoBozza,
		model.StatoInviata,
		model.StatoInLavorazione,
		model.StatoIntegrazioneRichiesta,
		model.StatoSospesa,
		model.StatoApprovata,
		model.StatoRifiutata,
		model.StatoAttendePagamento,
		model.StatoPagamentoRicevuto,
		model.StatoVistoInElaborazione,
		model.StatoVistoEmesso,
		model.StatoCompletata,
	}
	priorities := []model.Priorita{model.PrioritaNormale, model.PrioritaAlta, model.PrioritaUrgente}
	visti := []string{"TURISMO", "STUDIO", "LAVORO", "BUSINESS", "TRANSITO"}
	paesi := []string{"IT", "US", "GB", "CA", "AU", "JP", "AE", "SG"}

	for i := 1; i <= *praticheCount; i++ {
		utenteID := pick(userIDs, i-1)
		state := pick(states, i-1)
		priority := pick(priorities, i-1)
		tipoVisto := pick(visti, i-1)
		paese := pick(paesi, i-1)

		createdAt := now.Add(-time.Duration(i) * 12 * time.Hour)
		updatedAt := createdAt.Add(time.Duration((i%9)+1) * time.Hour)

		var inviatoIl any
		if state != model.StatoBozza {
			inviatoIl = createdAt.Add(2 * time.Hour)
		}

		var completatoIl any
		if state == model.StatoCompletata || state == model.StatoVistoEmesso || state == model.StatoApprovata {
			completatoIl = updatedAt
		}

		operatorID := ""
		if len(operatorIDs) > 0 && state != model.StatoBozza && state != model.StatoInviata {
			operatorID = pick(operatorIDs, i-1)
		}

		practiceID := uuid.NewString()
		code := fmt.Sprintf("VST-%d-%s-%06d", now.Year(), strings.ToUpper(cleanPrefix), i)
		filter := bson.M{"codice": code}
		update := bson.M{
			"$setOnInsert": bson.M{
				"id":       practiceID,
				"codice":   code,
				"utenteid": utenteID,
				"creatoil": createdAt,
			},
			"$set": bson.M{
				"stato":           state,
				"priorita":        priority,
				"tipovisto":       tipoVisto,
				"paesedest":       paese,
				"operatoreid":     operatorID,
				"noteinterne":     fmt.Sprintf("seed note %d", i),
				"noterichiedente": fmt.Sprintf("richiesta seed %d", i),
				"datianagrafici": bson.M{
					"nome":    fmt.Sprintf("Seed%02d", ((i-1)%*usersCount)+1),
					"cognome": "User",
				},
				"datipassaporto": bson.M{
					"numero": fmt.Sprintf("P%08d", i),
				},
				"importodovuto": float64(50 + (i % 150)),
				"valuta":        "EUR",
				"aggiornatoil":  updatedAt,
				"inviatoil":     inviatoIl,
				"completatoil":  completatoIl,
				"eventi": []bson.M{
					{
						"id":         uuid.NewString(),
						"praticaid":  practiceID,
						"attoreid":   utenteID,
						"tipoevento": "SEED_DATA",
						"astato":     state,
						"messaggio":  "record creato via seed script",
						"creatoil":   updatedAt,
					},
				},
			},
		}

		if _, err := praticheCol.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
			log.Fatalf("errore upsert pratica %s: %v", code, err)
		}
	}

	fmt.Println("Seed completato con successo")
	fmt.Printf("Utenti richiedenti: %d\n", *usersCount)
	fmt.Printf("Pratiche: %d\n", *praticheCount)
	fmt.Printf("Prefix: %s\n", cleanPrefix)
	fmt.Printf("Database: %s\n", *dbName)
}

func loadActiveOperatorIDs(ctx context.Context, usersCol *mongo.Collection) ([]string, error) {
	cur, err := usersCol.Find(ctx, bson.M{
		"ruolo":           model.RoleOperatore,
		"attivo":          true,
		"emailverificata": true,
	}, options.Find().SetProjection(bson.M{"id": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	ids := make([]string, 0)
	for cur.Next(ctx) {
		var row struct {
			ID string `bson:"id"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		if strings.TrimSpace(row.ID) != "" {
			ids = append(ids, row.ID)
		}
	}
	return ids, cur.Err()
}