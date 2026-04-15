package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/mail"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
	"visto-easy/internal/notifications"
	storagepkg "visto-easy/internal/storage"
	"visto-easy/internal/store"
)

type Server struct {
	store   store.DataStore
	tokens  *auth.TokenManager
	presign storagepkg.PresignService
	mailer  notifications.EmailSender
	authRL  *simpleRateLimiter
	loginLT *loginLockTracker
}

var (
	passwordHasLower   = regexp.MustCompile(`[a-z]`)
	passwordHasUpper   = regexp.MustCompile(`[A-Z]`)
	passwordHasNumber  = regexp.MustCompile(`\d`)
	passwordHasSpecial = regexp.MustCompile(`[^A-Za-z0-9]`)
)

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

	srv := &Server{
		store:   st,
		tokens:  tm,
		presign: presign,
		mailer:  notifications.NewEmailSenderFromEnv(),
		authRL:  newSimpleRateLimiter(limitPerMinute, time.Minute),
		loginLT: newLoginLockTracker(maxAttempts, time.Duration(windowMinutes)*time.Minute),
	}
	srv.ensureSeedBackofficeUsers()
	srv.ensureSeedDemoPratiche()
	return srv
}

func shouldSeedBackofficeUsers() bool {
	enabled := strings.ToLower(strings.TrimSpace(os.Getenv("BACKOFFICE_SEED_ENABLED")))
	if enabled == "true" || enabled == "1" || enabled == "yes" {
		return true
	}
	if enabled == "false" || enabled == "0" || enabled == "no" {
		return false
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	return env != "production"
}

func shouldSeedDemoPratiche() bool {
	enabled := strings.ToLower(strings.TrimSpace(os.Getenv("BACKOFFICE_FAKE_PRACTICES_ENABLED")))
	if enabled == "true" || enabled == "1" || enabled == "yes" {
		return true
	}
	if enabled == "false" || enabled == "0" || enabled == "no" {
		return false
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	return env != "production"
}

func (s *Server) ensureSeedBackofficeUsers() {
	if !shouldSeedBackofficeUsers() {
		return
	}

	password := strings.TrimSpace(os.Getenv("BACKOFFICE_SEED_PASSWORD"))
	if len(password) < 8 {
		password = "Admin123!Change"
		log.Printf("[auth] BACKOFFICE_SEED_PASSWORD not set (or too short): using default dev password")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Printf("[auth] unable to generate backoffice seed password hash: %v", err)
		return
	}

	now := time.Now().UTC()
	seeds := []model.Utente{
		{Email: "operatore@vistoeasy.local", Nome: "Mario", Cognome: "Operatore", Ruolo: model.RoleOperatore, PasswordHash: string(hash), Attivo: true, EmailVerificata: true, CreatoIl: now, AggiornatoIl: now},
		{Email: "supervisore@vistoeasy.local", Nome: "Luca", Cognome: "Supervisore", Ruolo: model.RoleSupervisore, PasswordHash: string(hash), Attivo: true, EmailVerificata: true, CreatoIl: now, AggiornatoIl: now},
		{Email: "admin@vistoeasy.local", Nome: "Anna", Cognome: "Admin", Ruolo: model.RoleAdmin, PasswordHash: string(hash), Attivo: true, EmailVerificata: true, CreatoIl: now, AggiornatoIl: now},
	}

	for _, u := range seeds {
		existing, err := s.store.GetUserByEmail(u.Email)
		if err == nil {
			if _, upErr := s.store.UpdateUserPassword(existing.ID, string(hash)); upErr != nil {
				log.Printf("[auth] unable to update seed password for %s: %v", u.Email, upErr)
			}
			continue
		}
		if _, err := s.store.CreateUser(u); err != nil && !errors.Is(err, store.ErrAlreadyExists) {
			log.Printf("[auth] unable to seed user %s: %v", u.Email, err)
		}
	}
}

func (s *Server) ensureSeedDemoPratiche() {
	if !shouldSeedDemoPratiche() {
		return
	}

	userPassword := strings.TrimSpace(os.Getenv("BACKOFFICE_FAKE_USER_PASSWORD"))
	if len(userPassword) < 8 {
		userPassword = "User123!Demo"
	}

	userHash, err := bcrypt.GenerateFromPassword([]byte(userPassword), 12)
	if err != nil {
		log.Printf("[seed] unable to hash BACKOFFICE_FAKE_USER_PASSWORD: %v", err)
		return
	}

	now := time.Now().UTC()
	demoUser, err := s.store.GetUserByEmail("demo.richiedente@vistoeasy.local")
	if err != nil {
		demoUser, err = s.store.CreateUser(model.Utente{
			Email:           "demo.richiedente@vistoeasy.local",
			Nome:            "Giulia",
			Cognome:         "Demo",
			Ruolo:           model.RoleRichiedente,
			PasswordHash:    string(userHash),
			Attivo:          true,
			EmailVerificata: true,
			CreatoIl:        now,
			AggiornatoIl:    now,
		})
		if err != nil && !errors.Is(err, store.ErrAlreadyExists) {
			log.Printf("[seed] unable to create demo richiedente: %v", err)
			return
		}
		demoUser, _ = s.store.GetUserByEmail("demo.richiedente@vistoeasy.local")
	}

	if demoUser.ID == "" {
		log.Printf("[seed] demo richiedente not available")
		return
	}

	existing := s.store.ListPraticheByUser(demoUser.ID)
	targetCount := 100
	seedDemoDocumento := func(praticaID string, idx int) {
		if idx%8 != 0 {
			return
		}
		docs, err := s.store.ListDocumenti(praticaID)
		if err != nil {
			log.Printf("[seed] list documenti failed for pratica %s: %v", praticaID, err)
			return
		}
		if len(docs) > 0 {
			return
		}
		if _, err := s.store.AddDocumento(praticaID, model.Documento{
			Tipo:       "PASSAPORTO",
			NomeFile:   fmt.Sprintf("passaporto-demo-%03d.pdf", idx),
			MimeType:   "application/pdf",
			Dimensione: int64(180000 + idx),
		}); err != nil {
			log.Printf("[seed] add documento failed for pratica %s: %v", praticaID, err)
		}
	}

	if len(existing) >= targetCount {
		for i, p := range existing {
			seedDemoDocumento(p.ID, i+1)
		}
		return
	}

	operatorIDs := []string{}
	if op, err := s.store.GetUserByEmail("operatore@vistoeasy.local"); err == nil && op.ID != "" {
		operatorIDs = append(operatorIDs, op.ID)
	}
	if sup, err := s.store.GetUserByEmail("supervisore@vistoeasy.local"); err == nil && sup.ID != "" {
		operatorIDs = append(operatorIDs, sup.ID)
	}
	if adm, err := s.store.GetUserByEmail("admin@vistoeasy.local"); err == nil && adm.ID != "" {
		operatorIDs = append(operatorIDs, adm.ID)
	}
	if len(operatorIDs) == 0 {
		operatorIDs = append(operatorIDs, demoUser.ID)
	}

	types := []string{"TURISMO", "STUDIO", "LAVORO", "BUSINESS", "TRANSITO"}
	countries := []string{"JP", "CA", "US", "AE", "GB", "AU", "SG", "IT"}
	priorities := []model.Priorita{model.PrioritaNormale, model.PrioritaAlta, model.PrioritaUrgente}
	plans := [][]model.StatoPratica{
		{},
		{model.StatoInviata},
		{model.StatoInviata, model.StatoInLavorazione},
		{model.StatoInviata, model.StatoInLavorazione, model.StatoIntegrazioneRichiesta},
		{model.StatoInviata, model.StatoInLavorazione, model.StatoSospesa},
		{model.StatoInviata, model.StatoRifiutata},
		{model.StatoAnnullata},
		{model.StatoInviata, model.StatoInLavorazione, model.StatoApprovata, model.StatoAttendePagamento},
		{model.StatoInviata, model.StatoInLavorazione, model.StatoApprovata, model.StatoAttendePagamento, model.StatoPagamentoRicevuto, model.StatoVistoInElaborazione, model.StatoVistoEmesso, model.StatoCompletata},
		{model.StatoInviata, model.StatoInLavorazione, model.StatoApprovata, model.StatoAttendePagamento, model.StatoPagamentoRicevuto, model.StatoVistoInElaborazione, model.StatoVistoEmesso},
		{model.StatoInviata, model.StatoInLavorazione, model.StatoIntegrazioneRichiesta, model.StatoInLavorazione, model.StatoApprovata},
		{model.StatoInviata, model.StatoInLavorazione, model.StatoApprovata},
	}

	needed := targetCount - len(existing)
	for i := 0; i < needed; i++ {
		idx := len(existing) + i + 1
		plan := plans[idx%len(plans)]
		priority := priorities[idx%len(priorities)]
		if idx%10 == 0 {
			priority = model.PrioritaUrgente
		}
		p, err := s.store.CreatePratica(model.Pratica{
			UtenteID:  demoUser.ID,
			TipoVisto: types[idx%len(types)],
			PaeseDest: countries[(idx*3)%len(countries)],
			Priorita:  priority,
			DatiAnagrafici: map[string]any{
				"nome":    "Giulia",
				"cognome": "Demo",
				"indice":  idx,
				"lotto":   fmt.Sprintf("demo-%03d", idx),
			},
			DatiPassaporto: map[string]any{
				"numero": fmt.Sprintf("YA%07d", 4000000+idx),
			},
		}, demoUser.ID)
		if err != nil {
			log.Printf("[seed] create pratica failed: %v", err)
			continue
		}

		assignedOperator := ""
		if len(operatorIDs) > 0 && idx%4 != 0 && len(plan) > 0 {
			assignedOperator = operatorIDs[idx%len(operatorIDs)]
		}

		if len(plan) > 0 {
			if _, err := s.store.ChangePraticaState(p.ID, demoUser.ID, plan[0], fmt.Sprintf("seed demo step 1/%d", len(plan))); err != nil {
				log.Printf("[seed] state transition failed for pratica %s -> %s: %v", p.ID, plan[0], err)
				continue
			}
			if assignedOperator != "" && idx%5 != 0 {
				if _, err := s.store.AssignOperatore(p.ID, assignedOperator, demoUser.ID); err != nil {
					log.Printf("[seed] assign operator failed for pratica %s: %v", p.ID, err)
				}
			}
			for step, next := range plan[1:] {
				actor := demoUser.ID
				if assignedOperator != "" {
					actor = assignedOperator
				}
				if _, err := s.store.ChangePraticaState(p.ID, actor, next, fmt.Sprintf("seed demo step %d/%d", step+2, len(plan))); err != nil {
					log.Printf("[seed] state transition failed for pratica %s -> %s: %v", p.ID, next, err)
					break
				}
			}
		}

		if idx%6 == 0 {
			if _, err := s.store.AddNota(p.ID, demoUser.ID, "Seed demo: pratica pronta per revisione documentale", false); err != nil {
				log.Printf("[seed] add note failed for pratica %s: %v", p.ID, err)
			}
		}
		if idx%9 == 0 {
			if _, err := s.store.AddNota(p.ID, demoUser.ID, "Seed demo: verifica allegati e coerenza anagrafica", true); err != nil {
				log.Printf("[seed] add internal note failed for pratica %s: %v", p.ID, err)
			}
		}
		if idx%12 == 0 && assignedOperator != "" {
			if _, err := s.store.AssignOperatore(p.ID, assignedOperator, demoUser.ID); err != nil {
				log.Printf("[seed] reassign operator failed for pratica %s: %v", p.ID, err)
			}
		}


		seedDemoDocumento(p.ID, idx)
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(securityHeadersMiddleware)
	r.Use(corsMiddleware)
	r.Use(requestJSON)
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(resolveWebPath(filepath.Join("assets"))))))

	r.Get("/", s.handleRoot)
	r.Get("/backoffice", s.handleRoot)
	r.Get("/index.html", func(w http.ResponseWriter, r *http.Request) {
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		s.handleRoot(w, r2)
	})
	r.Get("/verify-email", s.handleVerifyEmailLanding)
	r.Get("/reset-password", s.handleResetPasswordLanding)

	r.Get("/privacy-policy", s.handlePrivacyPolicy)
	r.Get("/cookie-policy", s.handleCookiePolicy)
	r.Get("/api/v1/health", s.handleHealth)
	r.Post("/api/pagamento/webhook", s.handlePagamentoWebhook)

	r.Route("/api/auth", func(r chi.Router) {
		r.With(s.authRateLimitMiddleware()).Post("/register", s.handleRegister)
		r.With(s.authRateLimitMiddleware()).Post("/login", s.handleLogin)
		r.Post("/refresh", s.handleRefresh)
		r.Post("/logout", s.handleLogout)
		r.With(s.authRateLimitMiddleware()).Post("/forgot-password", s.handleForgotPassword)
		r.With(s.authRateLimitMiddleware()).Post("/reset-password", s.handleResetPassword)
		r.Post("/verify-email", s.handleVerifyEmail)
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
			r.Get("/{id}/documenti/{docId}", s.handleGetDocumento)
			r.Get("/{id}/documenti/{docId}/preview", s.handlePreviewDocumento)
			r.Get("/{id}/documenti/{docId}/download", s.handleDownloadDocumento)
			r.Delete("/{id}/documenti/{docId}", s.handleDeleteDocumento)
		})

		r.Route("/api/pagamento", func(r chi.Router) {
			r.Get("/{token}", s.handleGetPagamento)
			r.Get("/{token}/stato", s.handleGetPagamento)
			r.Post("/crea-sessione", s.requireRoles(model.RoleOperatore, model.RoleSupervisore, model.RoleAdmin)(http.HandlerFunc(s.handleCreatePagamentoSessione)).ServeHTTP)
		})

		r.Post("/api/auth/2fa/setup", s.requireRoles(model.RoleOperatore, model.RoleSupervisore, model.RoleAdmin)(http.HandlerFunc(s.handle2FASetup)).ServeHTTP)
		r.Post("/api/auth/2fa/enable", s.requireRoles(model.RoleOperatore, model.RoleSupervisore, model.RoleAdmin)(http.HandlerFunc(s.handle2FAEnable)).ServeHTTP)
		r.Post("/api/auth/2fa/disable", s.requireRoles(model.RoleOperatore, model.RoleSupervisore, model.RoleAdmin)(http.HandlerFunc(s.handle2FADisable)).ServeHTTP)

		r.Route("/api/bo", func(r chi.Router) {
			r.Use(s.requireRoles(model.RoleOperatore, model.RoleSupervisore, model.RoleAdmin))
			r.Get("/utenti", s.handleBOListUtenti)
			r.Post("/operatori/inviti", s.handleBOInviteOperatore)
			r.Get("/utenti/{id}/sessioni", s.handleBOListUserSessions)
			r.Post("/utenti/{id}/sessioni/revoca-all", s.handleBORevokeUserSessions)
			r.Post("/sessioni/{id}/revoca", s.handleBORevokeSession)
			r.Post("/pagamenti/{token}/rimborso", s.handleBORefundPagamento)
			r.Get("/security/allowed-ips", s.handleBOListAllowedIPs)
			r.Post("/security/allowed-ips/allow", s.handleBOAllowIP)
			r.Post("/security/allowed-ips/revoke", s.handleBORevokeAllowedIP)
			r.Post("/security/allowed-ips/revoke-bulk", s.handleBORevokeAllowedIPBulk)
			r.Get("/security/blocked-ips", s.handleBOListBlockedIPs)
			r.Get("/security/evaluate-ip", s.handleBOEvaluateIP)
			r.Post("/security/blocked-ips/block", s.handleBOBlockIP)
			r.Post("/security/blocked-ips/unblock", s.handleBOUnblockIP)
			r.Post("/security/blocked-ips/unblock-bulk", s.handleBOUnblockIPBulk)
			r.Get("/security-events", s.handleBOSecurityEvents)
			r.Get("/security-events/stats", s.handleBOSecurityEventsStats)
			r.Get("/security-events/stream", s.handleBOSecurityAlertsStream)
			r.Get("/security-events/report.csv", s.handleBOSecurityEventsCSV)
			r.Get("/security-events/{id}", s.handleBOGetSecurityEvent)
			r.Get("/audit-events", s.handleBOAuditEvents)
			r.Get("/audit-events/report.csv", s.handleBOAuditEventsCSV)
			r.Get("/audit-events/{id}", s.handleBOGetAuditEvent)
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

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(r.URL.Path)
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/assets/") {
			writeErr(w, http.StatusNotFound, "endpoint non trovato")
			return
		}
		s.handleRoot(w, r)
	})

	return r
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, resolveWebPath("index.html"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"service": "visto-easy", "status": "running"})
}

func (s *Server) handlePrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, resolveWebPath("privacy-policy.html"))
}

func (s *Server) handleCookiePolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, resolveWebPath("cookie-policy.html"))
}

func resolveWebPath(relative string) string {
	relative = filepath.Clean(strings.TrimSpace(relative))
	if relative == "." || relative == string(filepath.Separator) || relative == "" {
		relative = "index.html"
	}

	cwdCandidate := filepath.Join("web", relative)
	if _, err := os.Stat(cwdCandidate); err == nil {
		return cwdCandidate
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		projectRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
		fallback := filepath.Join(projectRoot, "web", relative)
		if _, err := os.Stat(fallback); err == nil {
			return fallback
		}
	}

	return cwdCandidate
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
	if !decodeJSON(w, r, &req) {
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeErr(w, http.StatusBadRequest, "email non valida")
		return
	}
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || strings.TrimSpace(parsedEmail.Address) == "" {
		writeErr(w, http.StatusBadRequest, "email non valida")
		return
	}
	if ok, msg := validateStrongPassword(req.Password); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	pwd, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	u, err := s.store.CreateUser(model.Utente{Email: parsedEmail.Address, PasswordHash: string(pwd), Nome: req.Nome, Cognome: req.Cognome, Ruolo: model.RoleRichiedente, Attivo: false, EmailVerificata: false})
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			writeErr(w, http.StatusConflict, "utente già esistente")
			return
		}
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}

	verificationTokenRaw := make([]byte, 24)
	if _, err := rand.Read(verificationTokenRaw); err != nil {
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}
	verificationToken := hex.EncodeToString(verificationTokenRaw)
	verifyTTLMinutes := envInt("EMAIL_VERIFY_TOKEN_TTL_MINUTES", 30)
	if verifyTTLMinutes <= 0 {
		verifyTTLMinutes = 30
	}
	expiresAt := time.Now().UTC().Add(time.Duration(verifyTTLMinutes) * time.Minute)
	if _, err := s.store.CreatePasswordResetToken(model.PasswordResetToken{Token: verificationToken, Purpose: "email_verification", UserID: u.ID, Email: u.Email, ExpiresAt: expiresAt}); err != nil {
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}
	verifyURL := buildActionURL(strings.TrimSpace(os.Getenv("FRONTEND_VERIFY_EMAIL_URL")), "/verify-email", verificationToken)
	textBody := "Benvenuto su Visto Easy.\nConferma il tuo account cliccando il link: " + verifyURL + "\nScade: " + expiresAt.Format(time.RFC3339)
	htmlBody := "<p>Benvenuto su Visto Easy.</p><p>Conferma il tuo account cliccando il link: <a href=\"" + verifyURL + "\">attiva account</a></p><p>Scade: " + expiresAt.Format(time.RFC3339) + "</p>"
	if err := s.sendEmail(u.Email, "Verifica email Visto Easy", textBody, htmlBody); err != nil {
		s.recordSecurityEvent(model.SecurityEvent{Type: "EMAIL_DELIVERY_FAILED", Outcome: "registration_verification", Email: u.Email, UserID: u.ID, IP: clientIP(r)})
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "pending_verification", "user": u, "verification_sent": true, "verification_expires": expiresAt.Format(time.RFC3339)})
}

func validateStrongPassword(password string) (bool, string) {
	value := password
	if len(value) < 10 {
		return false, "password troppo corta: minimo 10 caratteri"
	}
	if len(value) > 128 {
		return false, "password troppo lunga: massimo 128 caratteri"
	}
	if !passwordHasLower.MatchString(value) {
		return false, "password non valida: aggiungi almeno una lettera minuscola"
	}
	if !passwordHasUpper.MatchString(value) {
		return false, "password non valida: aggiungi almeno una lettera maiuscola"
	}
	if !passwordHasNumber.MatchString(value) {
		return false, "password non valida: aggiungi almeno un numero"
	}
	if !passwordHasSpecial.MatchString(value) {
		return false, "password non valida: aggiungi almeno un simbolo"
	}
	return true, ""
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		OTP      string `json:"otp"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
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
	if !u.Attivo || !u.EmailVerificata {
		s.recordSecurityEvent(model.SecurityEvent{Type: "LOGIN_FAILED", Outcome: "account_not_verified", Email: email, UserID: u.ID, IP: ip, UserAgent: ua})
		writeErr(w, http.StatusForbidden, "account non ancora verificato")
		return
	}
	if isBackofficeRole(u.Ruolo) && u.TOTPEnabled {
		if !auth.ValidateTOTP(req.OTP, u.TOTPSecret) {
			s.recordSecurityEvent(model.SecurityEvent{Type: "LOGIN_FAILED", Outcome: "invalid_totp", Email: email, UserID: u.ID, IP: ip, UserAgent: ua})
			writeErr(w, http.StatusUnauthorized, "codice 2fa non valido")
			return
		}
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
	refreshID := uuid.NewString()
	refresh, _ := s.tokens.SignRefreshWithJTI(u.ID, string(u.Ruolo), refreshID)
	_, _ = s.store.CreateRefreshSession(model.RefreshSession{
		ID:        refreshID,
		UserID:    u.ID,
		Role:      string(u.Ruolo),
		ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
		Revoked:   false,
	})
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access, "refresh_token": refresh, "user": u})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	c, err := s.tokens.Parse(req.RefreshToken)
	if err != nil || c.Type != "refresh" {
		_, _ = s.store.AddAuditEvent(model.AuditEvent{
			Action:   "AUTH_REFRESH_REJECTED",
			Resource: "refresh_session",
			IP:       clientIP(r),
			Details:  map[string]any{"reason": "invalid_token"},
		})
		writeErr(w, http.StatusUnauthorized, "refresh token non valido")
		return
	}
	if strings.TrimSpace(c.ID) == "" {
		_, _ = s.store.AddAuditEvent(model.AuditEvent{
			Action:    "AUTH_REFRESH_REJECTED",
			Resource:  "refresh_session",
			ActorID:   c.UserID,
			ActorRole: c.Role,
			IP:        clientIP(r),
			Details:   map[string]any{"reason": "missing_session_id"},
		})
		writeErr(w, http.StatusUnauthorized, "refresh token non valido")
		return
	}
	session, err := s.store.GetRefreshSessionByID(c.ID)
	if err != nil || session.Revoked || session.UserID != c.UserID || session.Role != c.Role {
		_, _ = s.store.AddAuditEvent(model.AuditEvent{
			Action:     "AUTH_REFRESH_REJECTED",
			Resource:   "refresh_session",
			ResourceID: c.ID,
			ActorID:    c.UserID,
			ActorRole:  c.Role,
			IP:         clientIP(r),
			Details:    map[string]any{"reason": "invalid_session"},
		})
		writeErr(w, http.StatusUnauthorized, "refresh token non valido")
		return
	}
	access, _ := s.tokens.SignAccess(c.UserID, c.Role)
	newRefreshID := uuid.NewString()
	refresh, _ := s.tokens.SignRefreshWithJTI(c.UserID, c.Role, newRefreshID)
	_, _ = s.store.CreateRefreshSession(model.RefreshSession{
		ID:        newRefreshID,
		UserID:    c.UserID,
		Role:      c.Role,
		ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
		Revoked:   false,
	})
	_, _ = s.store.RevokeRefreshSession(c.ID, newRefreshID)
	_, _ = s.store.AddAuditEvent(model.AuditEvent{
		ActorID:    c.UserID,
		ActorRole:  c.Role,
		Action:     "AUTH_REFRESH_ROTATED",
		Resource:   "refresh_session",
		ResourceID: c.ID,
		IP:         clientIP(r),
		Details: map[string]any{
			"new_refresh_session_id": newRefreshID,
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access, "refresh_token": refresh})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if decodeJSON(w, r, &req) && strings.TrimSpace(req.RefreshToken) != "" {
		if c, err := s.tokens.Parse(req.RefreshToken); err == nil && c.Type == "refresh" && strings.TrimSpace(c.ID) != "" {
			if ok, _ := s.store.RevokeRefreshSession(c.ID, ""); ok {
				_, _ = s.store.AddAuditEvent(model.AuditEvent{
					ActorID:    c.UserID,
					ActorRole:  c.Role,
					Action:     "AUTH_LOGOUT",
					Resource:   "refresh_session",
					ResourceID: c.ID,
					IP:         clientIP(r),
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "logged_out"})
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email != "" {
		u, err := s.store.GetUserByEmail(email)
		if err == nil {
			buf := make([]byte, 24)
			if _, e := rand.Read(buf); e == nil {
				token := hex.EncodeToString(buf)
				expiresAt := time.Now().UTC().Add(30 * time.Minute)
				_, _ = s.store.CreatePasswordResetToken(model.PasswordResetToken{
					Token:     token,
					Purpose:   "password_reset",
					UserID:    u.ID,
					Email:     email,
					ExpiresAt: expiresAt,
				})
				resetURL := buildActionURL(strings.TrimSpace(os.Getenv("FRONTEND_RESET_PASSWORD_URL")), "/reset-password", token)
				textBody := "Abbiamo ricevuto una richiesta di reset password."
				htmlBody := "<p>Abbiamo ricevuto una richiesta di reset password.</p>"
				textBody += "\nUsa questo link: " + resetURL
				htmlBody += "<p>Usa questo link: <a href=\"" + resetURL + "\">reset password</a></p>"
				textBody += "\nScade: " + expiresAt.Format(time.RFC3339)
				htmlBody += "<p>Scadenza: " + expiresAt.Format(time.RFC3339) + "</p>"
				if err := s.sendEmail(email, "Reset password Visto Easy", textBody, htmlBody); err != nil {
					s.recordSecurityEvent(model.SecurityEvent{Type: "EMAIL_DELIVERY_FAILED", Outcome: "password_reset", Email: email, UserID: u.ID, IP: clientIP(r)})
				}
				s.recordSecurityEvent(model.SecurityEvent{
					Type:    "PASSWORD_RESET_REQUESTED",
					Outcome: "accepted",
					Email:   email,
					UserID:  u.ID,
					IP:      clientIP(r),
					Metadata: map[string]any{
						"expires_at":  expiresAt.Format(time.RFC3339),
						"reset_token": token,
					},
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "accepted", "message": "se l'email esiste riceverai istruzioni"})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(strings.TrimSpace(req.NewPassword)) < 8 {
		writeErr(w, http.StatusBadRequest, "password troppo corta")
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeErr(w, http.StatusBadRequest, "token non valido")
		return
	}
	resetRec, err := s.store.ConsumePasswordResetToken(token)
	if err != nil {
		_, _ = s.store.AddAuditEvent(model.AuditEvent{
			Action:   "AUTH_PASSWORD_RESET_REJECTED",
			Resource: "user",
			IP:       clientIP(r),
			Details: map[string]any{
				"reason": "invalid_or_expired_token",
			},
		})
		writeErr(w, http.StatusUnauthorized, "token non valido o scaduto")
		return
	}
	if !purposeMatches(resetRec.Purpose, "password_reset", "account_activation") {
		writeErr(w, http.StatusUnauthorized, "token non valido o scaduto")
		return
	}
	pwd, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(req.NewPassword)), 12)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}
	ok, err := s.store.UpdateUserPassword(resetRec.UserID, string(pwd))
	if err != nil || !ok {
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}
	if resetRec.Purpose == "account_activation" {
		_, _ = s.store.SetUserVerificationState(resetRec.UserID, true, true)
	}
	s.recordSecurityEvent(model.SecurityEvent{
		Type:    "PASSWORD_RESET_COMPLETED",
		Outcome: "ok",
		Email:   resetRec.Email,
		UserID:  resetRec.UserID,
		IP:      clientIP(r),
	})
	actorRole := ""
	if u, err := s.store.GetUserByID(resetRec.UserID); err == nil {
		actorRole = string(u.Ruolo)
	}
	_, _ = s.store.AddAuditEvent(model.AuditEvent{
		ActorID:    resetRec.UserID,
		ActorRole:  actorRole,
		Action:     "AUTH_PASSWORD_RESET",
		Resource:   "user",
		ResourceID: resetRec.UserID,
		IP:         clientIP(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset_completed"})
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeErr(w, http.StatusBadRequest, "token non valido")
		return
	}
	verificationRec, err := s.store.ConsumePasswordResetToken(token)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "token non valido o scaduto")
		return
	}
	if !purposeMatches(verificationRec.Purpose, "email_verification") {
		writeErr(w, http.StatusUnauthorized, "token non valido o scaduto")
		return
	}
	if ok, _ := s.store.SetUserVerificationState(verificationRec.UserID, true, true); !ok {
		writeErr(w, http.StatusNotFound, "utente non trovato")
		return
	}
	_, _ = s.store.AddAuditEvent(model.AuditEvent{ActorID: verificationRec.UserID, Action: "AUTH_EMAIL_VERIFIED", Resource: "user", ResourceID: verificationRec.UserID, Details: map[string]any{"email": verificationRec.Email}})
	writeJSON(w, http.StatusOK, map[string]any{"status": "verified"})
}

func (s *Server) handleVerifyEmailLanding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, resolveWebPath("verify-email.html"))
}

func (s *Server) handleResetPasswordLanding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, resolveWebPath("reset-password.html"))
}
func (s *Server) handle2FASetup(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil {
		writeErr(w, http.StatusUnauthorized, "non autenticato")
		return
	}
	u, err := s.store.GetUserByID(claims.UserID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "utente non trovato")
		return
	}
	secret, uri, err := auth.GenerateTOTPSecret(u.Email, "Visto Easy")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore setup 2fa")
		return
	}
	ok, err := s.store.SetUserTOTPSecret(u.ID, secret)
	if err != nil || !ok {
		writeErr(w, http.StatusInternalServerError, "errore setup 2fa")
		return
	}
	_, _ = s.store.SetUserTOTPEnabled(u.ID, false)
	s.recordSecurityEvent(model.SecurityEvent{Type: "TWO_FA_SETUP", Outcome: "pending_enable", UserID: u.ID, Email: u.Email, IP: clientIP(r)})
	s.recordAuditEvent(r, claims, "AUTH_2FA_SETUP", "user", u.ID, nil)
	writeJSON(w, http.StatusOK, map[string]any{"status": "pending", "secret": secret, "provisioning_uri": uri})
}

func (s *Server) handle2FAEnable(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil {
		writeErr(w, http.StatusUnauthorized, "non autenticato")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.store.GetUserByID(claims.UserID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "utente non trovato")
		return
	}
	if strings.TrimSpace(u.TOTPSecret) == "" {
		writeErr(w, http.StatusBadRequest, "setup 2fa non inizializzato")
		return
	}
	if !auth.ValidateTOTP(req.Code, u.TOTPSecret) {
		s.recordAuditEvent(r, claims, "AUTH_2FA_ENABLE_REJECTED", "user", u.ID, map[string]any{"reason": "invalid_code"})
		writeErr(w, http.StatusUnauthorized, "codice 2fa non valido")
		return
	}
	ok, err := s.store.SetUserTOTPEnabled(u.ID, true)
	if err != nil || !ok {
		writeErr(w, http.StatusInternalServerError, "errore attivazione 2fa")
		return
	}
	s.recordSecurityEvent(model.SecurityEvent{Type: "TWO_FA_ENABLED", Outcome: "ok", UserID: u.ID, Email: u.Email, IP: clientIP(r)})
	s.recordAuditEvent(r, claims, "AUTH_2FA_ENABLED", "user", u.ID, nil)
	writeJSON(w, http.StatusOK, map[string]any{"status": "enabled"})
}

func (s *Server) handle2FADisable(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil {
		writeErr(w, http.StatusUnauthorized, "non autenticato")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.store.GetUserByID(claims.UserID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "utente non trovato")
		return
	}
	if u.TOTPEnabled && !auth.ValidateTOTP(req.Code, u.TOTPSecret) {
		s.recordAuditEvent(r, claims, "AUTH_2FA_DISABLE_REJECTED", "user", u.ID, map[string]any{"reason": "invalid_code"})
		writeErr(w, http.StatusUnauthorized, "codice 2fa non valido")
		return
	}
	if _, err := s.store.SetUserTOTPEnabled(u.ID, false); err != nil {
		writeErr(w, http.StatusInternalServerError, "errore disattivazione 2fa")
		return
	}
	_, _ = s.store.SetUserTOTPSecret(u.ID, "")
	s.recordSecurityEvent(model.SecurityEvent{Type: "TWO_FA_DISABLED", Outcome: "ok", UserID: u.ID, Email: u.Email, IP: clientIP(r)})
	s.recordAuditEvent(r, claims, "AUTH_2FA_DISABLED", "user", u.ID, nil)
	writeJSON(w, http.StatusOK, map[string]any{"status": "disabled"})
}

func isBackofficeRole(role model.Role) bool {
	return role == model.RoleOperatore || role == model.RoleSupervisore || role == model.RoleAdmin
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
	if !decodeJSON(w, r, &req) {
		return
	}
	p, err := s.store.CreatePratica(model.Pratica{UtenteID: claims.UserID, TipoVisto: req.TipoVisto, PaeseDest: req.PaeseDest, DatiAnagrafici: req.DatiAnagrafici, DatiPassaporto: req.DatiPassaporto}, claims.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore creazione pratica")
		return
	}
	go s.notifyBackofficePraticaCreated(p)
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleListMiePratiche(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil {
		writeErr(w, http.StatusUnauthorized, "non autenticato")
		return
	}
	if claims.Role == string(model.RoleRichiedente) {
		writeJSON(w, http.StatusOK, s.store.ListPraticheByUser(claims.UserID))
		return
	}
	writeJSON(w, http.StatusOK, s.store.ListAllPratiche())
}

func (s *Server) handleGetPratica(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := s.store.GetPratica(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	claims := claimsFromCtx(r.Context())
	if claims == nil {
		writeErr(w, http.StatusUnauthorized, "non autenticato")
		return
	}
	if claims.Role == string(model.RoleRichiedente) && p.UtenteID != claims.UserID {
		writeErr(w, http.StatusForbidden, "accesso negato")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handlePatchPratica(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleRichiedente) {
		writeErr(w, http.StatusForbidden, "solo richiedente")
		return
	}
	id := chi.URLParam(r, "id")
	var data map[string]any
	if !decodeJSON(w, r, &data) {
		return
	}
	p, err := s.store.UpdatePraticaAsDraft(id, claims.UserID, data)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica non trovata")
			return
		}
		writeErr(w, http.StatusForbidden, "modifica non consentita")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeletePratica(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleRichiedente) {
		writeErr(w, http.StatusForbidden, "solo richiedente")
		return
	}
	if err := s.store.DeletePraticaAsDraft(chi.URLParam(r, "id"), claims.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica non trovata")
			return
		}
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
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
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
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Dimensione > 10*1024*1024 {
		writeErr(w, http.StatusBadRequest, "file troppo grande")
		return
	}
	doc, err := s.store.AddDocumento(id, model.Documento{Tipo: req.Tipo, NomeFile: req.NomeFile, MimeType: strings.ToLower(req.MimeType), Dimensione: req.Dimensione})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore upload")
		return
	}
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
	docs, err := s.store.ListDocumenti(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func (s *Server) handleGetDocumento(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	praticaID := chi.URLParam(r, "id")
	docID := chi.URLParam(r, "docId")
	p, err := s.store.GetPratica(praticaID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	if claims == nil || (claims.Role == string(model.RoleRichiedente) && p.UtenteID != claims.UserID) {
		writeErr(w, http.StatusForbidden, "accesso negato")
		return
	}
	d, err := s.store.GetDocumento(praticaID, docID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "documento non trovato")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleDownloadDocumento(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	praticaID := chi.URLParam(r, "id")
	docID := chi.URLParam(r, "docId")
	p, err := s.store.GetPratica(praticaID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	if claims == nil || (claims.Role == string(model.RoleRichiedente) && p.UtenteID != claims.UserID) {
		writeErr(w, http.StatusForbidden, "accesso negato")
		return
	}
	d, err := s.store.GetDocumento(praticaID, docID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "documento non trovato")
		return
	}

	if s.presign != nil && strings.TrimSpace(d.S3Key) != "" {
		session, err := s.presign.PresignDocumentDownload(d.S3Key, d.NomeFile)
		if err == nil && strings.TrimSpace(session.DownloadURL) != "" {
			http.Redirect(w, r, session.DownloadURL, http.StatusFound)
			return
		}
	}

	writeErr(w, http.StatusServiceUnavailable, "download non disponibile: storage non configurato")
}

func (s *Server) handlePreviewDocumento(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	praticaID := chi.URLParam(r, "id")
	docID := chi.URLParam(r, "docId")
	p, err := s.store.GetPratica(praticaID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	if claims == nil || (claims.Role == string(model.RoleRichiedente) && p.UtenteID != claims.UserID) {
		writeErr(w, http.StatusForbidden, "accesso negato")
		return
	}
	d, err := s.store.GetDocumento(praticaID, docID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "documento non trovato")
		return
	}

	if s.presign != nil && strings.TrimSpace(d.S3Key) != "" {
		session, err := s.presign.PresignDocumentDownload(d.S3Key, "")
		if err == nil && strings.TrimSpace(session.DownloadURL) != "" {
			http.Redirect(w, r, session.DownloadURL, http.StatusFound)
			return
		}
	}

	writeErr(w, http.StatusServiceUnavailable, "anteprima non disponibile: storage non configurato")
}

func (s *Server) handleDeleteDocumento(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	praticaID := chi.URLParam(r, "id")
	docID := chi.URLParam(r, "docId")
	p, err := s.store.GetPratica(praticaID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	if claims == nil {
		writeErr(w, http.StatusUnauthorized, "non autenticato")
		return
	}
	if claims.Role == string(model.RoleRichiedente) {
		if p.UtenteID != claims.UserID {
			writeErr(w, http.StatusForbidden, "accesso negato")
			return
		}
		if p.Stato != model.StatoBozza {
			writeErr(w, http.StatusForbidden, "eliminazione documenti consentita solo in bozza")
			return
		}
	}

	deleted, err := s.store.DeleteDocumento(praticaID, docID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore eliminazione documento")
		return
	}
	if !deleted {
		writeErr(w, http.StatusNotFound, "documento non trovato")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	var req struct {
		PraticaID, Provider string
		Importo             float64
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Provider == "" {
		req.Provider = "stripe"
	}
	pay, err := s.store.CreatePayment(req.PraticaID, req.Provider, req.Importo)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	pay, err = s.enrichPaymentCheckoutSession(pay)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "errore creazione sessione pagamento provider")
		return
	}
	_, _ = s.store.ChangePraticaState(req.PraticaID, claimsFromCtx(r.Context()).UserID, model.StatoAttendePagamento, "link pagamento generato")
	s.notifyPaymentLink(pay)
	writeJSON(w, http.StatusCreated, pay)
}

func (s *Server) handleGetPagamento(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	pay, err := s.store.GetPaymentByToken(token)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pagamento non trovato")
		return
	}
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
	if err != nil {
		writeErr(w, http.StatusNotFound, "pagamento non trovato")
		return
	}
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoPagamentoRicevuto, "webhook provider")
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoVistoInElaborazione, "generazione visto")
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoVistoEmesso, "visto emesso")
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoCompletata, "visto consegnato")
	s.notifyPagamentoCompletato(pay)
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

func (s *Server) handleBOInviteOperatore(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil {
		writeErr(w, http.StatusUnauthorized, "non autenticato")
		return
	}
	if claims.Role != string(model.RoleAdmin) && claims.Role != string(model.RoleSupervisore) {
		writeErr(w, http.StatusForbidden, "solo admin o supervisore")
		return
	}

	var req struct {
		Email   string `json:"email"`
		Nome    string `json:"nome"`
		Cognome string `json:"cognome"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	nome := strings.TrimSpace(req.Nome)
	cognome := strings.TrimSpace(req.Cognome)
	if email == "" || !strings.Contains(email, "@") {
		writeErr(w, http.StatusBadRequest, "email non valida")
		return
	}
	if nome == "" || cognome == "" {
		writeErr(w, http.StatusBadRequest, "nome e cognome obbligatori")
		return
	}

	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}
	tempPassword := hex.EncodeToString(buf)
	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), 12)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}

	u, err := s.store.CreateUser(model.Utente{
		Email:           email,
		PasswordHash:    string(hash),
		Nome:            nome,
		Cognome:         cognome,
		Ruolo:           model.RoleOperatore,
		Attivo:          false,
		EmailVerificata: false,
	})
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			writeErr(w, http.StatusConflict, "operatore gia esistente")
			return
		}
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}

	resetTokenRaw := make([]byte, 24)
	if _, err := rand.Read(resetTokenRaw); err != nil {
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}
	inviteToken := hex.EncodeToString(resetTokenRaw)
	expiresAt := time.Now().UTC().Add(72 * time.Hour)
	if _, err := s.store.CreatePasswordResetToken(model.PasswordResetToken{
		Token:     inviteToken,
		Purpose:   "account_activation",
		UserID:    u.ID,
		Email:     email,
		ExpiresAt: expiresAt,
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "errore interno")
		return
	}

	inviteURL := buildActionURL(strings.TrimSpace(os.Getenv("FRONTEND_OPERATOR_INVITE_URL")), "/reset-password", inviteToken)

	textBody := "Sei stato invitato come operatore su Visto Easy."
	htmlBody := "<p>Sei stato invitato come operatore su Visto Easy.</p>"
	if inviteURL != "" {
		textBody += "\nCompleta l'attivazione qui: " + inviteURL
		htmlBody += "<p>Completa l'attivazione qui: <a href=\"" + inviteURL + "\">attiva account</a></p>"
	} else {
		textBody += "\nToken invito: " + inviteToken
		htmlBody += "<p>Token invito: <strong>" + inviteToken + "</strong></p>"
	}
	textBody += "\nScadenza: " + expiresAt.Format(time.RFC3339)
	htmlBody += "<p>Scadenza: " + expiresAt.Format(time.RFC3339) + "</p>"

	if err := s.sendEmail(email, "Invito operatore Visto Easy", textBody, htmlBody); err != nil {
		s.recordSecurityEvent(model.SecurityEvent{Type: "EMAIL_DELIVERY_FAILED", Outcome: "operator_invite", Email: email, UserID: u.ID, IP: clientIP(r)})
	}

	s.recordAuditEvent(r, claims, "BO_OPERATOR_INVITED", "user", u.ID, map[string]any{"email": email})
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":       "invited",
		"invited_user": u,
		"invite_url":   inviteURL,
		"invite_token": inviteToken,
		"expires_at":   expiresAt.Format(time.RFC3339),
	})
}

func buildActionURL(baseURL, fallbackPath, token string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = fallbackPath
	}
	baseURL = strings.ReplaceAll(baseURL, "{token}", token)
	if u, err := url.Parse(baseURL); err == nil {
		q := u.Query()
		q.Set("token", token)
		u.RawQuery = q.Encode()
		return u.String()
	}

	sep := "?"
	if strings.Contains(baseURL, "?") {
		sep = "&"
	}
	if strings.HasSuffix(baseURL, "?") || strings.HasSuffix(baseURL, "&") {
		sep = ""
	}
	return baseURL + sep + "token=" + token
}

func purposeMatches(actual string, expected ...string) bool {
	actual = strings.ToLower(strings.TrimSpace(actual))
	if actual == "" {
		return true
	}
	for _, item := range expected {
		if actual == strings.ToLower(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}

func (s *Server) handleBOListUserSessions(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleAdmin) {
		writeErr(w, http.StatusForbidden, "solo admin")
		return
	}
	userID := strings.TrimSpace(chi.URLParam(r, "id"))
	if userID == "" {
		writeErr(w, http.StatusBadRequest, "utente non valido")
		return
	}
	if _, err := s.store.GetUserByID(userID); err != nil {
		writeErr(w, http.StatusNotFound, "utente non trovato")
		return
	}
	items := s.store.ListRefreshSessionsByUser(userID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (s *Server) handleBORevokeUserSessions(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleAdmin) {
		writeErr(w, http.StatusForbidden, "solo admin")
		return
	}
	userID := strings.TrimSpace(chi.URLParam(r, "id"))
	if userID == "" {
		writeErr(w, http.StatusBadRequest, "utente non valido")
		return
	}
	if _, err := s.store.GetUserByID(userID); err != nil {
		writeErr(w, http.StatusNotFound, "utente non trovato")
		return
	}
	revoked, err := s.store.RevokeAllRefreshSessionsByUser(userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore revoca sessioni")
		return
	}
	s.recordSecurityEvent(model.SecurityEvent{Type: "ADMIN_SESSIONS_REVOKED", Outcome: "bulk", UserID: claims.UserID, Metadata: map[string]any{"target_user_id": userID, "revoked": revoked}})
	s.recordAuditEvent(r, claims, "ADMIN_REVOKE_USER_SESSIONS", "refresh_session", userID, map[string]any{"revoked": revoked})
	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked", "user_id": userID, "revoked": revoked})
}

func (s *Server) handleBORevokeSession(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleAdmin) {
		writeErr(w, http.StatusForbidden, "solo admin")
		return
	}
	sessionID := strings.TrimSpace(chi.URLParam(r, "id"))
	if sessionID == "" {
		writeErr(w, http.StatusBadRequest, "sessione non valida")
		return
	}
	ok, err := s.store.RevokeRefreshSession(sessionID, "admin_manual_revoke")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore revoca sessione")
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "sessione non trovata o già revocata")
		return
	}
	s.recordSecurityEvent(model.SecurityEvent{Type: "ADMIN_SESSION_REVOKED", Outcome: "single", UserID: claims.UserID, Metadata: map[string]any{"session_id": sessionID}})
	s.recordAuditEvent(r, claims, "ADMIN_REVOKE_SESSION", "refresh_session", sessionID, nil)
	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked", "session_id": sessionID})
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

func (s *Server) handleBOAuditEvents(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	filtered := s.filterAuditEvents(r)
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

func (s *Server) handleBOGetAuditEvent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id evento non valido")
		return
	}
	evt, err := s.store.GetAuditEventByID(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "evento audit non trovato")
		return
	}
	writeJSON(w, http.StatusOK, evt)
}

func (s *Server) handleBOAuditEventsCSV(w http.ResponseWriter, r *http.Request) {
	items := s.filterAuditEvents(r)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_events.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "actor_id", "actor_role", "action", "resource", "resource_id", "ip", "creato_il"})
	for _, evt := range items {
		_ = cw.Write([]string{
			evt.ID,
			evt.ActorID,
			evt.ActorRole,
			evt.Action,
			evt.Resource,
			evt.ResourceID,
			evt.IP,
			evt.CreatoIl.Format(time.RFC3339),
		})
	}
	cw.Flush()
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
		"total":                len(items),
		"window_minutes":       windowMinutes,
		"recent_failed_logins": recentFailed,
		"recent_locked_logins": recentLocked,
		"by_type":              byType,
		"by_outcome":           byOutcome,
		"top_failed_ips":       topMapEntries(recentFailedByIP, 5),
		"top_failed_emails":    topMapEntries(recentFailedByEmail, 5),
		"high_risk_detected":   recentFailed >= envInt("SECURITY_ALERT_FAILED_THRESHOLD", 5) || recentLocked > 0,
	})
}

func (s *Server) handleBOListBlockedIPs(w http.ResponseWriter, _ *http.Request) {
	items := s.store.ListBlockedIPs()
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}

func (s *Server) handleBOEvaluateIP(w http.ResponseWriter, r *http.Request) {
	input := strings.TrimSpace(r.URL.Query().Get("ip"))
	if input == "" {
		input = clientIP(r)
	}
	ip, err := normalizeIPAddress(input)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "ip non valido")
		return
	}

	decision := s.evaluateClientIPPolicy(ip)
	writeJSON(w, http.StatusOK, map[string]any{
		"ip":       ip,
		"decision": decision,
	})
}

func (s *Server) handleBOListAllowedIPs(w http.ResponseWriter, _ *http.Request) {
	items := s.store.ListAllowedIPs()
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}

func (s *Server) handleBOAllowIP(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		IP         string `json:"ip"`
		Reason     string `json:"reason"`
		TTLMinutes int    `json:"ttl_minutes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	target, err := normalizeBlockTarget(req.IP)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "target non valido (usa IP o CIDR)")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "manual_allow"
	}
	ttlMinutes := req.TTLMinutes
	if ttlMinutes <= 0 {
		ttlMinutes = envInt("SECURITY_ALLOW_IP_DEFAULT_TTL_MINUTES", 240)
	}
	now := time.Now().UTC()
	entry := model.AllowedIP{
		IP:        target,
		Reason:    reason,
		AllowedBy: claims.UserID,
		AllowedAt: now,
	}
	if ttlMinutes > 0 {
		exp := now.Add(time.Duration(ttlMinutes) * time.Minute)
		entry.ExpiresAt = &exp
	}
	entry, err = s.store.UpsertAllowedIP(entry)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore salvataggio allowlist")
		return
	}
	s.recordSecurityEvent(model.SecurityEvent{
		Type:    "IP_ALLOWED",
		Outcome: "manual",
		UserID:  claims.UserID,
		IP:      target,
		Metadata: map[string]any{
			"reason":      reason,
			"ttl_minutes": ttlMinutes,
		},
	})
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleBORevokeAllowedIP(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		IP string `json:"ip"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	target, err := normalizeBlockTarget(req.IP)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "target non valido (usa IP o CIDR)")
		return
	}
	removed, err := s.store.RemoveAllowedIP(target)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore aggiornamento allowlist")
		return
	}
	if !removed {
		writeErr(w, http.StatusNotFound, "target non presente in allowlist")
		return
	}
	s.recordSecurityEvent(model.SecurityEvent{
		Type:    "IP_ALLOW_REVOKED",
		Outcome: "manual",
		UserID:  claims.UserID,
		IP:      target,
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked", "ip": target})
}

func (s *Server) handleBORevokeAllowedIPBulk(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		Targets   []string `json:"targets"`
		RevokeAll bool     `json:"revoke_all"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	targets := make([]string, 0)
	if req.RevokeAll {
		for _, item := range s.store.ListAllowedIPs() {
			targets = append(targets, item.IP)
		}
	} else {
		targets = req.Targets
	}

	removed := make([]string, 0)
	notFound := make([]string, 0)
	invalid := make([]string, 0)
	for _, raw := range targets {
		target, err := normalizeBlockTarget(raw)
		if err != nil {
			invalid = append(invalid, strings.TrimSpace(raw))
			continue
		}
		ok, err := s.store.RemoveAllowedIP(target)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "errore aggiornamento allowlist")
			return
		}
		if ok {
			removed = append(removed, target)
			s.recordSecurityEvent(model.SecurityEvent{
				Type:    "IP_ALLOW_REVOKED",
				Outcome: "bulk",
				UserID:  claims.UserID,
				IP:      target,
			})
		} else {
			notFound = append(notFound, target)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"removed":   removed,
		"not_found": notFound,
		"invalid":   invalid,
		"count": map[string]any{
			"removed":   len(removed),
			"not_found": len(notFound),
			"invalid":   len(invalid),
		},
	})
}

func (s *Server) handleBOBlockIP(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		IP         string `json:"ip"`
		Reason     string `json:"reason"`
		TTLMinutes int    `json:"ttl_minutes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	target, err := normalizeBlockTarget(req.IP)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "target non valido (usa IP o CIDR)")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "manual_block"
	}
	ttlMinutes := req.TTLMinutes
	if ttlMinutes <= 0 {
		ttlMinutes = envInt("SECURITY_BLOCK_IP_DEFAULT_TTL_MINUTES", 120)
	}
	now := time.Now().UTC()
	entry := model.BlockedIP{
		IP:        target,
		Reason:    reason,
		BlockedBy: claims.UserID,
		BlockedAt: now,
	}
	if ttlMinutes > 0 {
		exp := now.Add(time.Duration(ttlMinutes) * time.Minute)
		entry.ExpiresAt = &exp
	}
	entry, err = s.store.UpsertBlockedIP(entry)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore salvataggio denylist")
		return
	}
	s.recordSecurityEvent(model.SecurityEvent{
		Type:    "IP_BLOCKED",
		Outcome: "manual",
		UserID:  claims.UserID,
		IP:      target,
		Metadata: map[string]any{
			"reason":      reason,
			"ttl_minutes": ttlMinutes,
		},
	})
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleBOUnblockIP(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		IP string `json:"ip"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	target, err := normalizeBlockTarget(req.IP)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "target non valido (usa IP o CIDR)")
		return
	}
	removed, err := s.store.RemoveBlockedIP(target)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "errore aggiornamento denylist")
		return
	}
	if !removed {
		writeErr(w, http.StatusNotFound, "ip non presente in denylist")
		return
	}
	s.recordSecurityEvent(model.SecurityEvent{
		Type:    "IP_UNBLOCKED",
		Outcome: "manual",
		UserID:  claims.UserID,
		IP:      target,
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "unblocked", "ip": target})
}

func (s *Server) handleBOUnblockIPBulk(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		Targets    []string `json:"targets"`
		UnblockAll bool     `json:"unblock_all"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	targets := make([]string, 0)
	if req.UnblockAll {
		for _, item := range s.store.ListBlockedIPs() {
			targets = append(targets, item.IP)
		}
	} else {
		targets = req.Targets
	}

	removed := make([]string, 0)
	notFound := make([]string, 0)
	invalid := make([]string, 0)
	for _, raw := range targets {
		target, err := normalizeBlockTarget(raw)
		if err != nil {
			invalid = append(invalid, strings.TrimSpace(raw))
			continue
		}
		ok, err := s.store.RemoveBlockedIP(target)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "errore aggiornamento denylist")
			return
		}
		if ok {
			removed = append(removed, target)
			s.recordSecurityEvent(model.SecurityEvent{
				Type:    "IP_UNBLOCKED",
				Outcome: "bulk",
				UserID:  claims.UserID,
				IP:      target,
			})
		} else {
			notFound = append(notFound, target)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"removed":   removed,
		"not_found": notFound,
		"invalid":   invalid,
		"count": map[string]any{
			"removed":   len(removed),
			"not_found": len(notFound),
			"invalid":   len(invalid),
		},
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

func (s *Server) filterAuditEvents(r *http.Request) []model.AuditEvent {
	action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
	resource := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("resource")))
	actorID := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	fromTs := parseOptionalTime(r.URL.Query().Get("from"))
	toTs := parseOptionalTime(r.URL.Query().Get("to"))

	all := s.store.ListAuditEvents()
	filtered := make([]model.AuditEvent, 0, len(all))
	for _, evt := range all {
		if action != "" && strings.ToLower(evt.Action) != action {
			continue
		}
		if resource != "" && strings.ToLower(evt.Resource) != resource {
			continue
		}
		if actorID != "" && evt.ActorID != actorID {
			continue
		}
		if !fromTs.IsZero() && evt.CreatoIl.Before(fromTs) {
			continue
		}
		if !toTs.IsZero() && evt.CreatoIl.After(toTs) {
			continue
		}
		if q != "" {
			h := strings.ToLower(strings.Join([]string{evt.Action, evt.Resource, evt.ResourceID, evt.ActorID, evt.ActorRole, evt.IP}, "|"))
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

func (s *Server) sendEmail(to, subject, textBody, htmlBody string) error {
	if s.mailer == nil {
		return nil
	}
	return s.mailer.Send(to, subject, textBody, htmlBody)
}

func (s *Server) notifyBackofficePraticaCreated(p model.Pratica) {
	if s.mailer == nil {
		return
	}
	praticante, err := s.store.GetUserByID(p.UtenteID)
	if err != nil {
		return
	}
	recipients := s.backofficeNotificationRecipients()
	if len(recipients) == 0 {
		return
	}
	subject := "Nuovo visto inserito: " + p.Codice
	textBody := fmt.Sprintf("Nuovo visto inserito.\nCodice: %s\nUtente: %s %s <%s>\nTipo visto: %s\nPaese destinazione: %s\nStato: %s", p.Codice, praticante.Nome, praticante.Cognome, praticante.Email, p.TipoVisto, p.PaeseDest, p.Stato)
	htmlBody := fmt.Sprintf("<p>Nuovo visto inserito.</p><ul><li><strong>Codice:</strong> %s</li><li><strong>Utente:</strong> %s %s &lt;%s&gt;</li><li><strong>Tipo visto:</strong> %s</li><li><strong>Paese destinazione:</strong> %s</li><li><strong>Stato:</strong> %s</li></ul>", p.Codice, praticante.Nome, praticante.Cognome, praticante.Email, p.TipoVisto, p.PaeseDest, p.Stato)
	for _, recipient := range recipients {
		if err := s.sendEmail(recipient, subject, textBody, htmlBody); err != nil {
			s.recordSecurityEvent(model.SecurityEvent{Type: "EMAIL_DELIVERY_FAILED", Outcome: "new_pratica_notify", Email: recipient, UserID: praticante.ID, IP: ""})
		}
	}
}

func (s *Server) backofficeNotificationRecipients() []string {
	raw := strings.TrimSpace(os.Getenv("BACKOFFICE_NOTIFY_EMAILS"))
	if raw != "" {
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		seen := map[string]struct{}{}
		for _, part := range parts {
			email := strings.ToLower(strings.TrimSpace(part))
			if email == "" {
				continue
			}
			if _, ok := seen[email]; ok {
				continue
			}
			seen[email] = struct{}{}
			out = append(out, email)
		}
		return out
	}
	users := s.store.ListUsers()
	out := make([]string, 0)
	seen := map[string]struct{}{}
	for _, u := range users {
		if !u.Attivo || !u.EmailVerificata {
			continue
		}
		if u.Ruolo != model.RoleOperatore && u.Ruolo != model.RoleSupervisore && u.Ruolo != model.RoleAdmin {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(u.Email))
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		out = append(out, email)
	}
	return out
}

func (s *Server) notifyPaymentLink(pay model.Pagamento) {
	pratica, err := s.store.GetPratica(pay.PraticaID)
	if err != nil {
		return
	}
	utente, err := s.store.GetUserByID(pratica.UtenteID)
	if err != nil || strings.TrimSpace(utente.Email) == "" {
		return
	}
	subject := "Link pagamento pratica " + pratica.Codice
	textBody := fmt.Sprintf("La tua pratica %s e' pronta per il pagamento.\nImporto: %.2f %s\nLink: %s", pratica.Codice, pay.Importo, strings.ToUpper(pay.Valuta), pay.LinkPagamento)
	htmlBody := fmt.Sprintf("<p>La tua pratica <strong>%s</strong> e' pronta per il pagamento.</p><p>Importo: <strong>%.2f %s</strong></p><p><a href=\"%s\">Vai al pagamento</a></p>", pratica.Codice, pay.Importo, strings.ToUpper(pay.Valuta), pay.LinkPagamento)
	if err := s.sendEmail(utente.Email, subject, textBody, htmlBody); err != nil {
		s.recordSecurityEvent(model.SecurityEvent{Type: "EMAIL_DELIVERY_FAILED", Outcome: "payment_link", Email: utente.Email, UserID: utente.ID})
	}
}

func (s *Server) notifyPagamentoCompletato(pay model.Pagamento) {
	pratica, err := s.store.GetPratica(pay.PraticaID)
	if err != nil {
		return
	}
	utente, err := s.store.GetUserByID(pratica.UtenteID)
	if err != nil || strings.TrimSpace(utente.Email) == "" {
		return
	}
	subject := "Pagamento ricevuto e visto emesso"
	textBody := fmt.Sprintf("Pagamento ricevuto per pratica %s. Il visto e' stato emesso.", pratica.Codice)
	htmlBody := fmt.Sprintf("<p>Pagamento ricevuto per pratica <strong>%s</strong>.</p><p>Il visto e' stato emesso.</p>", pratica.Codice)
	if err := s.sendEmail(utente.Email, subject, textBody, htmlBody); err != nil {
		s.recordSecurityEvent(model.SecurityEvent{Type: "EMAIL_DELIVERY_FAILED", Outcome: "payment_completed", Email: utente.Email, UserID: utente.ID})
	}
}

func (s *Server) handleBOChangeStato(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		Stato model.StatoPratica `json:"stato"`
		Nota  string             `json:"nota"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	praticaID := chi.URLParam(r, "id")
	p, err := s.store.ChangePraticaState(praticaID, claims.UserID, req.Stato, req.Nota)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica non trovata")
			return
		}
		writeErr(w, http.StatusBadRequest, "transizione non valida")
		return
	}
	s.recordAuditEvent(r, claims, "PRATICA_CHANGE_STATE", "pratica", praticaID, map[string]any{"new_state": req.Stato, "note": req.Nota})
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleBOAssegna(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		OperatoreID string `json:"operatore_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	praticaID := chi.URLParam(r, "id")
	operatorID := strings.TrimSpace(req.OperatoreID)
	p, err := s.store.AssignOperatore(praticaID, operatorID, claims.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica o operatore non trovato")
			return
		}
		writeErr(w, http.StatusBadRequest, "assegnazione non valida")
		return
	}
	s.recordAuditEvent(r, claims, "PRATICA_ASSIGN_OPERATOR", "pratica", praticaID, map[string]any{"operatore_id": operatorID})
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleBOAddNota(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		Messaggio string `json:"messaggio"`
		Interna   bool   `json:"interna"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	p, err := s.store.AddNota(chi.URLParam(r, "id"), claims.UserID, req.Messaggio, req.Interna)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica non trovata")
			return
		}
		writeErr(w, http.StatusBadRequest, "nota non valida")
		return
	}
	s.recordAuditEvent(r, claims, "PRATICA_ADD_NOTE", "pratica", chi.URLParam(r, "id"), map[string]any{"interna": req.Interna})
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleBORichiediDoc(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		Documento string `json:"documento"`
		Nota      string `json:"nota"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	p, err := s.store.RequestDocumento(chi.URLParam(r, "id"), claims.UserID, req.Documento, req.Nota)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pratica non trovata")
			return
		}
		writeErr(w, http.StatusBadRequest, "richiesta documento non valida")
		return
	}
	s.recordAuditEvent(r, claims, "PRATICA_REQUEST_DOCUMENT", "pratica", chi.URLParam(r, "id"), map[string]any{"documento": req.Documento})
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleBOCreateLinkPagamento(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	var req struct {
		Importo  float64 `json:"importo"`
		Provider string  `json:"provider"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Provider == "" {
		req.Provider = "stripe"
	}
	praticaID := chi.URLParam(r, "id")
	pay, err := s.store.CreatePayment(praticaID, req.Provider, req.Importo)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	pay, err = s.enrichPaymentCheckoutSession(pay)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "errore creazione sessione pagamento provider")
		return
	}
	_, _ = s.store.ChangePraticaState(praticaID, claims.UserID, model.StatoAttendePagamento, "link pagamento generato")
	s.notifyPaymentLink(pay)
	s.recordAuditEvent(r, claims, "PRATICA_CREATE_PAYMENT_LINK", "pratica", praticaID, map[string]any{"payment_token": pay.Token, "provider": pay.Provider, "amount": pay.Importo})
	writeJSON(w, http.StatusCreated, pay)
}

func (s *Server) handleBORefundPagamento(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		writeErr(w, http.StatusBadRequest, "token pagamento non valido")
		return
	}
	var req struct {
		Amount float64 `json:"amount"`
		Reason string  `json:"reason"`
	}
	if r.ContentLength > 0 {
		if !decodeJSON(w, r, &req) {
			return
		}
	}

	pay, err := s.store.RefundPaymentByToken(token)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "pagamento non trovato")
			return
		}
		if errors.Is(err, store.ErrInvalidState) {
			writeErr(w, http.StatusConflict, "rimborso non consentito nello stato corrente")
			return
		}
		writeErr(w, http.StatusInternalServerError, "errore rimborso")
		return
	}

	outcome := "full"
	if req.Amount > 0 && req.Amount < pay.Importo {
		outcome = "partial"
	}
	reason := strings.TrimSpace(req.Reason)
	s.recordSecurityEvent(model.SecurityEvent{
		Type:    "PAYMENT_REFUNDED",
		Outcome: outcome,
		UserID:  claims.UserID,
		Metadata: map[string]any{
			"payment_token": token,
			"pratica_id":    pay.PraticaID,
			"provider":      pay.Provider,
			"reason":        reason,
			"amount":        req.Amount,
		},
	})
	s.recordAuditEvent(r, claims, "PAYMENT_REFUND", "payment", token, map[string]any{"pratica_id": pay.PraticaID, "provider": pay.Provider, "amount": req.Amount, "reason": reason, "outcome": outcome})

	writeJSON(w, http.StatusOK, map[string]any{"status": "refunded", "payment": pay})
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
	s.recordAuditEvent(r, claims, "PRATICA_SEND_VISTO", "pratica", id, nil)
	writeJSON(w, http.StatusOK, map[string]any{"status": "visto inviato"})
}

func (s *Server) handleBOStats(w http.ResponseWriter, _ *http.Request) {
	all := s.store.ListAllPratiche()
	stats := map[model.StatoPratica]int{}
	for _, p := range all {
		stats[p.Stato]++
	}
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
			decision := s.evaluateClientIPPolicy(key)
			if decision.Action == "allow" && decision.AllowedBy != nil {
				s.recordSecurityEvent(model.SecurityEvent{
					Type:      "IP_ALLOWLIST_MATCH",
					Outcome:   "allowed",
					IP:        key,
					UserAgent: strings.TrimSpace(r.UserAgent()),
					Metadata: map[string]any{
						"path":        r.URL.Path,
						"reason":      decision.AllowedBy.Reason,
						"allowed_by":  decision.AllowedBy.AllowedBy,
						"allow_rule":  decision.AllowedBy.IP,
						"expires_at":  formatOptTime(decision.AllowedBy.ExpiresAt),
						"policy_rule": decision.Reason,
					},
				})
				next.ServeHTTP(w, r)
				return
			}
			if decision.Action == "block" && decision.BlockedBy != nil {
				s.recordSecurityEvent(model.SecurityEvent{
					Type:      "IP_BLOCKED_REQUEST",
					Outcome:   "blocked",
					IP:        key,
					UserAgent: strings.TrimSpace(r.UserAgent()),
					Metadata: map[string]any{
						"path":        r.URL.Path,
						"reason":      decision.BlockedBy.Reason,
						"blocked_by":  decision.BlockedBy.BlockedBy,
						"block_rule":  decision.BlockedBy.IP,
						"expires_at":  formatOptTime(decision.BlockedBy.ExpiresAt),
						"policy_rule": decision.Reason,
					},
				})
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "ip bloccato"})
				return
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

func normalizeIPAddress(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", errors.New("empty ip")
	}
	if host, _, err := net.SplitHostPort(v); err == nil && strings.TrimSpace(host) != "" {
		v = strings.TrimSpace(host)
	}
	ip := net.ParseIP(v)
	if ip == nil {
		return "", errors.New("invalid ip")
	}
	return ip.String(), nil
}

func normalizeBlockTarget(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", errors.New("empty target")
	}
	if host, _, err := net.SplitHostPort(v); err == nil && strings.TrimSpace(host) != "" {
		v = strings.TrimSpace(host)
	}
	if ip := net.ParseIP(v); ip != nil {
		return ip.String(), nil
	}
	if _, network, err := net.ParseCIDR(v); err == nil {
		return network.String(), nil
	}
	return "", errors.New("invalid target")
}

func ipMatchesBlockedTarget(clientIP net.IP, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" || clientIP == nil {
		return false
	}
	if ip := net.ParseIP(target); ip != nil {
		return clientIP.Equal(ip)
	}
	if _, network, err := net.ParseCIDR(target); err == nil {
		return network.Contains(clientIP)
	}
	return false
}

type ipPolicyDecision struct {
	Action    string           `json:"action"`
	Reason    string           `json:"reason"`
	BlockedBy *model.BlockedIP `json:"blocked_by,omitempty"`
	AllowedBy *model.AllowedIP `json:"allowed_by,omitempty"`
}

func (s *Server) evaluateClientIPPolicy(clientIPRaw string) ipPolicyDecision {
	clientIP := net.ParseIP(strings.TrimSpace(clientIPRaw))
	if clientIP == nil {
		return ipPolicyDecision{Action: "allow", Reason: "invalid_ip_fallback"}
	}

	blockedMatch, hasBlocked := bestBlockedMatch(clientIP, s.store.ListBlockedIPs())
	allowedMatch, hasAllowed := bestAllowedMatch(clientIP, s.store.ListAllowedIPs())

	if hasBlocked && isExactRule(blockedMatch.IP) {
		return ipPolicyDecision{Action: "block", Reason: "exact_block_rule", BlockedBy: &blockedMatch, AllowedBy: optAllowedPtr(hasAllowed, allowedMatch)}
	}
	if hasAllowed && isExactRule(allowedMatch.IP) {
		return ipPolicyDecision{Action: "allow", Reason: "exact_allow_rule", AllowedBy: &allowedMatch, BlockedBy: optBlockedPtr(hasBlocked, blockedMatch)}
	}
	if hasBlocked && hasAllowed {
		blockSpec := ruleSpecificity(blockedMatch.IP)
		allowSpec := ruleSpecificity(allowedMatch.IP)
		if blockSpec >= allowSpec {
			return ipPolicyDecision{Action: "block", Reason: "cidr_block_precedence", BlockedBy: &blockedMatch, AllowedBy: &allowedMatch}
		}
		return ipPolicyDecision{Action: "allow", Reason: "cidr_allow_precedence", AllowedBy: &allowedMatch, BlockedBy: &blockedMatch}
	}
	if hasBlocked {
		return ipPolicyDecision{Action: "block", Reason: "block_rule", BlockedBy: &blockedMatch}
	}
	if hasAllowed {
		return ipPolicyDecision{Action: "allow", Reason: "allow_rule", AllowedBy: &allowedMatch}
	}
	return ipPolicyDecision{Action: "allow", Reason: "no_matching_rule"}
}

func bestBlockedMatch(clientIP net.IP, rules []model.BlockedIP) (model.BlockedIP, bool) {
	found := false
	best := model.BlockedIP{}
	bestSpec := -1
	for _, rule := range rules {
		if !ipMatchesBlockedTarget(clientIP, rule.IP) {
			continue
		}
		spec := ruleSpecificity(rule.IP)
		if !found || spec > bestSpec || (spec == bestSpec && rule.BlockedAt.After(best.BlockedAt)) {
			best = rule
			bestSpec = spec
			found = true
		}
	}
	return best, found
}

func bestAllowedMatch(clientIP net.IP, rules []model.AllowedIP) (model.AllowedIP, bool) {
	found := false
	best := model.AllowedIP{}
	bestSpec := -1
	for _, rule := range rules {
		if !ipMatchesBlockedTarget(clientIP, rule.IP) {
			continue
		}
		spec := ruleSpecificity(rule.IP)
		if !found || spec > bestSpec || (spec == bestSpec && rule.AllowedAt.After(best.AllowedAt)) {
			best = rule
			bestSpec = spec
			found = true
		}
	}
	return best, found
}

func ruleSpecificity(target string) int {
	target = strings.TrimSpace(target)
	if ip := net.ParseIP(target); ip != nil {
		if ip.To4() != nil {
			return 32
		}
		return 128
	}
	if _, network, err := net.ParseCIDR(target); err == nil {
		ones, _ := network.Mask.Size()
		return ones
	}
	return -1
}

func isExactRule(target string) bool {
	return net.ParseIP(strings.TrimSpace(target)) != nil
}

func optAllowedPtr(ok bool, value model.AllowedIP) *model.AllowedIP {
	if !ok {
		return nil
	}
	v := value
	return &v
}

func optBlockedPtr(ok bool, value model.BlockedIP) *model.BlockedIP {
	if !ok {
		return nil
	}
	v := value
	return &v
}

func (s *Server) recordSecurityEvent(evt model.SecurityEvent) {
	_, _ = s.store.AddSecurityEvent(evt)
}

func (s *Server) recordAuditEvent(r *http.Request, claims *auth.Claims, action, resource, resourceID string, details map[string]any) {
	if claims == nil {
		return
	}
	_, _ = s.store.AddAuditEvent(model.AuditEvent{
		ActorID:    claims.UserID,
		ActorRole:  claims.Role,
		Action:     strings.TrimSpace(action),
		Resource:   strings.TrimSpace(resource),
		ResourceID: strings.TrimSpace(resourceID),
		IP:         clientIP(r),
		Details:    details,
	})
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

func (s *Server) enrichPaymentCheckoutSession(pay model.Pagamento) (model.Pagamento, error) {
	provider := strings.ToLower(strings.TrimSpace(pay.Provider))
	if provider != "stripe" {
		return pay, nil
	}
	secret := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
	if secret == "" {
		return pay, nil
	}

	apiBase := strings.TrimSpace(os.Getenv("STRIPE_API_BASE"))
	if apiBase == "" {
		apiBase = "https://api.stripe.com"
	}
	successURL := strings.TrimSpace(os.Getenv("PAYMENT_SUCCESS_URL"))
	if successURL == "" {
		successURL = "https://example.com/conferma-pagamento"
	}
	cancelURL := strings.TrimSpace(os.Getenv("PAYMENT_CANCEL_URL"))
	if cancelURL == "" {
		cancelURL = "https://example.com/pagamento-annullato"
	}
	amountCents := int64(pay.Importo * 100)
	if amountCents <= 0 {
		amountCents = 100
	}

	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("client_reference_id", pay.Token)
	form.Set("metadata[token]", pay.Token)
	form.Set("line_items[0][price_data][currency]", strings.ToLower(strings.TrimSpace(pay.Valuta)))
	if strings.TrimSpace(pay.Valuta) == "" {
		form.Set("line_items[0][price_data][currency]", "eur")
	}
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(amountCents, 10))
	form.Set("line_items[0][price_data][product_data][name]", "Visto Easy pratica "+pay.PraticaID)
	form.Set("line_items[0][quantity]", "1")

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(apiBase, "/")+"/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return model.Pagamento{}, err
	}
	req.SetBasicAuth(secret, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return model.Pagamento{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return model.Pagamento{}, fmt.Errorf("stripe checkout create failed status=%d body=%s", resp.StatusCode, string(body))
	}
	var stripeResp struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stripeResp); err != nil {
		return model.Pagamento{}, err
	}
	if strings.TrimSpace(stripeResp.ID) == "" || strings.TrimSpace(stripeResp.URL) == "" {
		return model.Pagamento{}, errors.New("stripe response missing id/url")
	}

	updated, err := s.store.UpdatePaymentCheckout(pay.ID, stripeResp.ID, stripeResp.URL)
	if err != nil {
		return model.Pagamento{}, err
	}
	return updated, nil
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

func formatOptTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
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
