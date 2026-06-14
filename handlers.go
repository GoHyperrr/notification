package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SendNotification executes the notification delivery via the provider.
func (m *Module) SendNotification(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	sender, _ := workflowInput["sender"].(string)
	recipient, _ := workflowInput["recipient"].(string)
	if recipient == "" {
		return nil, fmt.Errorf("missing recipient")
	}
	channelStr, _ := workflowInput["channel"].(string)
	subject, _ := workflowInput["subject"].(string)
	body, _ := workflowInput["body"].(string)

	n := &Notification{
		ID:        "notif_" + uuid.New().String(),
		Sender:    sender,
		Recipient: recipient,
		Channel:   NotificationChannel(channelStr),
		Subject:   subject,
		Body:      body,
		Status:    StatusPending,
	}

	if err := m.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("failed to save pending notification: %w", err)
	}

	err := m.provider.Send(ctx, n)
	if err != nil {
		n.Status = StatusFailed
		m.repo.Save(ctx, n)
		return nil, fmt.Errorf("failed to send notification: %w", err)
	}

	n.Status = StatusSent
	if err := m.repo.Save(ctx, n); err != nil {
		return nil, fmt.Errorf("failed to save sent notification status: %w", err)
	}

	return n, nil
}
