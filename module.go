package notification

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GoHyperrr/mdk"
	"github.com/google/uuid"
)

// Module implements the mdk.Module interface for Notification.
type Module struct {
	repo               *Repository
	provider           Provider
	rt                 mdk.Runtime
	schedulerCtxCancel context.CancelFunc
	activeTriggers     map[string]func()
	triggersMu         sync.Mutex
}

func NewModule(provider Provider) *Module {
	return &Module{
		provider:       provider,
		activeTriggers: make(map[string]func()),
	}
}

func (m *Module) ID() string {
	return "notification"
}

func (m *Module) Init(ctx context.Context, rt mdk.Runtime) error {
	m.rt = rt
	m.repo = NewRepository(rt.DB())

	if m.provider == nil {
		// Load default SMTP Config
		var smtpCfg SMTPConfig
		if host, ok := rt.Config("smtp_host").(string); ok {
			smtpCfg.Host = host
		}
		if port, ok := rt.Config("smtp_port").(float64); ok {
			smtpCfg.Port = int(port)
		} else if portInt, ok := rt.Config("smtp_port").(int); ok {
			smtpCfg.Port = portInt
		}
		if user, ok := rt.Config("smtp_user").(string); ok {
			smtpCfg.Username = user
		}
		if pass, ok := rt.Config("smtp_pass").(string); ok {
			smtpCfg.Password = pass
		}
		if from, ok := rt.Config("smtp_from").(string); ok {
			smtpCfg.From = from
		}

		// Load specific SMTP sender profiles if defined under "smtp_senders"
		smtpSenders := make(map[string]SMTPConfig)
		if sendersRaw, ok := rt.Config("smtp_senders").(map[string]any); ok {
			for email, details := range sendersRaw {
				if detailMap, ok := details.(map[string]any); ok {
					var sc SMTPConfig
					sc.Host, _ = detailMap["smtp_host"].(string)
					if p, ok := detailMap["smtp_port"].(float64); ok {
						sc.Port = int(p)
					} else if p, ok := detailMap["smtp_port"].(int); ok {
						sc.Port = p
					}
					sc.Username, _ = detailMap["smtp_user"].(string)
					sc.Password, _ = detailMap["smtp_pass"].(string)
					sc.From, _ = detailMap["smtp_from"].(string)
					if sc.From == "" {
						sc.From = email
					}
					smtpSenders[email] = sc
				}
			}
		}

		// Load default Twilio Config
		var twilioCfg TwilioWhatsappConfig
		if sid, ok := rt.Config("twilio_sid").(string); ok {
			twilioCfg.AccountSID = sid
		}
		if token, ok := rt.Config("twilio_token").(string); ok {
			twilioCfg.AuthToken = token
		}
		if from, ok := rt.Config("twilio_from").(string); ok {
			twilioCfg.From = from
		}

		// Load specific WhatsApp sender profiles if defined under "whatsapp_senders"
		whatsappSenders := make(map[string]TwilioWhatsappConfig)
		if sendersRaw, ok := rt.Config("whatsapp_senders").(map[string]any); ok {
			for phone, details := range sendersRaw {
				if detailMap, ok := details.(map[string]any); ok {
					var wc TwilioWhatsappConfig
					wc.AccountSID, _ = detailMap["twilio_sid"].(string)
					wc.AuthToken, _ = detailMap["twilio_token"].(string)
					wc.From, _ = detailMap["twilio_from"].(string)
					if wc.From == "" {
						wc.From = phone
					}
					whatsappSenders[phone] = wc
				}
			}
		}

		emailProvider := NewSMTPProvider(smtpCfg, smtpSenders)
		whatsappProvider := NewTwilioWhatsappProvider(twilioCfg, whatsappSenders)
		m.provider = NewMultiChannelRoutingProvider(emailProvider, whatsappProvider)
	}

	// Register workflows
	_ = rt.Workflows().Register(mdk.Workflow{
		ID:   "notification.send_welcome",
		Name: "Send Welcome",
		Steps: []mdk.Step{
			{
				ID:   "send",
				Name: "Send Email",
				Uses: "notification.send",
			},
		},
	})

	// Register workflow step handlers
	_ = rt.Workflows().RegisterHandler("notification.send", m.SendNotificationStep)

	// Subscribe to Identity User Created
	_, _ = rt.Bus().Subscribe("identity", "user_created", func(ctx context.Context, event mdk.Event) error {
		email := getString(event.Payload, "email")
		name := getString(event.Payload, "name")

		input := map[string]any{
			"recipient": email,
			"channel":   string(ChannelEmail),
			"subject":   "Welcome to hyperrr!",
			"body":      fmt.Sprintf("Hi %s, thanks for joining.", name),
		}

		go rt.Workflows().Execute(ctx, "notification.send_welcome", input)
		return nil
	})

	// Subscribe to Order Completed (Workflow Completed)
	_, _ = rt.Bus().Subscribe("workflow", "completed", func(ctx context.Context, event mdk.Event) error {
		wfName := getString(event.Payload, "name")
		if wfName != "fulfillment.v1" {
			return nil
		}

		// In a real system, we'd fetch the order details here to get the email.
		// For this MVP, we'll just log that we would send it if we had the context easily available.
		rt.Logger().Info("Fulfillment completed, would send order confirmation email")

		return nil
	})

	// Start scheduler background worker
	schCtx, cancel := context.WithCancel(ctx)
	m.schedulerCtxCancel = cancel
	go m.startScheduler(schCtx)

	// Register dynamic event triggers
	m.registerDynamicTriggers(ctx)

	return nil
}

func (m *Module) Shutdown(ctx context.Context) error {
	if m.schedulerCtxCancel != nil {
		m.schedulerCtxCancel()
	}
	m.triggersMu.Lock()
	for _, unsub := range m.activeTriggers {
		unsub()
	}
	m.activeTriggers = make(map[string]func())
	m.triggersMu.Unlock()
	return nil
}

func (m *Module) Models() []any {
	return []any{&Notification{}, &EventTrigger{}, &ScheduledNotification{}}
}

func (m *Module) Routes() []mdk.Route {
	return nil
}

func (m *Module) Repo() *Repository {
	return m.repo
}

// SendNotificationStep wraps SendNotification to mdk.StepHandler.
func (m *Module) SendNotificationStep(sCtx mdk.StepContext) mdk.StepResult {
	res, err := m.SendNotification(sCtx.Ctx, map[string]any{
		"input": sCtx.Input,
	})
	if err != nil {
		return mdk.StepResult{Err: err}
	}
	resMap, ok := res.(*Notification)
	if ok {
		return mdk.StepResult{Output: map[string]any{"notification": resMap}}
	}
	return mdk.StepResult{}
}

func (m *Module) startScheduler(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.processSchedules(ctx)
		}
	}
}

func (m *Module) processSchedules(ctx context.Context) {
	if m.rt == nil || m.rt.DB() == nil {
		return
	}
	var pending []ScheduledNotification
	now := time.Now()

	if err := m.rt.DB().Where("status = ? AND scheduled_at <= ?", "PENDING", now).Find(&pending).Error; err != nil {
		return
	}

	for _, job := range pending {
		n := &Notification{
			ID:        "notif_" + uuid.New().String(),
			Sender:    job.Sender,
			Recipient: job.Recipient,
			Channel:   NotificationChannel(job.Channel),
			Subject:   job.Subject,
			Body:      job.Body,
			Status:    StatusPending,
		}

		err := m.repo.Save(ctx, n)
		if err == nil {
			err = m.provider.Send(ctx, n)
		}

		tx := m.rt.DB().Begin()
		if err != nil {
			n.Status = StatusFailed
			_ = m.repo.Save(ctx, n)

			job.Status = "FAILED"
			tx.Save(&job)
			tx.Commit()
			continue
		}

		n.Status = StatusSent
		_ = m.repo.Save(ctx, n)

		job.LastRunAt = &now
		if job.CronExpression == "" {
			job.Status = "SENT"
		} else {
			nextRun, parseErr := parseCronAndGetNext(job.CronExpression, now)
			if parseErr != nil {
				job.Status = "FAILED"
				m.rt.Logger().Error("scheduler: invalid cron expression", "expr", job.CronExpression, "err", parseErr)
			} else {
				job.ScheduledAt = nextRun
				job.Status = "PENDING"
			}
		}
		tx.Save(&job)
		tx.Commit()
	}
}

func (m *Module) registerDynamicTriggers(ctx context.Context) {
	if m.rt == nil || m.rt.DB() == nil {
		return
	}
	var triggers []EventTrigger
	if err := m.rt.DB().Where("enabled = ?", true).Find(&triggers).Error; err != nil {
		return
	}

	for _, t := range triggers {
		m.subscribeTrigger(ctx, t)
	}
}

func (m *Module) subscribeTrigger(ctx context.Context, t EventTrigger) {
	m.triggersMu.Lock()
	defer m.triggersMu.Unlock()

	if unsub, ok := m.activeTriggers[t.ID]; ok {
		unsub()
	}

	unsub, err := m.rt.Bus().Subscribe(t.Namespace, t.Event, func(ctx context.Context, event mdk.Event) error {
		recipient := resolveTemplate(t.RecipientTemplate, event.Payload)
		subject := resolveTemplate(t.SubjectTemplate, event.Payload)
		body := resolveTemplate(t.BodyTemplate, event.Payload)

		input := map[string]any{
			"input": map[string]any{
				"sender":    t.Sender,
				"recipient": recipient,
				"channel":   t.Channel,
				"subject":   subject,
				"body":      body,
			},
		}

		_, err := m.SendNotification(ctx, input)
		return err
	})

	if err == nil {
		m.activeTriggers[t.ID] = unsub
	}
}

func (m *Module) unsubscribeTrigger(id string) {
	m.triggersMu.Lock()
	defer m.triggersMu.Unlock()

	if unsub, ok := m.activeTriggers[id]; ok {
		unsub()
		delete(m.activeTriggers, id)
	}
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func init() {
	mdk.Register(func() mdk.Module {
		return NewModule(nil)
	})
}
