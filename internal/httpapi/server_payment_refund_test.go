package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"visto-easy/internal/model"
)

func TestBORefundPagamentoEndpoint(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	u, err := st.CreateUser(model.Utente{Email: "refund-owner@example.com", PasswordHash: "x", Ruolo: model.RoleRichiedente, Nome: "R", Cognome: "O"})
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	p, err := st.CreatePratica(model.Pratica{UtenteID: u.ID, TipoVisto: "TURISMO", PaeseDest: "JP"}, u.ID)
	if err != nil {
		t.Fatalf("create pratica failed: %v", err)
	}
	pay, err := st.CreatePayment(p.ID, "stripe", 120)
	if err != nil {
		t.Fatalf("create payment failed: %v", err)
	}
	if _, err := st.ConfirmPaymentByToken(pay.Token); err != nil {
		t.Fatalf("confirm payment failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/bo/pagamenti/"+pay.Token+"/rimborso", strings.NewReader(`{"amount":20,"reason":"manual adjustment"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}

	refunded, err := st.GetPaymentByToken(pay.Token)
	if err != nil {
		t.Fatalf("get refunded payment failed: %v", err)
	}
	if refunded.Stato != model.PagamentoRimborsato {
		t.Fatalf("expected refunded status, got=%s", refunded.Stato)
	}

	foundRefundEvent := false
	for _, evt := range st.ListSecurityEvents() {
		if evt.Type == "PAYMENT_REFUNDED" {
			foundRefundEvent = true
			break
		}
	}
	if !foundRefundEvent {
		t.Fatalf("expected PAYMENT_REFUNDED audit event")
	}
}

func TestBORefundPagamentoConflictWhenNotCompleted(t *testing.T) {
	s, st, token := newSecurityHTTPTestServer(t)
	u, err := st.CreateUser(model.Utente{Email: "refund-pending@example.com", PasswordHash: "x", Ruolo: model.RoleRichiedente, Nome: "R", Cognome: "P"})
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	p, err := st.CreatePratica(model.Pratica{UtenteID: u.ID, TipoVisto: "STUDIO", PaeseDest: "FR"}, u.ID)
	if err != nil {
		t.Fatalf("create pratica failed: %v", err)
	}
	pay, err := st.CreatePayment(p.ID, "stripe", 80)
	if err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/bo/pagamenti/"+pay.Token+"/rimborso", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for non-completed payment, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

