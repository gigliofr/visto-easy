package httpapi

import (
	"net/http"
	"strings"
	"time"
)

func preferredEmailLang(r *http.Request) string {
	if r == nil {
		return "it"
	}
	raw := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept-Language")))
	if raw == "" {
		return "it"
	}
	for _, part := range strings.Split(raw, ",") {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		if semi := strings.Index(tag, ";"); semi >= 0 {
			tag = tag[:semi]
		}
		if strings.HasPrefix(tag, "en") {
			return "en"
		}
		if strings.HasPrefix(tag, "it") {
			return "it"
		}
	}
	if strings.Contains(raw, "en") {
		return "en"
	}
	return "it"
}

func buildAuthVerificationEmail(lang, verifyURL string, expiresAt time.Time) (string, string, string) {
	if lang == "en" {
		return "Verify your Visto Easy email",
			"Welcome to Visto Easy.\nConfirm your account by clicking this link: " + verifyURL + "\nExpires: " + expiresAt.Format(time.RFC3339),
			"<p>Welcome to Visto Easy.</p><p>Confirm your account by clicking this link: <a href=\"" + verifyURL + "\">activate account</a></p><p>Expires: " + expiresAt.Format(time.RFC3339) + "</p>"
	}
	return "Verifica email Visto Easy",
		"Benvenuto su Visto Easy.\nConferma il tuo account cliccando il link: " + verifyURL + "\nScade: " + expiresAt.Format(time.RFC3339),
		"<p>Benvenuto su Visto Easy.</p><p>Conferma il tuo account cliccando il link: <a href=\"" + verifyURL + "\">attiva account</a></p><p>Scade: " + expiresAt.Format(time.RFC3339) + "</p>"
}

func buildAuthPasswordResetEmail(lang, resetURL string, expiresAt time.Time) (string, string, string) {
	if lang == "en" {
		return "Reset your Visto Easy password",
			"We received a password reset request.\nUse this link: " + resetURL + "\nExpires: " + expiresAt.Format(time.RFC3339),
			"<p>We received a password reset request.</p><p>Use this link: <a href=\"" + resetURL + "\">reset password</a></p><p>Expires: " + expiresAt.Format(time.RFC3339) + "</p>"
	}
	return "Reset password Visto Easy",
		"Abbiamo ricevuto una richiesta di reset password.\nUsa questo link: " + resetURL + "\nScade: " + expiresAt.Format(time.RFC3339),
		"<p>Abbiamo ricevuto una richiesta di reset password.</p><p>Usa questo link: <a href=\"" + resetURL + "\">reset password</a></p><p>Scadenza: " + expiresAt.Format(time.RFC3339) + "</p>"
}

func buildAuthOperatorInviteEmail(lang, inviteURL, inviteToken string, expiresAt time.Time) (string, string, string) {
	if lang == "en" {
		textBody := "You have been invited as an operator on Visto Easy."
		htmlBody := "<p>You have been invited as an operator on Visto Easy.</p>"
		if inviteURL != "" {
			textBody += "\nComplete activation here: " + inviteURL
			htmlBody += "<p>Complete activation here: <a href=\"" + inviteURL + "\">activate account</a></p>"
		} else {
			textBody += "\nInvitation token: " + inviteToken
			htmlBody += "<p>Invitation token: <strong>" + inviteToken + "</strong></p>"
		}
		textBody += "\nExpires: " + expiresAt.Format(time.RFC3339)
		htmlBody += "<p>Expires: " + expiresAt.Format(time.RFC3339) + "</p>"
		return "Visto Easy operator invitation", textBody, htmlBody
	}

	textBody := "Sei stato invitato come operatore su Visto Easy."
	htmlBody := "<p>Sei stato invitato come operatore su Visto Easy.</p>"
	if inviteURL != "" {
		textBody += "\nCompleta l'attivazione qui: " + inviteURL
		htmlBody += "<p>Completa l'attivazione qui: <a href=\"" + inviteURL + "\">attiva account</a></p>"
	} else {
		textBody += "\nToken invito: " + inviteToken
		htmlBody += "<p>Token invito: <strong>" + inviteToken + "</strong></p>"
	}
	textBody += "\nScadenza: " + expiresAt.Format(time.RFC3339)
	htmlBody += "<p>Scadenza: " + expiresAt.Format(time.RFC3339) + "</p>"
	return "Invito operatore Visto Easy", textBody, htmlBody
}
