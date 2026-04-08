package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
	storagepkg "visto-easy/internal/storage"
	"visto-easy/internal/store"
)

type Server struct {
	store  store.DataStore
	tokens *auth.TokenManager
	presign storagepkg.PresignService
}

func NewServer(st store.DataStore, tm *auth.TokenManager, presign storagepkg.PresignService) *Server {
	return &Server{store: st, tokens: tm, presign: presign}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(requestJSON)

	r.Get("/", s.handleRoot)
	r.Get("/api/v1/health", s.handleHealth)
	r.Post("/api/pagamento/webhook", s.handlePagamentoWebhook)

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", s.handleRegister)
		r.Post("/login", s.handleLogin)
		r.Post("/refresh", s.handleRefresh)
		r.Post("/logout", s.handleLogout)
		r.Post("/forgot-password", s.handleForgotPassword)
		r.Post("/reset-password", s.handleResetPassword)
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
			r.Get("/pratiche", s.handleBOListPratiche)
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
	u, err := s.store.GetUserByEmail(req.Email)
	if err != nil || u.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		writeErr(w, http.StatusUnauthorized, "credenziali non valide")
		return
	}
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

	var req struct { Token, Event string }
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "payload non valido")
		return
	}
	if strings.ToLower(req.Event) != "payment.succeeded" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ignored"})
		return
	}
	pay, err := s.store.ConfirmPaymentByToken(req.Token)
	if err != nil { writeErr(w, http.StatusNotFound, "pagamento non trovato"); return }
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoPagamentoRicevuto, "webhook provider")
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoVistoInElaborazione, "generazione visto")
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoVistoEmesso, "visto emesso")
	_, _ = s.store.ChangePraticaState(pay.PraticaID, "system", model.StatoCompletata, "visto consegnato")
	writeJSON(w, http.StatusOK, map[string]any{"status": "processed"})
}

func (s *Server) handleBOListPratiche(w http.ResponseWriter, r *http.Request) {
	all := s.store.ListAllPratiche()
	filtered := make([]map[string]any, 0, len(all))

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
		filtered = append(filtered, map[string]any{
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
		"items": filtered,
		"count": len(filtered),
	})
}

func (s *Server) handleBOListUtenti(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
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

func verifyStripeSignature(secret string, payload []byte, signatureHeader string) bool {
	if strings.TrimSpace(signatureHeader) == "" {
		return false
	}
	parts := strings.Split(signatureHeader, ",")
	v1 := ""
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "v1" {
			v1 = kv[1]
			break
		}
	}
	if v1 == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	provided, err := hexDecode(v1)
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

func hexDecode(v string) ([]byte, error) {
	const hexdigits = "0123456789abcdef"
	v = strings.ToLower(strings.TrimSpace(v))
	if len(v)%2 != 0 {
		return nil, errors.New("invalid hex")
	}
	out := make([]byte, len(v)/2)
	for i := 0; i < len(v); i += 2 {
		hi := strings.IndexByte(hexdigits, v[i])
		lo := strings.IndexByte(hexdigits, v[i+1])
		if hi < 0 || lo < 0 {
			return nil, errors.New("invalid hex")
		}
		out[i/2] = byte((hi << 4) | lo)
	}
	return out, nil
}
