package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
	storagepkg "visto-easy/internal/storage"
	"visto-easy/internal/store"
)

type Server struct {
	store   store.DataStore
	tokens  *auth.TokenManager
	presign storagepkg.PresignService
	authRL  *simpleRateLimiter
	loginLT *loginLockTracker
}

func NewServer(st store.DataStore, tm *auth.TokenManager, presign storagepkg.PresignService) *Server {
	limitPerMinute := envInt("AUTH_RATE_LIMIT_RPM", 30)
	if limitPerMinute <= 0 {
		limitPerMinute = 30
	}
	maxAttempts := envInt("AUTH_LOCK_MAX_ATTEMPTS", 5)
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	windowMinutes := envInt("AUTH_LOCK_WINDOW_MINUTES", 15)
	if windowMinutes <= 0 {
		windowMinutes = 15
	}

	return &Server{
		store:   st,
		tokens:  tm,
		presign: presign,
		authRL:  newSimpleRateLimiter(limitPerMinute, time.Minute),
		loginLT: newLoginLockTracker(maxAttempts, time.Duration(windowMinutes)*time.Minute),
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(requestJSON)

	r.Get("/", s.handleRoot)
	r.Get("/api/v1/health", s.handleHealth)
	r.Post("/api/pagamento/webhook", s.handlePagamentoWebhook)

	r.Route("/api/auth", func(r chi.Router) {
		r.With(s.authRateLimitMiddleware()).Post("/register", s.handleRegister)
		r.With(s.authRateLimitMiddleware()).Post("/login", s.handleLogin)
		r.Post("/refresh", s.handleRefresh)
		r.Post("/logout", s.handleLogout)
		r.With(s.authRateLimitMiddleware()).Post("/forgot-password", s.handleForgotPassword)
		r.With(s.authRateLimitMiddleware()).Post("/reset-password", s.handleResetPassword)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware())
		r.Route("/api/pratiche", func(r chi.Router) {
			r.Post("/", s.handleCreatePratica)
			r.Get("/", s.handleListMiePratiche)
			r.Get("/{id}", s.handleGetPratica)
			r.Get("/{id}/eventi", s.handleListEventiPratica)
			r.Patch("/{id}", s.handlePatchPratica)
			r.Delete("/{id}", s.handleDeletePratica)
			r.Post("/{id}/submit", s.handleSubmitPratica)
			r.Post("/{id}/documenti/presign", s.handlePresignDocumento)
			r.Post("/{id}/documenti", s.handleAddDocumento)
			r.Get("/{id}/documenti", s.handleListDocumenti)
		})

		r.Route("/api/pagamento", func(r chi.Router) {
			r.Get("/{token}", s.handleGetPagamento)
			r.Get("/{token}/stato", s.handleGetPagamento)
			r.Post("/crea-sessione", s.requireRoles(model.RoleOperatore, model.RoleSupervisore, model.RoleAdmin)(http.HandlerFunc(s.handleCreatePagamentoSessione)).ServeHTTP)
		})

		r.Route("/api/bo", func(r chi.Router) {
			r.Use(s.requireRoles(model.RoleOperatore, model.RoleSupervisore, model.RoleAdmin))
			r.Get("/utenti", s.handleBOListUtenti)
			r.Get("/security-events", s.handleBOSecurityEvents)
			r.Get("/security-events/stats", s.handleBOSecurityEventsStats)
			r.Get("/security-events/stream", s.handleBOSecurityAlertsStream)
			r.Get("/security-events/report.csv", s.handleBOSecurityEventsCSV)
			r.Get("/security-events/{id}", s.handleBOGetSecurityEvent)
			r.Get("/pratiche", s.handleBOListPratiche)
			r.Get("/report.csv", s.handleBOReportCSV)
			r.Get("/notifications/stream", s.handleBONotificationsStream)
			r.Get("/pratiche/{id}", s.handleGetPratica)
			r.Get("/pratiche/{id}/eventi", s.handleListEventiPratica)
			r.Patch("/pratiche/{id}/stato", s.handleBOChangeStato)
			r.Post("/pratiche/{id}/note", s.handleBOAddNota)
			r.Post("/pratiche/{id}/assegna", s.handleBOAssegna)
			r.Post("/pratiche/{id}/link-pagamento", s.handleBOCreateLinkPagamento)
			r.Post("/pratiche/{id}/richiedi-doc", s.handleBORichiediDoc)
			r.Post("/pratiche/{id}/invia-visto", s.handleBOInviaVisto)
			r.Get("/dashboard/stats", s.handleBOStats)
		})
	})

	return r
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>Visto Easy</title><style>body{font-family:ui-sans-serif,system-ui,-apple-system,Segoe UI,Roboto,sans-serif;margin:0;background:linear-gradient(120deg,#f6f7fb,#eef4ff);color:#112;padding:48px}main{max-width:880px;margin:0 auto;background:#fff;border-radius:20px;padding:32px;box-shadow:0 10px 30px rgba(17,34,68,.08)}h1{margin:0 0 8px;font-size:40px}p{line-height:1.5;color:#445}a{display:inline-block;margin-right:12px;margin-top:16px;padding:10px 14px;border-radius:10px;text-decoration:none;border:1px solid #ccd}a.primary{background:#0f4fff;color:#fff;border-color:#0f4fff}</style></head><body><main><h1>Visto Easy</h1><p>Portale di gestione richieste visto. API operative su /api/*.</p><a class=\"primary\" href=\"/api/v1/health\">Health</a><a href=\"/api/pratiche\">API Pratiche</a></main></body></html>"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"service": "visto-easy", "status": "running"})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Nome     string `json:"nome"`
		Cognome  string `json:"cognome"`
	}
	if !decodeJSON(w, r, &req) { return }
	if req.Email == "" || len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "email/password non validi")
		return
	}
	pwd, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	u, err := s.store.CreateUser(model.Utente{Email: req.Email, PasswordHash: string(pwd), Nome: req.Nome, Cognome: req.Cognome, Ruolo: model.RoleRichiedente})
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) { writeErr(w, http.StatusConflict, "utente già esistente"); return }
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct { Email, Password string }
	if !decodeJSON(w, r, &req) { return }
	email := strings.ToLower(strings.TrimSpace(req.Email))
	ip := clientIP(r)
	ua := strings.TrimSpace(r.UserAgent())
	if email == "" {
		s.recordSecurityEvent(model.SecurityEvent{
			Type:      "LOGIN_FAILED",
			Outcome:   "invalid_email",
			IP:        ip,
			UserAgent: ua,
		})
		writeErr(w, http.StatusUnauthorized, "credenziali non valide")
		return
	}
	if locked, remaining := s.loginLT.IsLocked(email); locked {
		s.recordSecurityEvent(model.SecurityEvent{
			Type:      "LOGIN_LOCKED",
			Outcome:   "blocked",
			Email:     email,
			IP:        ip,
			UserAgent: ua,
			Metadata: map[string]any{
				"retry_after_seconds": int(remaining.Seconds()),
			},
		})
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":               "troppi tentativi di login, riprova più tardi",
			"retry_after_seconds": int(remaining.Seconds()),
		})
		return
	}

	u, err := s.store.GetUserByEmail(email)
	if err != nil || u.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		if locked, remaining := s.loginLT.RecordFailure(email); locked {
			s.recordSecurityEvent(model.SecurityEvent{
				Type:      "LOGIN_LOCKED",
				Outcome:   "threshold_reached",
				Email:     email,
				IP:        ip,
				UserAgent: ua,
				Metadata: map[string]any{
					"retry_after_seconds": int(remaining.Seconds()),
				},
			})
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":               "troppi tentativi di login, riprova più tardi",
				"retry_after_seconds": int(remaining.Seconds()),
			})
			return
		}
		s.recordSecurityEvent(model.SecurityEvent{
			Type:      "LOGIN_FAILED",
			Outcome:   "invalid_credentials",
			Email:     email,
			IP:        ip,
			UserAgent: ua,
		})
		writeErr(w, http.StatusUnauthorized, "credenziali non valide")
		return
	}
	s.loginLT.Clear(email)
	s.recordSecurityEvent(model.SecurityEvent{
		Type:      "LOGIN_SUCCESS",
		Outcome:   "ok",
		Email:     email,
		UserID:    u.ID,
		IP:        ip,
		UserAgent: ua,
	})
	access, _ := s.tokens.SignAccess(u.ID, string(u.Ruolo))
	refresh, _ := s.tokens.SignRefresh(u.ID, string(u.Ruolo))
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access, "refresh_token": refresh, "user": u})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct { RefreshToken string `json:"refresh_token"` }
	if !decodeJSON(w, r, &req) { return }
	c, err := s.tokens.Parse(req.RefreshToken)
	if err != nil || c.Type != "refresh" {
		writeErr(w, http.StatusUnauthorized, "refresh token non valido")
		return
	}
	access, _ := s.tokens.SignAccess(c.UserID, c.Role)
	refresh, _ := s.tokens.SignRefresh(c.UserID, c.Role)
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access, "refresh_token": refresh})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "logged_out"})
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct { Email string `json:"email"` }
	if !decodeJSON(w, r, &req) { return }
	writeJSON(w, http.StatusOK, map[string]any{"status": "accepted", "message": "se l'email esiste riceverai istruzioni"})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) { return }
	if len(strings.TrimSpace(req.NewPassword)) < 8 {
		writeErr(w, http.StatusBadRequest, "password troppo corta")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset_completed"})
}

func (s *Server) handleCreatePratica(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleRichiedente) {
		writeErr(w, http.StatusForbidden, "solo richiedente")
		return
	}
	var req struct {
		TipoVisto      string         `json:"tipo_visto"`
		PaeseDest      string         `json:"paese_dest"`
		DatiAnagrafici map[string]any `json:"dati_anagrafici"`
		DatiPassaporto map[string]any `json:"dati_passaporto"`
	}
	if !decodeJSON(w, r, &req) { return }
	p, err := s.store.CreatePratica(model.Pratica{UtenteID: claims.UserID, TipoVisto: req.TipoVisto, PaeseDest: req.PaeseDest, DatiAnagrafici: req.DatiAnagrafici, DatiPassaporto: req.DatiPassaporto}, claims.UserID)
	if err != nil { writeErr(w, http.StatusInternalServerError, "errore creazione pratica"); return }
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleListMiePratiche(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil { writeErr(w, http.StatusUnauthorized, "non autenticato"); return }
	if claims.Role == string(model.RoleRichiedente) {
		writeJSON(w, http.StatusOK, s.store.ListPraticheByUser(claims.UserID)); return
	}
	writeJSON(w, http.StatusOK, s.store.ListAllPratiche())
}

func (s *Server) handleGetPratica(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := s.store.GetPratica(id)
	if err != nil { writeErr(w, http.StatusNotFound, "pratica non trovata"); return }
	claims := claimsFromCtx(r.Context())
	if claims == nil { writeErr(w, http.StatusUnauthorized, "non autenticato"); return }
	if claims.Role == string(model.RoleRichiedente) && p.UtenteID != claims.UserID {
		writeErr(w, http.StatusForbidden, "accesso negato")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handlePatchPratica(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleRichiedente) { writeErr(w, http.StatusForbidden, "solo richiedente"); return }
	id := chi.URLParam(r, "id")
	var data map[string]any
	if !decodeJSON(w, r, &data) { return }
	p, err := s.store.UpdatePraticaAsDraft(id, claims.UserID, data)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) { writeErr(w, http.StatusNotFound, "pratica non trovata"); return }
		writeErr(w, http.StatusForbidden, "modifica non consentita")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeletePratica(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleRichiedente) { writeErr(w, http.StatusForbidden, "solo richiedente"); return }
	if err := s.store.DeletePraticaAsDraft(chi.URLParam(r, "id"), claims.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) { writeErr(w, http.StatusNotFound, "pratica non trovata"); return }
		writeErr(w, http.StatusForbidden, "eliminazione non consentita")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSubmitPratica(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleRichiedente) {
		writeErr(w, http.StatusForbidden, "solo richiedente")
		return
	}
	p, err := s.store.SubmitPratica(chi.URLParam(r, "id"), claims.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica non trovata")
			return
		}
		writeErr(w, http.StatusForbidden, "submit non consentito")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleAddDocumento(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	id := chi.URLParam(r, "id")
	p, err := s.store.GetPratica(id)
	if err != nil { writeErr(w, http.StatusNotFound, "pratica non trovata"); return }
	if claims == nil || (claims.Role == string(model.RoleRichiedente) && p.UtenteID != claims.UserID) {
		writeErr(w, http.StatusForbidden, "accesso negato")
		return
	}
	var req struct {
		Tipo       string `json:"tipo"`
		NomeFile   string `json:"nome_file"`
		MimeType   string `json:"mime_type"`
		Dimensione int64  `json:"dimensione"`
	}
	if !decodeJSON(w, r, &req) { return }
	if req.Dimensione > 10*1024*1024 { writeErr(w, http.StatusBadRequest, "file troppo grande") ; return }
	doc, err := s.store.AddDocumento(id, model.Documento{Tipo: req.Tipo, NomeFile: req.NomeFile, MimeType: strings.ToLower(req.MimeType), Dimensione: req.Dimensione})
	if err != nil { writeErr(w, http.StatusInternalServerError, "errore upload"); return }
	writeJSON(w, http.StatusCreated, map[string]any{"documento": doc, "upload_url": "https://storage.example/upload/" + doc.ID})
}

func (s *Server) handlePresignDocumento(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	id := chi.URLParam(r, "id")
	p, err := s.store.GetPratica(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	if claims == nil || (claims.Role == string(model.RoleRichiedente) && p.UtenteID != claims.UserID) {
		writeErr(w, http.StatusForbidden, "accesso negato")
		return
	}

	var req struct {
		NomeFile   string `json:"nome_file"`
		MimeType   string `json:"mime_type"`
		Dimensione int64  `json:"dimensione"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.NomeFile == "" || !isAllowedMime(req.MimeType) {
		writeErr(w, http.StatusBadRequest, "file non valido")
		return
	}
	if req.Dimensione <= 0 || req.Dimensione > 10*1024*1024 {
		writeErr(w, http.StatusBadRequest, "dimensione file non valida")
		return
	}

	session, err := s.presign.PresignDocumentUpload(id, req.NomeFile, req.MimeType, req.Dimensione)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "storage non configurato")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload": session,
	})
}

func (s *Server) handleListDocumenti(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	docs, err := s.store.ListDocumenti(id)
	if err != nil { writeErr(w, http.StatusNotFound, "pratica non trovata"); return }
	writeJSON(w, http.StatusOK, docs)
}

func (s *Server) handleListEventiPratica(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	id := chi.URLParam(r, "id")
	p, err := s.store.GetPratica(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	if claims == nil {
		writeErr(w, http.StatusUnauthorized, "non autenticato")
		return
	}
	if claims.Role == string(model.RoleRichiedente) && p.UtenteID != claims.UserID {
		writeErr(w, http.StatusForbidden, "accesso negato")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": p.Eventi, "count": len(p.Eventi)})
}

func (s *Server) handleCreatePagamentoSessione(w http.ResponseWriter, r *http.Request) {
	var req struct { PraticaID, Provider string; Importo float64 }
	if !decodeJSON(w, r, &req) { return }
	if req.Provider == "" { req.Provider = "stripe" }
	pay, err := s.store.CreatePayment(req.PraticaID, req.Provider, req.Importo)
	if err != nil { writeErr(w, http.StatusNotFound, "pratica non trovata"); return }
	_, _ = s.store.ChangePraticaState(req.PraticaID, claimsFromCtx(r.Context()).UserID, model.StatoAttendePagamento, "link pagamento generato")
	writeJSON(w, http.StatusCreated, pay)
}

func (s *Server) handleGetPagamento(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	pay, err := s.store.GetPaymentByToken(token)
	if err != nil { writeErr(w, http.StatusNotFound, "pagamento non trovato"); return }
	writeJSON(w, http.StatusOK, pay)
}

func (s *Server) handlePagamentoWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "payload non valido")
		return
	}
	secret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	if secret != "" {
		sig := strings.TrimSpace(r.Header.Get("Stripe-Signature"))
		if !verifyStripeSignature(secret, body, sig) {
			writeErr(w, http.StatusUnauthorized, "firma webhook non valida")
			return
		}
	}

	info, err := parseWebhookPaymentInfo(body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "payload non valido")
		return
	}
	if strings.ToLower(info.Event) != "payment.succeeded" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ignored"})
		return
	}
	if strings.TrimSpace(info.Token) == "" {
		writeErr(w, http.StatusBadRequest, "token pagamento mancante")
		return
	}

	existingPay, err := s.store.GetPaymentByToken(info.Token)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pagamento non trovato")
		return
	}

	alreadyProcessed, err := s.store.MarkWebhookEventProcessed("stripe", info.EventID, existingPay.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore deduplicazione webhook")
		return
	}
	if alreadyProcessed {
		writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate_ignored", "event_id": info.EventID})
		return
	}

	pay, err := s.store.ConfirmPaymentByToken(info.Token)
	if err != nil { writeErr(w, http.StatusNotFound, "pagamento non trovato"); return }
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoPagamentoRicevuto, "webhook provider")
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoVistoInElaborazione, "generazione visto")
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoVistoEmesso, "visto emesso")
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoCompletata, "visto consegnato")
	writeJSON(w, http.StatusOK, map[string]any{"status": "processed", "event_id": info.EventID})
}

func (s *Server) handleBOListPratiche(w http.ResponseWriter, r *http.Request) {
	filtered := s.filterPraticheBackoffice(r)
	page, pageSize := parsePagination(r)
	sortBy := strings.TrimSpace(r.URL.Query().Get("sort_by"))
	sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order")))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	s.sortPratiche(filtered, sortBy, sortOrder)

	usersByID := make(map[string]model.Utente)
	for _, u := range s.store.ListUsers() {
		usersByID[u.ID] = u
	}

	total := len(filtered)
	start, end := paginateBounds(total, page, pageSize)
	pagedPratiche := filtered[start:end]
	paged := make([]map[string]any, 0, len(pagedPratiche))
	for _, p := range pagedPratiche {
		richiedente := usersByID[p.UtenteID]
		paged = append(paged, map[string]any{
			"pratica": p,
			"richiedente": map[string]any{
				"id":      richiedente.ID,
				"email":   richiedente.Email,
				"nome":    richiedente.Nome,
				"cognome": richiedente.Cognome,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":     paged,
		"count":     len(paged),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (s *Server) filterPraticheBackoffice(r *http.Request) []model.Pratica {
	all := s.store.ListAllPratiche()
	filtered := make([]model.Pratica, 0, len(all))
	stato := strings.TrimSpace(r.URL.Query().Get("stato"))
	tipoVisto := strings.TrimSpace(r.URL.Query().Get("tipo_visto"))
	paeseDest := strings.TrimSpace(r.URL.Query().Get("paese_dest"))
	priorita := strings.TrimSpace(r.URL.Query().Get("priorita"))
	operatoreID := strings.TrimSpace(r.URL.Query().Get("operatore_id"))
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	fromTs := parseOptionalTime(r.URL.Query().Get("from"))
	toTs := parseOptionalTime(r.URL.Query().Get("to"))
	usersByID := make(map[string]model.Utente)
	for _, u := range s.store.ListUsers() {
		usersByID[u.ID] = u
	}

	for _, p := range all {
		if stato != "" && string(p.Stato) != stato {
			continue
		}
		if tipoVisto != "" && !strings.EqualFold(p.TipoVisto, tipoVisto) {
			continue
		}
		if paeseDest != "" && !strings.EqualFold(p.PaeseDest, paeseDest) {
			continue
		}
		if priorita != "" && string(p.Priorita) != priorita {
			continue
		}
		if operatoreID != "" && p.OperatoreID != operatoreID {
			continue
		}
		if !fromTs.IsZero() && p.CreatoIl.Before(fromTs) {
			continue
		}
		if !toTs.IsZero() && p.CreatoIl.After(toTs) {
			continue
		}
		richiedente := usersByID[p.UtenteID]
		if q != "" {
			haystack := strings.ToLower(strings.Join([]string{
				p.Codice, p.TipoVisto, p.PaeseDest, p.UtenteID,
				richiedente.Email, richiedente.Nome, richiedente.Cognome,
			}, "|"))
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		filtered = append(filtered, p)
	}

	return filtered
}

func (s *Server) sortPratiche(pratiche []model.Pratica, sortBy, sortOrder string) {
	sort.Slice(pratiche, func(i, j int) bool {
		pi := pratiche[i]
		pj := pratiche[j]
		less := false
		switch sortBy {
		case "codice":
			less = pi.Codice < pj.Codice
		case "stato":
			less = string(pi.Stato) < string(pj.Stato)
		case "priorita":
			less = string(pi.Priorita) < string(pj.Priorita)
		case "paese_dest":
			less = strings.ToLower(pi.PaeseDest) < strings.ToLower(pj.PaeseDest)
		default:
			less = pi.CreatoIl.Before(pj.CreatoIl)
		}
		if sortOrder == "asc" {
			return less
		}
		return !less
	})
}

func (s *Server) handleBOListUtenti(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	sortBy := strings.TrimSpace(r.URL.Query().Get("sort_by"))
	sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order")))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	rolo := strings.TrimSpace(r.URL.Query().Get("ruolo"))
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	items := make([]model.Utente, 0)
	for _, u := range s.store.ListUsers() {
		if rolo != "" && string(u.Ruolo) != rolo {
			continue
		}
		if q != "" {
			h := strings.ToLower(strings.Join([]string{u.Email, u.Nome, u.Cognome, string(u.Ruolo)}, "|"))
			if !strings.Contains(h, q) {
				continue
			}
		}
		items = append(items, u)
	}

	sort.Slice(items, func(i, j int) bool {
		less := false
		switch sortBy {
		case "email":
			less = strings.ToLower(items[i].Email) < strings.ToLower(items[j].Email)
		case "nome":
			less = strings.ToLower(items[i].Nome) < strings.ToLower(items[j].Nome)
		case "ruolo":
			less = string(items[i].Ruolo) < string(items[j].Ruolo)
		default:
			less = items[i].CreatoIl.Before(items[j].CreatoIl)
		}
		if sortOrder == "asc" {
			return less
		}
		return !less
	})

	total := len(items)
	start, end := paginateBounds(total, page, pageSize)
	paged := items[start:end]

	writeJSON(w, http.StatusOK, map[string]any{
		"items":     paged,
		"count":     len(paged),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (s *Server) handleBOSecurityEvents(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	filtered := s.filterSecurityEvents(r)
	total := len(filtered)
	start, end := paginateBounds(total, page, pageSize)
	paged := filtered[start:end]

	writeJSON(w, http.StatusOK, map[string]any{
		"items":     paged,
		"count":     len(paged),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (s *Server) handleBOSecurityEventsStats(w http.ResponseWriter, r *http.Request) {
	items := s.filterSecurityEvents(r)
	windowMinutes := envInt("SECURITY_ALERT_WINDOW_MINUTES", 15)
	if windowMinutes <= 0 {
		windowMinutes = 15
	}
	windowStart := time.Now().UTC().Add(-time.Duration(windowMinutes) * time.Minute)

	byType := map[string]int{}
	byOutcome := map[string]int{}
	recentFailedByIP := map[string]int{}
	recentFailedByEmail := map[string]int{}
	recentFailed := 0
	recentLocked := 0

	for _, evt := range items {
		byType[evt.Type]++
		byOutcome[evt.Outcome]++
		if evt.CreatoIl.Before(windowStart) {
			continue
		}
		if evt.Type == "LOGIN_FAILED" {
			recentFailed++
			if strings.TrimSpace(evt.IP) != "" {
				recentFailedByIP[evt.IP]++
			}
			if strings.TrimSpace(evt.Email) != "" {
				recentFailedByEmail[evt.Email]++
			}
		}
		if evt.Type == "LOGIN_LOCKED" {
			recentLocked++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":                 len(items),
		"window_minutes":        windowMinutes,
		"recent_failed_logins":  recentFailed,
		"recent_locked_logins":  recentLocked,
		"by_type":               byType,
		"by_outcome":            byOutcome,
		"top_failed_ips":        topMapEntries(recentFailedByIP, 5),
		"top_failed_emails":     topMapEntries(recentFailedByEmail, 5),
		"high_risk_detected":    recentFailed >= envInt("SECURITY_ALERT_FAILED_THRESHOLD", 5) || recentLocked > 0,
	})
}

func (s *Server) handleBOGetSecurityEvent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id evento non valido")
		return
	}
	evt, err := s.store.GetSecurityEventByID(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "evento sicurezza non trovato")
		return
	}
	writeJSON(w, http.StatusOK, evt)
}

func (s *Server) handleBOSecurityEventsCSV(w http.ResponseWriter, r *http.Request) {
	items := s.filterSecurityEvents(r)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=security_events.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "type", "outcome", "email", "user_id", "ip", "user_agent", "creato_il"})
	for _, evt := range items {
		_ = cw.Write([]string{
			evt.ID,
			evt.Type,
			evt.Outcome,
			evt.Email,
			evt.UserID,
			evt.IP,
			evt.UserAgent,
			evt.CreatoIl.Format(time.RFC3339),
		})
	}
	cw.Flush()
}

func (s *Server) handleBOSecurityAlertsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "stream non supportato")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	lastSignature := ""
	sendSnapshot := func() {
		snapshot := s.buildSecurityAlertSnapshot()
		signature := fmt.Sprintf("%s|%d|%d", snapshot["severity"], snapshot["recent_failed_logins"], snapshot["recent_locked_logins"])
		if signature == lastSignature {
			return
		}
		lastSignature = signature
		writeSSEEvent(w, "security_alert", snapshot)
		flusher.Flush()
	}

	writeSSEEvent(w, "ready", map[string]any{"status": "connected"})
	sendSnapshot()
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sendSnapshot()
		}
	}
}

func (s *Server) buildSecurityAlertSnapshot() map[string]any {
	windowMinutes := envInt("SECURITY_ALERT_WINDOW_MINUTES", 15)
	if windowMinutes <= 0 {
		windowMinutes = 15
	}
	failedThreshold := envInt("SECURITY_ALERT_FAILED_THRESHOLD", 5)
	if failedThreshold <= 0 {
		failedThreshold = 5
	}
	windowStart := time.Now().UTC().Add(-time.Duration(windowMinutes) * time.Minute)

	recentFailed := 0
	recentLocked := 0
	for _, evt := range s.store.ListSecurityEvents() {
		if evt.CreatoIl.Before(windowStart) {
			continue
		}
		if evt.Type == "LOGIN_FAILED" {
			recentFailed++
		}
		if evt.Type == "LOGIN_LOCKED" {
			recentLocked++
		}
	}

	severity := "ok"
	if recentLocked > 0 {
		severity = "critical"
	} else if recentFailed >= failedThreshold {
		severity = "warning"
	}

	return map[string]any{
		"severity":             severity,
		"window_minutes":       windowMinutes,
		"failed_threshold":     failedThreshold,
		"recent_failed_logins": recentFailed,
		"recent_locked_logins": recentLocked,
		"generated_at":         time.Now().UTC().Format(time.RFC3339),
	}
}

func (s *Server) filterSecurityEvents(r *http.Request) []model.SecurityEvent {
	eventType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	outcome := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("outcome")))
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	fromTs := parseOptionalTime(r.URL.Query().Get("from"))
	toTs := parseOptionalTime(r.URL.Query().Get("to"))

	all := s.store.ListSecurityEvents()
	filtered := make([]model.SecurityEvent, 0, len(all))
	for _, evt := range all {
		if eventType != "" && strings.ToLower(evt.Type) != eventType {
			continue
		}
		if outcome != "" && strings.ToLower(evt.Outcome) != outcome {
			continue
		}
		if !fromTs.IsZero() && evt.CreatoIl.Before(fromTs) {
			continue
		}
		if !toTs.IsZero() && evt.CreatoIl.After(toTs) {
			continue
		}
		if q != "" {
			h := strings.ToLower(strings.Join([]string{evt.Email, evt.UserID, evt.IP, evt.UserAgent, evt.Type, evt.Outcome}, "|"))
			if !strings.Contains(h, q) {
				continue
			}
		}
		filtered = append(filtered, evt)
	}

	sort.Slice(filtered, func(i, j int) bool { return filtered[i].CreatoIl.After(filtered[j].CreatoIl) })
	return filtered
}

func topMapEntries(items map[string]int, limit int) []map[string]any {
	type pair struct {
		Key   string
		Count int
	}
	pairs := make([]pair, 0, len(items))
	for k, v := range items {
		pairs = append(pairs, pair{Key: k, Count: v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Count == pairs[j].Count {
			return pairs[i].Key < pairs[j].Key
		}
		return pairs[i].Count > pairs[j].Count
	})
	if limit > 0 && len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, map[string]any{"key": p.Key, "count": p.Count})
	}
	return out
}

func (s *Server) handleBOReportCSV(w http.ResponseWriter, r *http.Request) {
	all := s.filterPraticheBackoffice(r)
	sortBy := strings.TrimSpace(r.URL.Query().Get("sort_by"))
	sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order")))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	s.sortPratiche(all, sortBy, sortOrder)

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=report_pratiche.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "codice", "stato", "priorita", "tipo_visto", "paese_dest", "utente_id", "operatore_id", "creato_il", "aggiornato_il"})
	for _, p := range all {
		_ = cw.Write([]string{
			p.ID,
			p.Codice,
			string(p.Stato),
			string(p.Priorita),
			p.TipoVisto,
			p.PaeseDest,
			p.UtenteID,
			p.OperatoreID,
			p.CreatoIl.Format(time.RFC3339),
			p.AggiornatoIl.Format(time.RFC3339),
		})
	}
	cw.Flush()
}

func (s *Server) handleBONotificationsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "stream non supportato")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	lastSignature := ""
	sendSnapshot := func() {
		snapshot := s.buildBONotificationSnapshot()
		signature := fmt.Sprintf("%d|%d|%s", snapshot["totale_pratiche"], snapshot["in_coda"], snapshot["ultimo_aggiornamento"])
		if signature == lastSignature {
			return
		}
		lastSignature = signature
		writeSSEEvent(w, "bo_snapshot", snapshot)
		flusher.Flush()
	}

	writeSSEEvent(w, "ready", map[string]any{"status": "connected"})
	sendSnapshot()
	flusher.Flush()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sendSnapshot()
		}
	}
}

func (s *Server) buildBONotificationSnapshot() map[string]any {
	all := s.store.ListAllPratiche()
	inCoda := 0
	ultimo := time.Time{}
	for _, p := range all {
		if p.Stato != model.StatoCompletata {
			inCoda++
		}
		if p.AggiornatoIl.After(ultimo) {
			ultimo = p.AggiornatoIl
		}
	}
	last := ""
	if !ultimo.IsZero() {
		last = ultimo.UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"totale_pratiche":      len(all),
		"in_coda":              inCoda,
		"ultimo_aggiornamento": last,
		"emesso_oggi":          s.countPraticheInStateSince(all, model.StatoVistoEmesso, time.Now().Add(-24*time.Hour)),
	}
}

func (s *Server) countPraticheInStateSince(items []model.Pratica, stato model.StatoPratica, since time.Time) int {
	count := 0
	for _, p := range items {
		if p.Stato == stato && p.AggiornatoIl.After(since) {
			count++
		}
	}
	return count
}

func writeSSEEvent(w http.ResponseWriter, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func (s *Server) handleBOChangeStato(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct { Stato model.StatoPratica `json:"stato"`; Nota string `json:"nota"` }
	if !decodeJSON(w, r, &req) { return }
	p, err := s.store.ChangePraticaState(chi.URLParam(r, "id"), claims.UserID, req.Stato, req.Nota)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) { writeErr(w, http.StatusNotFound, "pratica non trovata"); return }
		writeErr(w, http.StatusBadRequest, "transizione non valida")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleBOAssegna(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct { OperatoreID string `json:"operatore_id"` }
	if !decodeJSON(w, r, &req) { return }
	p, err := s.store.AssignOperatore(chi.URLParam(r, "id"), strings.TrimSpace(req.OperatoreID), claims.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica o operatore non trovato")
			return
		}
		writeErr(w, http.StatusBadRequest, "assegnazione non valida")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleBOAddNota(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		Messaggio string `json:"messaggio"`
		Interna   bool   `json:"interna"`
	}
	if !decodeJSON(w, r, &req) { return }
	p, err := s.store.AddNota(chi.URLParam(r, "id"), claims.UserID, req.Messaggio, req.Interna)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica non trovata")
			return
		}
		writeErr(w, http.StatusBadRequest, "nota non valida")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleBORichiediDoc(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		Documento string `json:"documento"`
		Nota      string `json:"nota"`
	}
	if !decodeJSON(w, r, &req) { return }
	p, err := s.store.RequestDocumento(chi.URLParam(r, "id"), claims.UserID, req.Documento, req.Nota)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica non trovata")
			return
		}
		writeErr(w, http.StatusBadRequest, "richiesta documento non valida")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleBOCreateLinkPagamento(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct { Importo float64 `json:"importo"`; Provider string `json:"provider"` }
	if !decodeJSON(w, r, &req) { return }
	if req.Provider == "" { req.Provider = "stripe" }
	pay, err := s.store.CreatePayment(chi.URLParam(r, "id"), req.Provider, req.Importo)
	if err != nil { writeErr(w, http.StatusNotFound, "pratica non trovata"); return }
	_, _ = s.store.ChangePraticaState(chi.URLParam(r, "id"), claims.UserID, model.StatoAttendePagamento, "link pagamento generato")
	writeJSON(w, http.StatusCreated, pay)
}

func (s *Server) handleBOInviaVisto(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	id := chi.URLParam(r, "id")
	_, err := s.store.ChangePraticaState(id, claims.UserID, model.StatoVistoEmesso, "invio manuale visto")
	if err != nil {
		if errors.Is(err, store.ErrInvalidState) {
			writeErr(w, http.StatusBadRequest, "stato non compatibile per invio visto")
			return
		}
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	_, _ = s.store.ChangePraticaState(id, claims.UserID, model.StatoCompletata, "pratica completata")
	writeJSON(w, http.StatusOK, map[string]any{"status": "visto inviato"})
}

func (s *Server) handleBOStats(w http.ResponseWriter, _ *http.Request) {
	all := s.store.ListAllPratiche()
	stats := map[model.StatoPratica]int{}
	for _, p := range all { stats[p.Stato]++ }
	writeJSON(w, http.StatusOK, map[string]any{"totale_pratiche": len(all), "by_stato": stats})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, "payload non valido")
		return false
	}
	return true
}

func requestJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authRateLimitMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)
			if key == "" {
				key = "unknown"
			}
			allowed, retryAfter := s.authRL.Allow(key)
			if !allowed {
				s.recordSecurityEvent(model.SecurityEvent{
					Type:      "AUTH_RATE_LIMIT_HIT",
					Outcome:   "blocked",
					IP:        key,
					UserAgent: strings.TrimSpace(r.UserAgent()),
					Metadata: map[string]any{
						"path":                r.URL.Path,
						"retry_after_seconds": int(retryAfter.Seconds()),
					},
				})
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error":               "troppi tentativi, riprova più tardi",
					"retry_after_seconds": int(retryAfter.Seconds()),
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	xForwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xForwardedFor != "" {
		parts := strings.Split(xForwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	xRealIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xRealIP != "" {
		return xRealIP
	}
	remote := strings.TrimSpace(r.RemoteAddr)
	host, _, err := net.SplitHostPort(remote)
	if err == nil && strings.TrimSpace(host) != "" {
		return strings.TrimSpace(host)
	}
	return remote
}

func (s *Server) recordSecurityEvent(evt model.SecurityEvent) {
	_, _ = s.store.AddSecurityEvent(evt)
}

func parseOptionalTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC()
	}
	return time.Time{}
}

type webhookPaymentInfo struct {
	Event   string
	EventID string
	Token   string
}

func parseWebhookPaymentInfo(body []byte) (webhookPaymentInfo, error) {
	var raw struct {
		Event   string `json:"event"`
		EventID string `json:"event_id"`
		Token   string `json:"token"`
		ID      string `json:"id"`
		Type    string `json:"type"`
		Data    struct {
			Object struct {
				ClientReferenceID string            `json:"client_reference_id"`
				Metadata          map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return webhookPaymentInfo{}, err
	}

	event := strings.TrimSpace(raw.Event)
	if event == "" {
		event = strings.TrimSpace(raw.Type)
	}
	eventID := strings.TrimSpace(raw.EventID)
	if eventID == "" {
		eventID = strings.TrimSpace(raw.ID)
	}
	token := strings.TrimSpace(raw.Token)
	if token == "" && raw.Data.Object.Metadata != nil {
		token = strings.TrimSpace(raw.Data.Object.Metadata["token"])
	}
	if token == "" {
		token = strings.TrimSpace(raw.Data.Object.ClientReferenceID)
	}
	if eventID == "" {
		eventID = webhookPayloadFingerprint(body)
	}

	return webhookPaymentInfo{Event: strings.ToLower(event), EventID: eventID, Token: token}, nil
}

func webhookPayloadFingerprint(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:12])
}

func verifyStripeSignature(secret string, payload []byte, signatureHeader string) bool {
	if strings.TrimSpace(signatureHeader) == "" {
		return false
	}
	parts := strings.Split(signatureHeader, ",")
	v1 := ""
	ts := ""
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "t" {
			ts = kv[1]
		}
		if kv[0] == "v1" {
			v1 = kv[1]
		}
	}
	if v1 == "" || ts == "" {
		return false
	}
	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	if delta := time.Since(time.Unix(timestamp, 0)); delta > 5*time.Minute || delta < -5*time.Minute {
		return false
	}
	signedPayload := []byte(ts + "." + string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(signedPayload)
	expected := mac.Sum(nil)
	provided, err := hex.DecodeString(strings.TrimSpace(v1))
	if err != nil {
		return false
	}
	return hmac.Equal(expected, provided)
}

func isAllowedMime(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	allowed := map[string]bool{
		"application/pdf": true,
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
		"image/heic":      true,
	}
	return allowed[v]
}

func parsePagination(r *http.Request) (int, int) {
	page := 1
	pageSize := 25
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("page_size")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			if v > 200 {
				v = 200
			}
			pageSize = v
		}
	}
	return page, pageSize
}

func paginateBounds(total, page, pageSize int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	start := (page - 1) * pageSize
	if start >= total {
		return total, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return start, end
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

type simpleRateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	bucketByK map[string]rateBucket
}

type rateBucket struct {
	count   int
	resetAt time.Time
}

func newSimpleRateLimiter(limit int, window time.Duration) *simpleRateLimiter {
	return &simpleRateLimiter{
		limit:     limit,
		window:    window,
		bucketByK: map[string]rateBucket{},
	}
}

func (l *simpleRateLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b := l.bucketByK[key]
	if b.resetAt.IsZero() || now.After(b.resetAt) {
		b = rateBucket{count: 0, resetAt: now.Add(l.window)}
	}
	if b.count >= l.limit {
		retry := time.Until(b.resetAt)
		if retry < 0 {
			retry = 0
		}
		l.bucketByK[key] = b
		return false, retry
	}
	b.count++
	l.bucketByK[key] = b
	return true, 0
}

type loginLockTracker struct {
	mu          sync.Mutex
	maxAttempts int
	window      time.Duration
	entries     map[string]loginAttempt
}

type loginAttempt struct {
	count       int
	windowStart time.Time
	lockedUntil time.Time
}

func newLoginLockTracker(maxAttempts int, window time.Duration) *loginLockTracker {
	return &loginLockTracker{
		maxAttempts: maxAttempts,
		window:      window,
		entries:     map[string]loginAttempt{},
	}
}

func (t *loginLockTracker) IsLocked(email string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.entries[email]
	if !ok {
		return false, 0
	}
	now := time.Now()
	if now.Before(entry.lockedUntil) {
		return true, time.Until(entry.lockedUntil)
	}
	if !entry.lockedUntil.IsZero() && now.After(entry.lockedUntil) {
		delete(t.entries, email)
	}
	return false, 0
}

func (t *loginLockTracker) RecordFailure(email string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	entry := t.entries[email]
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) > t.window {
		entry = loginAttempt{count: 0, windowStart: now}
	}
	if now.Before(entry.lockedUntil) {
		return true, time.Until(entry.lockedUntil)
	}
	entry.count++
	if entry.count >= t.maxAttempts {
		entry.lockedUntil = now.Add(t.window)
	}
	t.entries[email] = entry
	if now.Before(entry.lockedUntil) {
		return true, time.Until(entry.lockedUntil)
	}
	return false, 0
}

func (t *loginLockTracker) Clear(email string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, email)
}
