package main

import (
	"log"
	"net/http"
	"os"

	"visto-easy/internal/auth"
	"visto-easy/internal/httpapi"
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

	tm, err := auth.NewTokenManager(jwtSecret)
	if err != nil {
		log.Fatalf("token manager init failed: %v", err)
	}
	st := store.NewMemoryStore()
	srv := httpapi.NewServer(st, tm)

	log.Printf("visto-easy listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv.Router()); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
