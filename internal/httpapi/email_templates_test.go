package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPreferredEmailLang(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,it;q=0.8")
	if got := preferredEmailLang(req); got != "en" {
		t.Fatalf("expected en, got %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "fr-FR,fr;q=0.9")
	if got := preferredEmailLang(req); got != "it" {
		t.Fatalf("expected fallback it, got %q", got)
	}
}

func TestAuthVerificationEmailLocalization(t *testing.T) {
	_, textBody, htmlBody := buildAuthVerificationEmail("en", "https://example.test/verify?token=abc", time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC))
	if !strings.Contains(textBody, "Welcome to Visto Easy.") || !strings.Contains(textBody, "Expires:") {
		t.Fatalf("expected english verification body, got %q", textBody)
	}
	if !strings.Contains(htmlBody, "activate account") {
		t.Fatalf("expected english verification html, got %q", htmlBody)
	}

	_, textBody, htmlBody = buildAuthVerificationEmail("it", "https://example.test/verify?token=abc", time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC))
	if !strings.Contains(textBody, "Benvenuto su Visto Easy.") || !strings.Contains(textBody, "Scade:") {
		t.Fatalf("expected italian verification body, got %q", textBody)
	}
	if !strings.Contains(htmlBody, "attiva account") {
		t.Fatalf("expected italian verification html, got %q", htmlBody)
	}
}

func TestAuthInviteEmailLocalization(t *testing.T) {
	_, textBody, htmlBody := buildAuthOperatorInviteEmail("en", "", "invite-token", time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC))
	if !strings.Contains(textBody, "Invitation token:") || !strings.Contains(htmlBody, "Invitation token:") {
		t.Fatalf("expected english invite body, got %q / %q", textBody, htmlBody)
	}

	_, textBody, htmlBody = buildAuthOperatorInviteEmail("it", "", "invite-token", time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC))
	if !strings.Contains(textBody, "Token invito:") || !strings.Contains(htmlBody, "Token invito:") {
		t.Fatalf("expected italian invite body, got %q / %q", textBody, htmlBody)
	}
}
