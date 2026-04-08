package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootServesFrontendHTML(t *testing.T) {
	s, _, _ := newSecurityHTTPTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(strings.ToLower(ct), "text/html") {
		t.Fatalf("expected text/html content type, got=%q", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Frontend completo") {
		t.Fatalf("expected frontend marker in body")
	}
	if !strings.Contains(body, "id=\"formLogin\"") {
		t.Fatalf("expected auth form in served html")
	}
}

func TestAssetsAreServed(t *testing.T) {
	s, _, _ := newSecurityHTTPTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status for app.js: got=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Frontend inizializzato") {
		t.Fatalf("expected app.js content")
	}
}

func TestIndexHTMLRouteServed(t *testing.T) {
	s, _, _ := newSecurityHTTPTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status for /index.html: got=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Frontend completo") {
		t.Fatalf("expected frontend html body")
	}
}

func TestNonAPINotFoundFallsBackToFrontend(t *testing.T) {
	s, _, _ := newSecurityHTTPTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/app/dashboard", nil)
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status for frontend fallback: got=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "id=\"appOutput\"") {
		t.Fatalf("expected SPA shell for fallback route")
	}
}
