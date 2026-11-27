package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/gigliofr/gokit/httpx"
	"github.com/gigliofr/gokit/storagex/s3"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
	storagepkg "visto-easy/internal/storage"
	"visto-easy/internal/store"
)

// maxMessaggioRichiedente limita la lunghezza dei messaggi inviati dal richiedente.
const maxMessaggioRichiedente = 2000

// isEventoInterno indica se un evento della pratica è riservato al backoffice.
func isEventoInterno(e model.EventoPratica) bool {
	if e.Metadata == nil {
		return false
	}
	if v, ok := e.Metadata["scope"].(string); ok && strings.EqualFold(v, "interna") {
		return true
	}
	if v, ok := e.Metadata["interna"].(bool); ok && v {
		return true
	}
	return false
}

// eventiVisibiliRichiedente restituisce solo gli eventi che il richiedente può vedere.
func eventiVisibiliRichiedente(in []model.EventoPratica) []model.EventoPratica {
	out := make([]model.EventoPratica, 0, len(in))
	for _, e := range in {
		if !isEventoInterno(e) {
			out = append(out, e)
		}
	}
	return out
}

// praticaPerRichiedente rimuove dalla pratica i dati riservati al backoffice.
func praticaPerRichiedente(p model.Pratica) model.Pratica {
	p.NoteInterne = ""
	p.Eventi = eventiVisibiliRichiedente(p.Eventi)
	return p
}

// vistaPratica adatta la pratica al ruolo di chi la richiede.
func vistaPratica(claims *auth.Claims, p model.Pratica) model.Pratica {
	if claims != nil && claims.Role == string(model.RoleRichiedente) {
		return praticaPerRichiedente(p)
	}
	return p
}

func vistaPratiche(claims *auth.Claims, in []model.Pratica) []model.Pratica {
	if claims == nil || claims.Role != string(model.RoleRichiedente) {
		return in
	}
	out := make([]model.Pratica, 0, len(in))
	for _, p := range in {
		out = append(out, praticaPerRichiedente(p))
	}
	return out
}

// handleRichiedenteMessaggio permette al richiedente di scrivere all'operatore
// sulla propria pratica. Il messaggio è visibile a entrambi.
func (s *Server) handleRichiedenteMessaggio(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r.Context())
	if claims == nil || claims.Role != string(model.RoleRichiedente) {
		httpx.Error(w, http.StatusForbidden, "solo richiedente")
		return
	}
	id := chi.URLParam(r, "id")
	p, err := s.store.GetPratica(id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "pratica non trovata")
		return
	}
	if p.UtenteID != claims.UserID {
		httpx.Error(w, http.StatusForbidden, "accesso negato")
		return
	}
	var req struct {
		Messaggio string `json:"messaggio"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	msg := strings.TrimSpace(req.Messaggio)
	if msg == "" {
		httpx.Error(w, http.StatusBadRequest, "messaggio vuoto")
		return
	}
	if len([]rune(msg)) > maxMessaggioRichiedente {
		httpx.Error(w, http.StatusBadRequest, "messaggio troppo lungo")
		return
	}
	updated, err := s.store.AddNota(id, claims.UserID, msg, false)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "pratica non trovata")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "messaggio non valido")
		return
	}
	s.recordAuditEvent(r, claims, "PRATICA_MESSAGGIO_RICHIEDENTE", "pratica", id, nil)
	httpx.JSON(w, http.StatusCreated, praticaPerRichiedente(updated))
}

// isDocumentKeyForPratica verifica che una chiave di storage restituita dal
// presign appartenga alla cartella documenti della pratica indicata.
func isDocumentKeyForPratica(key, praticaID string) bool {
	if strings.Contains(key, "..") || strings.Contains(key, "\\") {
		return false
	}
	prefix := s3.Key("pratiche", praticaID, "documenti") + "/"
	return strings.HasPrefix(key, prefix) && len(key) > len(prefix)
}

// writeDownloadSession reindirizza al link firmato oppure, con ?format=json,
// lo restituisce in JSON: il frontend lo apre senza dipendere dal redirect,
// che non può portarsi dietro l'header Authorization.
func writeDownloadSession(w http.ResponseWriter, r *http.Request, session storagepkg.DownloadSession) {
	if strings.EqualFold(r.URL.Query().Get("format"), "json") {
		httpx.JSON(w, http.StatusOK, map[string]any{"url": session.DownloadURL, "expires_at": session.ExpiresAt})
		return
	}
	http.Redirect(w, r, session.DownloadURL, http.StatusFound)
}
