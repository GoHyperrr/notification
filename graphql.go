package notification

import (
	"context"
)

func (m *Module) Queries() map[string]any {
	return map[string]any{
		"listNotifications": m.ListNotifications,
	}
}

func (m *Module) Mutations() map[string]any {
	return nil
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

