package notification

import (
	"context"
	"testing"
	"time"

	"github.com/GoHyperrr/mdk"
	"github.com/GoHyperrr/mdk/mdktest"
)

func TestTemplateResolution(t *testing.T) {
	payload := map[string]any{
		"name": "Alice",
		"order": map[string]any{
			"id":    "ord_123",
			"total": 99.99,
		},
	}

	tpl := "Hi {{payload.name}}, your order {{payload.order.id}} of {{payload.order.total}} is processed."
	resolved := resolveTemplate(tpl, payload)
	expected := "Hi Alice, your order ord_123 of 99.99 is processed."
	if resolved != expected {
		t.Errorf("expected %q, got %q", expected, resolved)
	}
}

func TestCronParsing(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	// test */5 * * * *
	next, err := parseCronAndGetNext("*/5 * * * *", now)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	expectedNext := time.Date(2026, 6, 14, 12, 5, 0, 0, time.UTC)
	if !next.Equal(expectedNext) {
		t.Errorf("expected %v, got %v", expectedNext, next)
	}

	// test specific hour and minute
	next, err = parseCronAndGetNext("30 15 * * *", now)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	expectedNext = time.Date(2026, 6, 14, 15, 30, 0, 0, time.UTC)
	if !next.Equal(expectedNext) {
		t.Errorf("expected %v, got %v", expectedNext, next)
	}
}

func TestAdvancedFeatures(t *testing.T) {
	database, _ := mdktest.SetupSharedTestDB("")
	rt := mdktest.NewTestRuntime(database)
	mockProv := &MockProvider{}

	mod := NewModule(mockProv)
	_ = mod.Init(context.Background(), rt)
	_ = database.AutoMigrate(mod.Models()...)

	ctx := context.Background()

	t.Run("Create and Trigger EventTrigger", func(t *testing.T) {
		sender := "orders@mango.in"
		sub := "Order {{payload.id}} Paid"
		body := "Total amount was {{payload.total}}"

		input := CreateEventTriggerInput{
			Namespace:         "order",
			Event:             "paid",
			Channel:           "EMAIL",
			Sender:            &sender,
			RecipientTemplate: "{{payload.email}}",
			SubjectTemplate:   &sub,
			BodyTemplate:      body,
		}

		trig, err := mod.CreateEventTrigger(ctx, input)
		if err != nil {
			t.Fatalf("CreateEventTrigger failed: %v", err)
		}
		if trig.Namespace != "order" || trig.Event != "paid" {
			t.Errorf("unexpected trigger properties: %+v", trig)
		}

		// List event triggers
		trigs, err := mod.ListEventTriggers(ctx)
		if err != nil || len(trigs) != 1 {
			t.Errorf("ListEventTriggers failed: err=%v, len=%d", err, len(trigs))
		}

		// Trigger the event
		rt.Bus().Publish(ctx, mdk.Event{
			Namespace: "order",
			Type:      "paid",
			Payload: map[string]any{
				"id":    "ord_999",
				"total": "49.99",
				"email": "customer@mango.in",
			},
		})

		time.Sleep(100 * time.Millisecond)

		// Check repository
		notifs, err := mod.ListNotifications(ctx, nil)
		if err != nil || len(notifs) != 1 {
			t.Fatalf("expected 1 notification in repo, got len=%d, err=%v", len(notifs), err)
		}
		n := notifs[0]
		if n.Recipient != "customer@mango.in" || n.Subject != "Order ord_999 Paid" || n.Body != "Total amount was 49.99" {
			t.Errorf("resolved notification template mismatch: %+v", n)
		}

		// Delete trigger
		ok, err := mod.DeleteEventTrigger(ctx, trig.ID)
		if err != nil || !ok {
			t.Errorf("DeleteEventTrigger failed: err=%v, ok=%t", err, ok)
		}
	})

	t.Run("Schedule One-shot and Cron Notifications", func(t *testing.T) {
		sender := "support@mango.in"
		targetTime := time.Now().Add(100 * time.Millisecond)
		subInput := ScheduleNotificationInput{
			Sender:      &sender,
			Recipient:   "support_client@example.com",
			Channel:     "EMAIL",
			Subject:     nil,
			Body:        "Schedule Body",
			ScheduledAt: targetTime,
		}

		job, err := mod.ScheduleNotification(ctx, subInput)
		if err != nil {
			t.Fatalf("ScheduleNotification failed: %v", err)
		}

		jobs, err := mod.ListScheduledNotifications(ctx)
		if err != nil || len(jobs) != 1 {
			t.Errorf("ListScheduledNotifications failed: err=%v, len=%d", err, len(jobs))
		}

		mod.processSchedules(ctx)

		dbJob := ScheduledNotification{}
		database.First(&dbJob, "id = ?", job.ID)
		if dbJob.Status != "PENDING" {
			t.Errorf("expected PENDING status, got %s", dbJob.Status)
		}

		time.Sleep(150 * time.Millisecond)
		mod.processSchedules(ctx)

		database.First(&dbJob, "id = ?", job.ID)
		if dbJob.Status != "SENT" {
			t.Errorf("expected SENT status after running scheduler, got %s", dbJob.Status)
		}

		notifs, _ := mod.ListNotifications(ctx, nil)
		found := false
		for _, n := range notifs {
			if n.Recipient == "support_client@example.com" {
				found = true
			}
		}
		if !found {
			t.Error("expected scheduled notification in database repository")
		}

		cronExpr := "*/5 * * * *"
		cronInput := ScheduleNotificationInput{
			Recipient:      "cron_client@example.com",
			Channel:        "EMAIL",
			Body:           "Cron Body",
			ScheduledAt:    time.Now(),
			CronExpression: &cronExpr,
		}

		cronJob, err := mod.ScheduleNotification(ctx, cronInput)
		if err != nil {
			t.Fatalf("ScheduleNotification cron failed: %v", err)
		}
		if cronJob.CronExpression != "*/5 * * * *" {
			t.Errorf("cron expression mismatch: %s", cronJob.CronExpression)
		}

		ok, err := mod.CancelScheduledNotification(ctx, cronJob.ID)
		if err != nil || !ok {
			t.Errorf("CancelScheduledNotification failed: err=%v, ok=%t", err, ok)
		}

		dbJob = ScheduledNotification{}
		database.First(&dbJob, "id = ?", cronJob.ID)
		if dbJob.Status != "CANCELLED" {
			t.Errorf("expected CANCELLED status, got %s", dbJob.Status)
		}
	})
}
