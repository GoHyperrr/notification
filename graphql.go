package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (m *Module) Queries() map[string]any {
	return map[string]any{
		"listNotifications":          m.ListNotifications,
		"listEventTriggers":          m.ListEventTriggers,
		"listScheduledNotifications": m.ListScheduledNotifications,
	}
}

func (m *Module) Mutations() map[string]any {
	return map[string]any{
		"createEventTrigger":          m.CreateEventTrigger,
		"deleteEventTrigger":          m.DeleteEventTrigger,
		"scheduleNotification":        m.ScheduleNotification,
		"cancelScheduledNotification": m.CancelScheduledNotification,
	}
}

func (m *Module) FieldResolvers() map[string]any {
	return nil
}

func (m *Module) ListNotifications(ctx context.Context, recipient *string) ([]*Notification, error) {
	recip := ""
	if recipient != nil {
		recip = *recipient
	}

	return m.repo.List(ctx, recip)
}

func (m *Module) ListEventTriggers(ctx context.Context) ([]*EventTrigger, error) {
	var list []*EventTrigger
	if err := m.rt.DB().Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (m *Module) ListScheduledNotifications(ctx context.Context) ([]*ScheduledNotification, error) {
	var list []*ScheduledNotification
	if err := m.rt.DB().Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (m *Module) CreateEventTrigger(ctx context.Context, input CreateEventTriggerInput) (*EventTrigger, error) {
	sender := ""
	if input.Sender != nil {
		sender = *input.Sender
	}
	subject := ""
	if input.SubjectTemplate != nil {
		subject = *input.SubjectTemplate
	}

	t := EventTrigger{
		ID:                "trig_" + uuid.New().String(),
		Namespace:         input.Namespace,
		Event:             input.Event,
		Channel:           input.Channel,
		Sender:            sender,
		RecipientTemplate: input.RecipientTemplate,
		SubjectTemplate:   subject,
		BodyTemplate:      input.BodyTemplate,
		Enabled:           true,
	}

	if err := m.rt.DB().Create(&t).Error; err != nil {
		return nil, err
	}

	// Dynamic registration
	m.subscribeTrigger(ctx, t)

	return &t, nil
}

func (m *Module) DeleteEventTrigger(ctx context.Context, id string) (bool, error) {
	m.unsubscribeTrigger(id)
	if err := m.rt.DB().Delete(&EventTrigger{}, "id = ?", id).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (m *Module) ScheduleNotification(ctx context.Context, input ScheduleNotificationInput) (*ScheduledNotification, error) {
	sender := ""
	if input.Sender != nil {
		sender = *input.Sender
	}
	subject := ""
	if input.Subject != nil {
		subject = *input.Subject
	}
	cron := ""
	if input.CronExpression != nil {
		cron = *input.CronExpression
	}

	scheduledAt := input.ScheduledAt
	if cron != "" {
		var err error
		scheduledAt, err = parseCronAndGetNext(cron, time.Now())
		if err != nil {
			return nil, fmt.Errorf("invalid cron expression: %w", err)
		}
	}

	job := ScheduledNotification{
		ID:             "job_" + uuid.New().String(),
		Sender:         sender,
		Recipient:      input.Recipient,
		Channel:        input.Channel,
		Subject:        subject,
		Body:           input.Body,
		ScheduledAt:    scheduledAt,
		CronExpression: cron,
		Status:         "PENDING",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := m.rt.DB().Create(&job).Error; err != nil {
		return nil, err
	}

	return &job, nil
}

func (m *Module) CancelScheduledNotification(ctx context.Context, id string) (bool, error) {
	if err := m.rt.DB().Model(&ScheduledNotification{}).Where("id = ?", id).Update("status", "CANCELLED").Error; err != nil {
		return false, err
	}
	return true, nil
}
