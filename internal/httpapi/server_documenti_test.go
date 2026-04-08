package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
)

func newDocumentTestServer(t *testing.T) (*Server, *fakePolicyStore, *auth.TokenManager) {
	t.Helper()
	st := &fakePolicyStore{users: map[string]model.Utente{}, usersByEmail: map[string]string{}, pratiche: map[string]model.Pratica{}}
	tm, err := auth.NewTokenManager("this-is-a-long-test-secret-with-32-plus")
	if err != nil {
		t.Fatalf("token manager init failed: %v", err)
	}
	s := NewServer(st, tm, nil)
	return s, st, tm
}

func seedPraticaWithDocumento(t *testing.T, st *fakePolicyStore) (model.Utente, model.Utente, model.Pratica, model.Documento) {
	t.Helper()
	u1, err := st.CreateUser(model.Utente{Email: "owner@example.com", PasswordHash: "x", Ruolo: model.RoleRichiedente, Nome: "Owner", Cognome: "One"})
	if err != nil {
		t.Fatalf("create user1 failed: %v", err)
	}
	u2, err := st.CreateUser(model.Utente{Email: "other@example.com", PasswordHash: "x", Ruolo: model.RoleRichiedente, Nome: "Other", Cognome: "Two"})
	if err != nil {
		t.Fatalf("create user2 failed: %v", err)
	}

	p, err := st.CreatePratica(model.Pratica{UtenteID: u1.ID, TipoVisto: "TURISMO", PaeseDest: "JP"}, u1.ID)
	if err != nil {
		t.Fatalf("create pratica failed: %v", err)
	}
	d, err := st.AddDocumento(p.ID, model.Documento{Tipo: "PASSAPORTO", NomeFile: "pass.pdf", MimeType: "application/pdf", Dimensione: 1024})
	if err != nil {
		t.Fatalf("add documento failed: %v", err)
	}
	return u1, u2, p, d
}

func TestGetDocumentoOwnerCanRead(t *testing.T) {
	s, st, tm := newDocumentTestServer(t)
	u1, _, p, d := seedPraticaWithDocumento(t, st)
	tok, err := tm.SignAccess(u1.ID, string(model.RoleRichiedente))
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/pratiche/"+p.ID+"/documenti/"+d.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetDocumentoOtherUserForbidden(t *testing.T) {
	s, st, tm := newDocumentTestServer(t)
	_, u2, p, d := seedPraticaWithDocumento(t, st)
	tok, err := tm.SignAccess(u2.ID, string(model.RoleRichiedente))
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/pratiche/"+p.ID+"/documenti/"+d.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDeleteDocumentoAllowedOnlyInBozzaForOwner(t *testing.T) {
	s, st, tm := newDocumentTestServer(t)
	u1, _, p, d := seedPraticaWithDocumento(t, st)
	tok, err := tm.SignAccess(u1.ID, string(model.RoleRichiedente))
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}

	reqDelete := httptest.NewRequest(http.MethodDelete, "/api/pratiche/"+p.ID+"/documenti/"+d.ID, nil)
	reqDelete.Header.Set("Authorization", "Bearer "+tok)
	rrDelete := httptest.NewRecorder()
	s.Router().ServeHTTP(rrDelete, reqDelete)
	if rrDelete.Code != http.StatusNoContent {
		t.Fatalf("expected 204 in bozza, got=%d body=%s", rrDelete.Code, rrDelete.Body.String())
	}

	d2, err := st.AddDocumento(p.ID, model.Documento{Tipo: "PASSAPORTO", NomeFile: "pass2.pdf", MimeType: "application/pdf", Dimensione: 1024})
	if err != nil {
		t.Fatalf("re-add documento failed: %v", err)
	}
	if _, err := st.SubmitPratica(p.ID, u1.ID); err != nil {
		t.Fatalf("submit pratica failed: %v", err)
	}

	reqDeleteAfterSubmit := httptest.NewRequest(http.MethodDelete, "/api/pratiche/"+p.ID+"/documenti/"+d2.ID, nil)
	reqDeleteAfterSubmit.Header.Set("Authorization", "Bearer "+tok)
	rrDeleteAfterSubmit := httptest.NewRecorder()
	s.Router().ServeHTTP(rrDeleteAfterSubmit, reqDeleteAfterSubmit)
	if rrDeleteAfterSubmit.Code != http.StatusForbidden {
		t.Fatalf("expected 403 after submit, got=%d body=%s", rrDeleteAfterSubmit.Code, rrDeleteAfterSubmit.Body.String())
	}
}
