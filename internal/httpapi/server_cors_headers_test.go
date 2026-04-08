package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSPreflightAllowedOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	s, _, _ := newSecurityHTTPTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization,Content-Type")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for preflight, got=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("unexpected allow-origin: %s", got)
	}
}

func TestCORSPreflightDisallowedOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	s, _, _ := newSecurityHTTPTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed origin, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSecurityHeadersApplied(t *testing.T) {
	s, _, _ := newSecurityHTTPTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing X-Content-Type-Options")
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing X-Frame-Options")
	}
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("missing Content-Security-Policy")
	}
}
