package notification

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
)

// Provider defines the interface for sending notifications.
type Provider interface {
	Send(ctx context.Context, n *Notification) error
}

// MockProvider is a simple provider for testing and development.
type MockProvider struct {
	ShouldFail bool
}

func (m *MockProvider) Send(ctx context.Context, n *Notification) error {
	if m.ShouldFail {
		return context.DeadlineExceeded // Simulate a network failure
	}
	slog.Info("MockProvider: Sent notification", "recipient", n.Recipient, "channel", n.Channel, "sender", n.Sender, "subject", n.Subject)
	return nil
}

// SMTPConfig holds SMTP server credentials.
type SMTPConfig struct {
	Host     string `json:"smtp_host"`
	Port     int    `json:"smtp_port"`
	Username string `json:"smtp_user"`
	Password string `json:"smtp_pass"`
	From     string `json:"smtp_from"`
}

// SMTPProvider implements email delivery using net/smtp.
type SMTPProvider struct {
	defaultConfig SMTPConfig
	senders       map[string]SMTPConfig
}

func NewSMTPProvider(defaultConfig SMTPConfig, senders map[string]SMTPConfig) *SMTPProvider {
	return &SMTPProvider{
		defaultConfig: defaultConfig,
		senders:       senders,
	}
}

func (s *SMTPProvider) Send(ctx context.Context, n *Notification) error {
	cfg := s.defaultConfig
	if n.Sender != "" {
		if specificCfg, exists := s.senders[n.Sender]; exists {
			cfg = specificCfg
		}
	}

	if cfg.Host == "" {
		slog.Warn("SMTP: Host not configured. Mock sending email", "to", n.Recipient, "sender", n.Sender, "subject", n.Subject)
		return nil
	}

	from := cfg.From
	if n.Sender != "" {
		from = n.Sender
	}
	if from == "" {
		from = cfg.Username
	}

	msg := []byte("To: " + n.Recipient + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + n.Subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		n.Body + "\r\n")

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	err := smtp.SendMail(addr, auth, from, []string{n.Recipient}, msg)
	if err != nil {
		return fmt.Errorf("failed to send SMTP email: %w", err)
	}

	slog.Info("SMTP: Email sent successfully", "from", from, "to", n.Recipient, "subject", n.Subject)
	return nil
}

// TwilioWhatsappConfig holds Twilio API credentials.
type TwilioWhatsappConfig struct {
	AccountSID string `json:"twilio_sid"`
	AuthToken  string `json:"twilio_token"`
	From       string `json:"twilio_from"`
}

// TwilioWhatsappProvider implements WhatsApp message delivery using Twilio REST API.
type TwilioWhatsappProvider struct {
	defaultConfig TwilioWhatsappConfig
	senders       map[string]TwilioWhatsappConfig
}

func NewTwilioWhatsappProvider(defaultConfig TwilioWhatsappConfig, senders map[string]TwilioWhatsappConfig) *TwilioWhatsappProvider {
	return &TwilioWhatsappProvider{
		defaultConfig: defaultConfig,
		senders:       senders,
	}
}

func (w *TwilioWhatsappProvider) Send(ctx context.Context, n *Notification) error {
	cfg := w.defaultConfig
	if n.Sender != "" {
		if specificCfg, exists := w.senders[n.Sender]; exists {
			cfg = specificCfg
		}
	}

	if cfg.AccountSID == "" || cfg.AuthToken == "" {
		slog.Warn("Twilio: Credentials not configured. Mock sending WhatsApp", "to", n.Recipient, "sender", n.Sender, "body", n.Body)
		return nil
	}

	from := cfg.From
	if n.Sender != "" {
		from = n.Sender
	}

	if !strings.HasPrefix(from, "whatsapp:") {
		from = "whatsapp:" + from
	}
	to := n.Recipient
	if !strings.HasPrefix(to, "whatsapp:") {
		to = "whatsapp:" + to
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", cfg.AccountSID)

	data := url.Values{}
	data.Set("From", from)
	data.Set("To", to)
	data.Set("Body", n.Body)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.SetBasicAuth(cfg.AccountSID, cfg.AuthToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Twilio API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio API returned error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	slog.Info("Twilio: WhatsApp message sent successfully", "from", from, "to", to)
	return nil
}

// MultiChannelRoutingProvider routes notifications to appropriate providers based on channel.
type MultiChannelRoutingProvider struct {
	emailProvider    Provider
	whatsappProvider Provider
}

func NewMultiChannelRoutingProvider(email, whatsapp Provider) *MultiChannelRoutingProvider {
	return &MultiChannelRoutingProvider{
		emailProvider:    email,
		whatsappProvider: whatsapp,
	}
}

func (m *MultiChannelRoutingProvider) Send(ctx context.Context, n *Notification) error {
	switch n.Channel {
	case ChannelEmail:
		if m.emailProvider != nil {
			return m.emailProvider.Send(ctx, n)
		}
	case ChannelWhatsapp:
		if m.whatsappProvider != nil {
			return m.whatsappProvider.Send(ctx, n)
		}
	default:
		slog.Warn("No provider available for notification channel", "channel", n.Channel)
	}
	
	// Fallback/Default behavior is logging
	slog.Info("FallbackProvider: Sent notification", "recipient", n.Recipient, "channel", n.Channel, "subject", n.Subject)
	return nil
}
