package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"visto-easy/internal/auth"
	"visto-easy/internal/model"
)

func newPaymentTestServer(t *testing.T) (*Server, *fakePolicyStore, string) {
	t.Helper()
	st := &fakePolicyStore{
		users:               map[string]model.Utente{},
		usersByEmail:        map[string]string{},
		pratiche:            map[string]model.Pratica{},
		refreshSessions:     map[string]model.RefreshSession{},
		passwordResetTokens: map[string]model.PasswordResetToken{},
		payments:            map[string]model.Pagamento{},
	}
	tm, err := auth.NewTokenManager("this-is-a-long-test-secret-with-32-plus")
	if err != nil {
		t.Fatalf("token manager init failed: %v", err)
	}
	tok, err := tm.SignAccess("bo-admin-1", string(model.RoleAdmin))
	if err != nil {
		t.Fatalf("access token sign failed: %v", err)
	}
	s := &Server{store: st, tokens: tm, authRL: newSimpleRateLimiter(1000, time.Minute), loginLT: newLoginLockTracker(10, 15*time.Minute)}
	return s, st, tok
}

func TestCreatePagamentoSessioneFallsBackWithoutStripeSecret(t *testing.T) {
	s, st, token := newPaymentTestServer(t)
	p, err := st.CreatePratica(model.Pratica{UtenteID: "usr-1", TipoVisto: "TURISMO", PaeseDest: "JP"}, "usr-1")
	if err != nil {
		t.Fatalf("create pratica failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/pagamento/crea-sessione", strings.NewReader(`{"praticaid":"`+p.ID+`","provider":"stripe","importo":99.5}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}
	var pay model.Pagamento
	if err := json.Unmarshal(rr.Body.Bytes(), &pay); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if !strings.HasPrefix(pay.LinkPagamento, "/pagamento/") {
		t.Fatalf("expected local fallback payment link, got=%s", pay.LinkPagamento)
	}
}

func TestCreatePagamentoSessioneUsesStripeWhenConfigured(t *testing.T) {
	s, st, token := newPaymentTestServer(t)
	p, err := st.CreatePratica(model.Pratica{UtenteID: "usr-2", TipoVisto: "LAVORO", PaeseDest: "US"}, "usr-2")
	if err != nil {
		t.Fatalf("create pratica failed: %v", err)
	}

	var gotAuth string
	var gotForm url.Values
	stripeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkout/sessions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form failed: %v", err)
		}
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cs_test_123","url":"https://checkout.stripe.com/c/pay/cs_test_123"}`))
	}))
	defer stripeSrv.Close()

	t.Setenv("STRIPE_SECRET_KEY", "sk_test_abc")
	t.Setenv("STRIPE_API_BASE", stripeSrv.URL)
	t.Setenv("PAYMENT_SUCCESS_URL", "https://app.example.com/conferma")
	t.Setenv("PAYMENT_CANCEL_URL", "https://app.example.com/annulla")

	req := httptest.NewRequest(http.MethodPost, "/api/pagamento/crea-sessione", strings.NewReader(`{"praticaid":"`+p.ID+`","provider":"stripe","importo":120}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}
	var pay model.Pagamento
	if err := json.Unmarshal(rr.Body.Bytes(), &pay); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if pay.ProviderSessionID != "cs_test_123" {
		t.Fatalf("expected provider session id updated, got=%s", pay.ProviderSessionID)
	}
	if pay.LinkPagamento != "https://checkout.stripe.com/c/pay/cs_test_123" {
		t.Fatalf("expected provider checkout url, got=%s", pay.LinkPagamento)
	}
	if gotAuth == "" || !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("expected basic auth to stripe, got=%s", gotAuth)
	}
	if gotForm.Get("metadata[token]") == "" {
		t.Fatalf("expected metadata[token] in stripe payload")
	}
	if gotForm.Get("success_url") != "https://app.example.com/conferma" {
		t.Fatalf("unexpected success_url: %s", gotForm.Get("success_url"))
	}
}
