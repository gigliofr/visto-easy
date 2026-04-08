package notifications

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type EmailSender interface {
	Send(to, subject, textBody, htmlBody string) error
}

type noopSender struct{}

func (n noopSender) Send(_, _, _, _ string) error { return nil }

type sendGridSender struct {
	apiKey   string
	fromEmail string
	fromName string
	apiBase  string
	client   *http.Client
}

func NewEmailSenderFromEnv() EmailSender {
	apiKey := strings.TrimSpace(os.Getenv("SENDGRID_API_KEY"))
	fromEmail := strings.TrimSpace(os.Getenv("SENDGRID_FROM_EMAIL"))
	fromName := strings.TrimSpace(os.Getenv("SENDGRID_FROM_NAME"))
	if apiKey == "" || fromEmail == "" {
		return noopSender{}
	}
	apiBase := strings.TrimSpace(os.Getenv("SENDGRID_API_BASE"))
	if apiBase == "" {
		apiBase = "https://api.sendgrid.com"
	}
	return &sendGridSender{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
		apiBase:   strings.TrimRight(apiBase, "/"),
		client:    &http.Client{Timeout: 12 * time.Second},
	}
}

func (s *sendGridSender) Send(to, subject, textBody, htmlBody string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return errors.New("recipient email required")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "Notifica Visto Easy"
	}
	if strings.TrimSpace(textBody) == "" && strings.TrimSpace(htmlBody) == "" {
		return errors.New("email body is empty")
	}
	if strings.TrimSpace(textBody) == "" {
		textBody = stripHTMLBasic(htmlBody)
	}
	if strings.TrimSpace(htmlBody) == "" {
		htmlBody = "<p>" + textBody + "</p>"
	}

	payload := map[string]any{
		"personalizations": []any{map[string]any{"to": []any{map[string]any{"email": to}}}},
		"from": map[string]any{"email": s.fromEmail, "name": s.fromName},
		"subject": subject,
		"content": []any{
			map[string]any{"type": "text/plain", "value": textBody},
			map[string]any{"type": "text/html", "value": htmlBody},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, s.apiBase+"/v3/mail/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sendgrid send failed status=%d", resp.StatusCode)
	}
	return nil
}

func stripHTMLBasic(v string) string {
	repl := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "<p>", "", "</p>", "\n")
	out := repl.Replace(v)
	out = strings.ReplaceAll(out, "<", "")
	out = strings.ReplaceAll(out, ">", "")
	return strings.TrimSpace(out)
}
