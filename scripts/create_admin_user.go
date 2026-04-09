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

func main() {
	uri := flag.String("uri", envOr("MONGODB_URI", envOr("DATABASE_URL", envOr("MONGO_URL", envOr("MONGODB_URL", "")))), "MongoDB connection string")
	dbName := flag.String("db", envOr("MONGODB_DB_NAME", envOr("DATABASE_NAME", "visto-easy")), "MongoDB database name")
	email := flag.String("email", envOr("ADMIN_EMAIL", "admin@vistoeasy.local"), "Admin email address")
	password := flag.String("password", envOr("ADMIN_PASSWORD", envOr("BACKOFFICE_SEED_PASSWORD", "Admin123!Change")), "Admin plain-text password")
	nome := flag.String("nome", envOr("ADMIN_NOME", "Anna"), "Admin first name")
	cognome := flag.String("cognome", envOr("ADMIN_COGNOME", "Admin"), "Admin last name")
	flag.Parse()

	if strings.TrimSpace(*uri) == "" {
		log.Fatal("MongoDB URI mancante: imposta MONGODB_URI o passa -uri")
	}
	if len(strings.TrimSpace(*password)) < 8 {
		log.Fatal("La password deve avere almeno 8 caratteri")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(*password)), 12)
	if err != nil {
		log.Fatalf("impossibile generare hash bcrypt: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	users := client.Database(*dbName).Collection("utenti")
	now := time.Now().UTC()
	filter := bson.M{"email": strings.ToLower(strings.TrimSpace(*email))}
	update := bson.M{
		"$setOnInsert": bson.M{
			"id":              uuid.NewString(),
			"creatoil":        now,
			"emailverificata": true,
		},
		"$set": bson.M{
			"email":           strings.ToLower(strings.TrimSpace(*email)),
			"passwordhash":    string(hash),
			"nome":            strings.TrimSpace(*nome),
			"cognome":         strings.TrimSpace(*cognome),
			"ruolo":           model.RoleAdmin,
			"attivo":          true,
			"emailverificata": true,
			"aggiornatoil":    now,
		},
	}

	res, err := users.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		log.Fatalf("scrittura utente admin fallita: %v", err)
	}

	action := "aggiornato"
	if res.UpsertedCount > 0 {
		action = "creato"
	}

	fmt.Printf("Utente admin %s con successo\n", action)
	fmt.Printf("Email: %s\n", strings.ToLower(strings.TrimSpace(*email)))
	fmt.Printf("Ruolo: %s\n", model.RoleAdmin)
	fmt.Printf("Database: %s\n", *dbName)
	fmt.Println("Password: impostata correttamente")
}