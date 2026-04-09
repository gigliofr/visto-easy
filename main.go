package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"visto-easy/internal/auth"
	"visto-easy/internal/httpapi"
	"visto-easy/internal/storage"
	"visto-easy/internal/store"
)

type closableStore interface {
	store.DataStore
	Close() error
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = os.Getenv("SESSION_SECRET")
		if len(jwtSecret) < 32 && strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT"))) != "production" {
			jwtSecret = "vistoeasy-local-dev-secret-key-2026-verylong"
			log.Printf("JWT secret missing: using local development default secret")
		}
	}
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET (or SESSION_SECRET) is required and must be >= 32 chars")
	}
	mongoURI := firstNonEmpty(os.Getenv("MONGODB_URI"), os.Getenv("DATABASE_URL"), os.Getenv("MONGO_URL"), os.Getenv("MONGODB_URL"))
	if strings.TrimSpace(mongoURI) == "" {
		if strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT"))) == "production" {
			log.Fatal("MONGODB_URI is required (aliases: DATABASE_URL, MONGO_URL, MONGODB_URL)")
		}
	}
	mongoDBName := strings.TrimSpace(firstNonEmpty(os.Getenv("MONGODB_DB_NAME"), os.Getenv("DATABASE_NAME")))
	if mongoDBName == "" {
		mongoDBName = "visto-easy"
	}

	tm, err := auth.NewTokenManager(jwtSecret)
	if err != nil {
		log.Fatalf("token manager init failed: %v", err)
	}
	var st closableStore
	if strings.TrimSpace(mongoURI) == "" {
		log.Printf("mongo URI missing: using in-memory store for local development")
		st = store.NewMemoryStore()
	} else {
		mongoStore, err := store.NewMongoStore(mongoURI, mongoDBName)
		if err != nil {
			if strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT"))) == "production" {
				log.Fatalf("mongo store init failed: %v", err)
			}
			log.Printf("mongo store init failed, falling back to in-memory store: %v", err)
			st = store.NewMemoryStore()
		} else {
			st = mongoStore
		}
	}
	defer func() {
		if err := st.Close(); err != nil {
			log.Printf("mongo disconnect error: %v", err)
		}
	}()
	ps, err := storage.NewPresignServiceFromEnv()
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}
	srv := httpapi.NewServer(st, tm, ps)

	log.Printf("visto-easy listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv.Router()); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
