package notification

import (
	"time"

	"gorm.io/gorm"
)

type NotificationStatus string
type NotificationChannel string

const (
	StatusPending NotificationStatus = "PENDING"
	StatusSent    NotificationStatus = "SENT"
	StatusFailed  NotificationStatus = "FAILED"

	ChannelEmail    NotificationChannel = "EMAIL"
	ChannelSMS      NotificationChannel = "SMS"
	ChannelWhatsapp NotificationChannel = "WHATSAPP"
)

// Notification represents a message sent to a user.
type Notification struct {
	ID        string              `gorm:"primaryKey" json:"id"`
	Sender    string              `json:"sender"` // e.g. "orders@mango.in", "support@mango.in" or "+14155552671"
	Recipient string              `gorm:"index;not null" json:"recipient"`
	Channel   NotificationChannel `gorm:"not null" json:"channel"`
	Subject   string              `json:"subject"`
	Body      string              `json:"body"`
	Status    NotificationStatus  `gorm:"not null" json:"status"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
	DeletedAt gorm.DeletedAt      `gorm:"index" json:"-"`
}

type EventTrigger struct {
	ID                string `gorm:"primaryKey" json:"id"`
	Namespace         string `gorm:"index;not null" json:"namespace"`
	Event             string `gorm:"not null" json:"event"`
	Channel           string `gorm:"not null" json:"channel"`
	Sender            string `json:"sender"`
	RecipientTemplate string `gorm:"not null" json:"recipient_template"`
	SubjectTemplate   string `json:"subject_template"`
	BodyTemplate      string `gorm:"not null" json:"body_template"`
	Enabled           bool   `gorm:"default:true" json:"enabled"`
}

type ScheduledNotification struct {
	ID             string     `gorm:"primaryKey" json:"id"`
	Sender         string     `json:"sender"`
	Recipient      string     `gorm:"not null" json:"recipient"`
	Channel        string     `gorm:"not null" json:"channel"`
	Subject        string     `json:"subject"`
	Body           string     `gorm:"not null" json:"body"`
	ScheduledAt    time.Time  `gorm:"index" json:"scheduled_at"`
	CronExpression string     `json:"cron_expression"`
	Status         string     `gorm:"default:'PENDING'" json:"status"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CreateEventTriggerInput struct {
	Namespace         string  `json:"namespace"`
	Event             string  `json:"event"`
	Channel           string  `json:"channel"`
	Sender            *string `json:"sender,omitempty"`
	RecipientTemplate string  `json:"recipientTemplate"`
	SubjectTemplate   *string `json:"subjectTemplate,omitempty"`
	BodyTemplate      string  `json:"bodyTemplate"`
}

type ScheduleNotificationInput struct {
	Sender         *string    `json:"sender,omitempty"`
	Recipient      string     `json:"recipient"`
	Channel        string     `json:"channel"`
	Subject        *string    `json:"subject,omitempty"`
	Body           string     `json:"body"`
	ScheduledAt    time.Time  `json:"scheduledAt"`
	CronExpression *string    `json:"cronExpression,omitempty"`
}
