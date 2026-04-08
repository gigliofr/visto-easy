package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
	"visto-easy/internal/store"
)

type Server struct {
	store  store.DataStore
	tokens *auth.TokenManager
}

func NewServer(st store.DataStore, tm *auth.TokenManager) *Server {
	return &Server{store: st, tokens: tm}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(requestJSON)

	r.Get("/", s.handleRoot)
	r.Get("/api/v1/health", s.handleHealth)

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", s.handleRegister)
		r.Post("/login", s.handleLogin)
		r.Post("/refresh", s.handleRefresh)
		r.Post("/logout", s.handleLogout)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware())
		r.Route("/api/pratiche", func(r chi.Router) {
			r.Post("/", s.handleCreatePratica)
			r.Get("/", s.handleListMiePratiche)
			r.Get("/{id}", s.handleGetPratica)
			r.Patch("/{id}", s.handlePatchPratica)
			r.Delete("/{id}", s.handleDeletePratica)
			r.Post("/{id}/documenti", s.handleAddDocumento)
			r.Get("/{id}/documenti", s.handleListDocumenti)
		})

		r.Route("/api/pagamento", func(r chi.Router) {
			r.Get("/{token}", s.handleGetPagamento)
			r.Get("/{token}/stato", s.handleGetPagamento)
			r.Post("/crea-sessione", s.requireRoles(model.RoleOperatore, model.RoleSupervisore, model.RoleAdmin)(http.HandlerFunc(s.handleCreatePagamentoSessione)).ServeHTTP)
			r.Post("/webhook", s.handlePagamentoWebhook)
		})

		r.Route("/api/bo", func(r chi.Router) {
			r.Use(s.requireRoles(model.RoleOperatore, model.RoleSupervisore, model.RoleAdmin))
			r.Get("/pratiche", s.handleBOListPratiche)
			r.Get("/pratiche/{id}", s.handleGetPratica)
			r.Patch("/pratiche/{id}/stato", s.handleBOChangeStato)
			r.Post("/pratiche/{id}/assegna", s.handleBOAssegna)
			r.Post("/pratiche/{id}/link-pagamento", s.handleBOCreateLinkPagamento)
			r.Post("/pratiche/{id}/invia-visto", s.handleBOInviaVisto)
			r.Get("/dashboard/stats", s.handleBOStats)
		})
	})

	return r
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
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

func (s *Server) handleListDocumenti(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	docs, err := s.store.ListDocumenti(id)
	if err != nil { writeErr(w, http.StatusNotFound, "pratica non trovata"); return }
	writeJSON(w, http.StatusOK, docs)
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
	var req struct { Token, Event string }
	if !decodeJSON(w, r, &req) { return }
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

func (s *Server) handleBOListPratiche(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, s.store.ListAllPratiche()) }

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
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "assegnazione registrata"})
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
