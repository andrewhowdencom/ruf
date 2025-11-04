package email

import (
	"fmt"
	"net/smtp"

	"github.com/andrewhowdencom/ruf/internal/model"
)

// Client is an interface for sending emails.
type Client interface {
	Send(to []string, author, subject, body string, campaign model.Campaign) error
}

// SMTPClient is a client for sending emails using SMTP.
type SMTPClient struct {
	addr string
	auth smtp.Auth
	from string
}

// NewClient creates a new SMTP client.
func NewClient(host string, port int, username, password, from string) Client {
	auth := smtp.PlainAuth("", username, password, host)
	addr := fmt.Sprintf("%s:%d", host, port)

	return &SMTPClient{
		addr: addr,
		auth: auth,
		from: from,
	}
}

// Send sends an email to the specified recipients.
func (c *SMTPClient) Send(to []string, author, subject, body string, campaign model.Campaign) error {
	var errs []error
	for _, recipient := range to {
		// Default headers
		headers := map[string]string{
			"To":      recipient,
			"Subject": subject,
		}

		if campaign.Name != "" {
			headers["Subject"] = fmt.Sprintf("[%s] %s", campaign.Name, subject)
		}

		// Build message body
		buildMessage := func(hdrs map[string]string) string {
			msg := ""
			for k, v := range hdrs {
				msg += fmt.Sprintf("%s: %s\r\n", k, v)
			}
			msg += "\r\n" + body
			return msg
		}

		// If author is present, first attempt to send from author's email.
		if author != "" {
			headers["From"] = author
			headers["Reply-To"] = author
			msg := buildMessage(headers)

			// Attempt to send with the author's email as the SMTP FROM address.
			err := smtp.SendMail(c.addr, c.auth, author, []string{recipient}, []byte(msg))
			if err == nil {
				continue // Success, move to next recipient
			}
			// If sending fails, we'll fall back to the default sender.
			// We can log this failure if we had a logger. For now, we just proceed.
		}

		// Fallback or default case: send from the configured default address.
		headers["From"] = c.from
		// If author was present, Reply-To should still be the author on fallback.
		if author != "" {
			headers["Reply-To"] = author
		} else {
			// Ensure Reply-To is not set if there's no author
			delete(headers, "Reply-To")
		}

		msg := buildMessage(headers)

		err := smtp.SendMail(c.addr, c.auth, c.from, []string{recipient}, []byte(msg))
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to send email to %s: %w", recipient, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send email to some recipients: %v", errs)
	}

	return nil
}

// MockClient is a mock implementation of the Client interface.
type MockClient struct {
	SendFunc func(to []string, author, subject, body string, campaign model.Campaign) error
}

// NewMockClient returns a new mock client.
func NewMockClient() *MockClient {
	return &MockClient{}
}

// Send is the mock implementation of the Send method.
func (m *MockClient) Send(to []string, author, subject, body string, campaign model.Campaign) error {
	if m.SendFunc != nil {
		return m.SendFunc(to, author, subject, body, campaign)
	}
	return nil
}
