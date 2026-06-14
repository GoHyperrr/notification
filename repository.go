package notification

import (
	"context"

	"gorm.io/gorm"
)

// Repository handles data access for notifications.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository.
func NewRepository(database *gorm.DB) *Repository {
	return &Repository{db: database}
}

// Save persists a notification to the database.
func (r *Repository) Save(ctx context.Context, n *Notification) error {
	return r.db.WithContext(ctx).Save(n).Error
}

// GetByID retrieves a notification by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Notification, error) {
	var n Notification
	err := r.db.WithContext(ctx).First(&n, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// List retrieves all notifications, optionally filtered by recipient.
func (r *Repository) List(ctx context.Context, recipient string) ([]*Notification, error) {
	var notifications []*Notification
	q := r.db.WithContext(ctx)
	if recipient != "" {
		q = q.Where("recipient = ?", recipient)
	}
	err := q.Find(&notifications).Error
	return notifications, err
}
