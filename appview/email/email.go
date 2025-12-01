package email

import (
	"fmt"
	"net"
	"net/mail"
	"strings"

	"github.com/resend/resend-go/v2"
)

type Email struct {
	From    string
	To      string
	Subject string
	Text    string
	Html    string
	APIKey  string
}

func SendEmail(email Email) error {
	client := resend.NewClient(email.APIKey)
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    email.From,
		To:      []string{email.To},
		Subject: email.Subject,
		Text:    email.Text,
		Html:    email.Html,
	})
	if err != nil {
		return fmt.Errorf("error sending email: %w", err)
	}
	return nil
}

func IsValidEmail(email string) bool {
	// Reject whitespace (ParseAddress normalizes it away)
	if strings.ContainsAny(email, " \t\n\r") {
		return false
	}

	// Use stdlib RFC 5322 parser
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}

	// Split email into local and domain parts
	parts := strings.Split(addr.Address, "@")
	domain := parts[1]

	mx, err := net.LookupMX(domain)
	if err != nil || len(mx) == 0 {
		return false
	}

	return true
}
