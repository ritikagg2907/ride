package mailer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"

	"github.com/ride-hailing/shared/pkg/env"
)

type Mail struct {
	To      string
	Subject string
	Body    string // HTML
}

func Send(m Mail) error {
	if key := env.Get("BREVO_API_KEY", ""); key != "" {
		return sendBrevo(m, key)
	}
	return sendSMTP(m)
}

func sendBrevo(m Mail, apiKey string) error {
	payload := map[string]any{
		"sender":  map[string]string{"name": "Ride Hailing", "email": env.Get("FROM_EMAIL", "noreply@ridehailing.dev")},
		"to":      []map[string]string{{"email": m.To}},
		"subject": m.Subject,
		"htmlContent": m.Body,
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewReader(b))
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("brevo returned %d", resp.StatusCode)
	}
	return nil
}

func sendSMTP(m Mail) error {
	host := env.Get("EMAIL_HOST", "smtp.gmail.com")
	port := env.Get("EMAIL_PORT", "587")
	user := env.Get("EMAIL_USER", "")
	pass := env.Get("EMAIL_PASS", "")

	auth := smtp.PlainAuth("", user, pass, host)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html\r\n\r\n%s",
		user, m.To, m.Subject, m.Body)
	return smtp.SendMail(host+":"+port, auth, user, []string{m.To}, []byte(msg))
}
