package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
	"visto-easy/internal/store"
)

func newRichiedenteViewServer(t *testing.T) (*Server, *store.MemoryStore, *auth.TokenManager, model.Utente, model.Utente, model.Pratica) {
	t.Helper()
	st := store.NewMemoryStore()
	tm, err := auth.NewTokenManager("this-is-a-long-test-secret-with-32-plus")
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	owner, err := st.CreateUser(model.Utente{Email: "owner-view@example.com", PasswordHash: "x", Ruolo: model.RoleRichiedente, Attivo: true, EmailVerificata: true})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	op, err := st.CreateUser(model.Utente{Email: "op-view@example.com", PasswordHash: "x", Ruolo: model.RoleOperatore, Attivo: true, EmailVerificata: true})
	if err != nil {
		t.Fatalf("create operatore: %v", err)
	}
	p, err := st.CreatePratica(model.Pratica{UtenteID: owner.ID, TipoVisto: "TURISMO", PaeseDest: "JP"}, owner.ID)
	if err != nil {
		t.Fatalf("create pratica: %v", err)
	}
	if _, err := st.AddNota(p.ID, op.ID, "SEGRETO: controllare il garante", true); err != nil {
		t.Fatalf("nota interna: %v", err)
	}
	if _, err := st.AddNota(p.ID, op.ID, "Manca la foto tessera", false); err != nil {
		t.Fatalf("nota richiedente: %v", err)
	}
	return NewServer(st, tm, nil), st, tm, owner, op, p
}

func doAuthed(t *testing.T, s *Server, tm *auth.TokenManager, u model.Utente, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	tok, err := tm.SignAccess(u.ID, string(u.Ruolo))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	return rr
}

func TestRichiedenteNonVedeNoteInterne(t *testing.T) {
	s, _, tm, owner, _, p := newRichiedenteViewServer(t)
	for _, path := range []string{"/api/pratiche/" + p.ID, "/api/pratiche/", "/api/pratiche/" + p.ID + "/eventi"} {
		rr := doAuthed(t, s, tm, owner, http.MethodGet, path, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status %d body=%s", path, rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		if strings.Contains(body, "SEGRETO") || strings.Contains(body, "note_interne") {
			t.Fatalf("%s: dati interni esposti al richiedente: %s", path, body)
		}
		if !strings.Contains(body, "Manca la foto tessera") {
			t.Fatalf("%s: atteso il messaggio per il richiedente: %s", path, body)
		}
	}
}

func TestBackofficeVedeNoteInterne(t *testing.T) {
	s, _, tm, _, op, p := newRichiedenteViewServer(t)
	rr := doAuthed(t, s, tm, op, http.MethodGet, "/api/bo/pratiche/"+p.ID, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "SEGRETO") {
		t.Fatalf("l'operatore deve vedere le note interne")
	}
}

func TestRichiedenteMessaggio(t *testing.T) {
	s, _, tm, owner, op, p := newRichiedenteViewServer(t)
	rr := doAuthed(t, s, tm, owner, http.MethodPost, "/api/pratiche/"+p.ID+"/messaggi", `{"messaggio":"Ho caricato la foto"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}
	var got model.Pratica
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.NoteInterne != "" {
		t.Fatalf("note interne nella risposta")
	}
	last := got.Eventi[len(got.Eventi)-1]
	if last.Messaggio != "Ho caricato la foto" || last.AttoreID != owner.ID {
		t.Fatalf("evento inatteso: %+v", last)
	}

	if rr := doAuthed(t, s, tm, owner, http.MethodPost, "/api/pratiche/"+p.ID+"/messaggi", `{"messaggio":"   "}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("messaggio vuoto: atteso 400, got %d", rr.Code)
	}
	if rr := doAuthed(t, s, tm, op, http.MethodPost, "/api/pratiche/"+p.ID+"/messaggi", `{"messaggio":"ciao"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("operatore: atteso 403, got %d", rr.Code)
	}
}

func TestRichiedenteMessaggioAltraPratica(t *testing.T) {
	s, st, tm, _, _, p := newRichiedenteViewServer(t)
	other, err := st.CreateUser(model.Utente{Email: "intruso@example.com", PasswordHash: "x", Ruolo: model.RoleRichiedente, Attivo: true, EmailVerificata: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rr := doAuthed(t, s, tm, other, http.MethodPost, "/api/pratiche/"+p.ID+"/messaggi", `{"messaggio":"ciao"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("atteso 403, got %d", rr.Code)
	}
	if rr := doAuthed(t, s, tm, other, http.MethodGet, "/api/pratiche/"+p.ID+"/eventi", ""); rr.Code != http.StatusForbidden {
		t.Fatalf("eventi altrui: atteso 403, got %d", rr.Code)
	}
}

func TestIsDocumentKeyForPratica(t *testing.T) {
	cases := map[string]bool{
		"pratiche/abc/documenti/uuid_pass.pdf": true,
		"pratiche/abc/documenti/":              false,
		"pratiche/xyz/documenti/uuid_pass.pdf": false,
		"pratiche/abc/documenti/../x.pdf":      false,
		"altro/abc/documenti/x.pdf":            false,
	}
	for key, want := range cases {
		if got := isDocumentKeyForPratica(key, "abc"); got != want {
			t.Errorf("%q: got %v want %v", key, got, want)
		}
	}
}

func TestAddDocumentoConservaChiavePresign(t *testing.T) {
	s, _, tm, owner, _, p := newRichiedenteViewServer(t)
	key := "pratiche/" + p.ID + "/documenti/u1_passaporto.pdf"
	rr := doAuthed(t, s, tm, owner, http.MethodPost, "/api/pratiche/"+p.ID+"/documenti",
		`{"tipo":"PASSAPORTO","nome_file":"passaporto.pdf","mime_type":"application/pdf","dimensione":1000,"s3_key":"`+key+`"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), key) {
		t.Fatalf("chiave non conservata: %s", rr.Body.String())
	}
	rr = doAuthed(t, s, tm, owner, http.MethodPost, "/api/pratiche/"+p.ID+"/documenti",
		`{"tipo":"PASSAPORTO","nome_file":"x.pdf","mime_type":"application/pdf","dimensione":1000,"s3_key":"pratiche/altra/documenti/x.pdf"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("chiave di altra pratica: atteso 400, got %d", rr.Code)
	}
}

func TestDownloadDocumentoFormatoJSON(t *testing.T) {
	s, st, tm := newDocumentTestServer(t)
	s.presign = fakePresignService{downloadURL: "https://s3.example/firmato"}
	u1, _, p, d := seedPraticaWithDocumento(t, st)
	rr := doAuthed(t, s, tm, u1, http.MethodGet, "/api/pratiche/"+p.ID+"/documenti/"+d.ID+"/download?format=json", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "https://s3.example/firmato") {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}
}
