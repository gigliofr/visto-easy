package notifications

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"
)

type EmailSender interface {
	Send(to, subject, textBody, htmlBody string) error
}

type noopSender struct{}

func (n noopSender) Send(_, _, _, _ string) error { return nil }

type smtpSender struct {
	host     string
	port     int
	user     string
	key      string
	from     string
}

func NewEmailSenderFromEnv() EmailSender {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	portRaw := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	user := strings.TrimSpace(os.Getenv("SMTP_USER"))
	key := strings.TrimSpace(os.Getenv("SMTP_KEY"))
	from := strings.TrimSpace(os.Getenv("MAIL_FROM"))
	if host == "" || portRaw == "" || user == "" || key == "" || from == "" {
		return noopSender{}
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port <= 0 {
		return noopSender{}
	}
	return &smtpSender{
		host: host,
		port: port,
		user: user,
		key:  key,
		from: from,
	}
}

func (s *smtpSender) Send(to, subject, textBody, htmlBody string) error {
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
	var body bytes.Buffer
	boundary := "visto-easy-alt-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	body.WriteString("From: " + s.from + "\r\n")
	body.WriteString("To: " + to + "\r\n")
	body.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n")
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n")
	body.WriteString("\r\n")
	writer := multipart.NewWriter(&body)
	if err := writer.SetBoundary(boundary); err != nil {
		return err
	}
	plainHeader := textproto.MIMEHeader{}
	plainHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	plainHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	plainPart, err := writer.CreatePart(plainHeader)
	if err != nil {
		return err
	}
	plainQP := quotedprintable.NewWriter(plainPart)
	if _, err := plainQP.Write([]byte(textBody)); err != nil {
		_ = plainQP.Close()
		return err
	}
	if err := plainQP.Close(); err != nil {
		return err
	}
	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := writer.CreatePart(htmlHeader)
	if err != nil {
		return err
	}
	htmlQP := quotedprintable.NewWriter(htmlPart)
	if _, err := htmlQP.Write([]byte(htmlBody)); err != nil {
		_ = htmlQP.Close()
		return err
	}
	if err := htmlQP.Close(); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.key, s.host)
	if err := smtp.SendMail(addr, auth, s.from, []string{to}, body.Bytes()); err != nil {
		return fmt.Errorf("smtp send failed: %w", err)
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
