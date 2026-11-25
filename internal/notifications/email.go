// Package notifications fornisce l'invio email di visto-easy tramite
// github.com/gigliofr/gokit/mailx (Resend o SMTP, scelti dall'ambiente).
package notifications

import (
	"log"

	"github.com/gigliofr/gokit/mailx"
)

// EmailSender è compatibile con mailx.Sender.
type EmailSender = mailx.Sender

// NewEmailSenderFromEnv usa Resend se RESEND_API_KEY è impostata, altrimenti
// SMTP (SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_KEY, MAIL_FROM); senza
// configurazione le email non vengono inviate.
func NewEmailSenderFromEnv() EmailSender {
	s, why := mailx.FromEnv(mailx.Options{DefaultSubject: "Notifica Visto Easy"})
	log.Printf("notifications: %s", why)
	return s
}
