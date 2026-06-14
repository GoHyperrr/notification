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
