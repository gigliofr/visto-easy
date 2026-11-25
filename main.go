package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"visto-easy/internal/auth"
	"visto-easy/internal/httpapi"
	"visto-easy/internal/storage"
	"visto-easy/internal/store"

	"github.com/gigliofr/gokit/envx"
	"github.com/gigliofr/gokit/httpx"
	"github.com/gigliofr/gokit/notifyx"
)

type closableStore interface {
	store.DataStore
	Close() error
}

func main() {
	production := strings.EqualFold(envx.String("ENVIRONMENT", ""), "production")
	port := envx.String("PORT", "8080")
	ops := notifyx.OpsFromEnv("visto-easy")

	jwtSecret := envx.First("JWT_SECRET", "SESSION_SECRET")
	if len(jwtSecret) < 32 && !production {
		jwtSecret = "vistoeasy-local-dev-secret-key-2026-verylong"
		log.Printf("JWT secret missing: using local development default secret")
	}
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET (or SESSION_SECRET) is required and must be >= 32 chars")
	}
	mongoURI := envx.First("MONGODB_URI", "DATABASE_URL", "MONGO_URL", "MONGODB_URL")
	if mongoURI == "" && production {
		log.Fatal("MONGODB_URI is required (aliases: DATABASE_URL, MONGO_URL, MONGODB_URL)")
	}
	mongoDBName := envx.First("MONGODB_DB_NAME", "DATABASE_NAME")
	if mongoDBName == "" {
		mongoDBName = "visto-easy"
	}

	tm, err := auth.NewTokenManager(jwtSecret)
	if err != nil {
		log.Fatalf("token manager init failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var st closableStore
	if mongoURI == "" {
		log.Printf("mongo URI missing: using in-memory store for local development")
		st = store.NewMemoryStore()
	} else {
		mongoStore, err := store.NewMongoStore(mongoURI, mongoDBName)
		if err != nil {
			if production {
				ops.Error("mongo-init", "avvio fallito: MongoDB non raggiungibile", err)
				ops.Flush()
				log.Fatalf("mongo store init failed: %v", err)
			}
			log.Printf("mongo store init failed, falling back to in-memory store: %v", err)
			st = store.NewMemoryStore()
		} else {
			st = mongoStore
			ops.Watch(ctx, "MongoDB", time.Minute, mongoStore.Ping)
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
	srv.SetPanicHook(ops.PanicHook())

	log.Printf("visto-easy listening on :%s", port)
	ops.Started(envx.String("APP_VERSION", ""))
	err = httpx.Run(ctx, httpx.NewServer(":"+port, srv.Router()), 10*time.Second)
	if err != nil {
		ops.Error("server", "server HTTP arrestato per errore", err)
		ops.Flush()
		log.Printf("server stopped: %v", err)
		return
	}
	ops.Stopping("arresto richiesto (SIGTERM)")
}
