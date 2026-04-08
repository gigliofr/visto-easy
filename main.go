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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = os.Getenv("SESSION_SECRET")
	}
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET (or SESSION_SECRET) is required and must be >= 32 chars")
	}
	mongoURI := firstNonEmpty(os.Getenv("MONGODB_URI"), os.Getenv("DATABASE_URL"), os.Getenv("MONGO_URL"), os.Getenv("MONGODB_URL"))
	if strings.TrimSpace(mongoURI) == "" {
		log.Fatal("MONGODB_URI is required (aliases: DATABASE_URL, MONGO_URL, MONGODB_URL)")
	}
	mongoDBName := strings.TrimSpace(firstNonEmpty(os.Getenv("MONGODB_DB_NAME"), os.Getenv("DATABASE_NAME")))
	if mongoDBName == "" {
		mongoDBName = "visto-easy"
	}

	tm, err := auth.NewTokenManager(jwtSecret)
	if err != nil {
		log.Fatalf("token manager init failed: %v", err)
	}
	st, err := store.NewMongoStore(mongoURI, mongoDBName)
	if err != nil {
		log.Fatalf("mongo store init failed: %v", err)
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
