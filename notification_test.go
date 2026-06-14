package notification

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/GoHyperrr/mdk"
	"github.com/GoHyperrr/mdk/mdktest"
)

func TestNotificationModule(t *testing.T) {
	database, _ := mdktest.SetupSharedTestDB("")
	rt := mdktest.NewTestRuntime(database)
	
	// Create mock provider
	mockProv := &MockProvider{}

	mod := NewModule(mockProv)
	_ = mod.Init(context.Background(), rt)
	_ = database.AutoMigrate(mod.Models()...)
	runner := rt.Workflows().(*mdktest.TestWorkflowEngine)

	t.Run("Send Notification Success", func(t *testing.T) {
		recipient := fmt.Sprintf("test_%s@example.com", uuid.New().String()[:8])
		wf := mdk.Workflow{
			ID:    "test-send-wf",
			Name:  "Test Send Notification",
			Steps: []mdk.Step{{ID: "send", Uses: "notification.send"}},
		}
		_ = runner.Register(wf)

		input := map[string]any{
			"recipient": recipient,
			"channel":   "EMAIL",
			"subject":   "Test",
			"body":      "Hello",
		}

		res, err := runner.ExecuteSync(context.Background(), "n1", "test-send-wf", input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		sendRes, ok := res["send"].(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", res["send"])
		}
		n, ok := sendRes["notification"].(*Notification)
		if !ok {
			t.Fatalf("expected notification pointer, got %T", sendRes["notification"])
		}
		if n.Status != StatusSent {
			t.Errorf("expected SENT status, got %s", n.Status)
		}

		// Verify Repo
		list, _ := mod.Repo().List(context.Background(), recipient)
		if len(list) != 1 {
			t.Error("expected 1 notification in repo")
		}
	})

	t.Run("Send Notification Failure", func(t *testing.T) {
		recipient := fmt.Sprintf("fail_%s@example.com", uuid.New().String()[:8])
		mockProv.ShouldFail = true
		
		wf := mdk.Workflow{
			ID:    "test-fail-wf",
			Name:  "Test Fail Notification",
			Steps: []mdk.Step{{ID: "send", Uses: "notification.send"}},
		}
		_ = runner.Register(wf)

		input := map[string]any{
			"recipient": recipient,
			"channel":   "EMAIL",
			"subject":   "Test",
			"body":      "Hello",
		}

		_, err := runner.ExecuteSync(context.Background(), "n2", "test-fail-wf", input)
		if err == nil {
			t.Fatal("expected workflow failure")
		}

		// DB should still have it as FAILED
		list, _ := mod.Repo().List(context.Background(), recipient)
		if len(list) != 1 || list[0].Status != StatusFailed {
			t.Error("expected 1 FAILED notification in repo")
		}
		
		mockProv.ShouldFail = false // Reset
	})
	
	t.Run("Event Subscriptions", func(t *testing.T) {
		recipient := fmt.Sprintf("event_%s@example.com", uuid.New().String()[:8])
		// Test identity.user_created
		rt.Bus().Publish(context.Background(), mdk.Event{
			Namespace: "identity",
			Type:      "user_created",
			Payload: map[string]any{
				"email": recipient,
				"name":  "Event User",
			},
		})
		
		// Wait for welcome email workflow to be executed asynchronously
		time.Sleep(100 * time.Millisecond)
		
		list, _ := mod.Repo().List(context.Background(), recipient)
		if len(list) != 1 {
			t.Error("expected welcome email to be sent")
		}
		
		// Test workflow.completed (fulfillment)
		rt.Bus().Publish(context.Background(), mdk.Event{
			Namespace: "workflow",
			Type:      "completed",
			Payload: map[string]any{
				"name": "fulfillment.v1",
			},
		})
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		// 1. Invalid input
		_, err := mod.SendNotification(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		
		// 2. Missing workflow input
		_, err = mod.SendNotification(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }
		
		// 3. Missing recipient
		_, err = mod.SendNotification(context.Background(), map[string]any{"input": map[string]any{}})
		if err == nil { t.Error("expected error for missing recipient") }
	})

	t.Run("Repository Edge Cases", func(t *testing.T) {
		repo := mod.Repo()
		ctx := context.Background()

		// 1. GetByID Not Found
		_, err := repo.GetByID(ctx, "ghost")
		if err == nil { t.Error("expected error for non-existent notif") }

		// 2. List with recipient filter
		n1 := &Notification{ID: "notif_1", Recipient: "user1", Status: StatusSent}
		n2 := &Notification{ID: "notif_2", Recipient: "user2", Status: StatusSent}
		repo.Save(ctx, n1)
		repo.Save(ctx, n2)

		list1, _ := repo.List(ctx, "user1")
		if len(list1) != 1 || list1[0].ID != "notif_1" { t.Error("List filter failed for user1") }

		listAll, _ := repo.List(ctx, "")
		if len(listAll) < 2 { t.Error("List with empty filter failed") }
	})
}

func TestSMTPIntegration(t *testing.T) {
	ctx := context.Background()

	// Start mock SMTP server
	addr, cleanup := startMockSMTPServer(t)
	defer cleanup()

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to split host/port: %v", err)
	}
	port, _ := strconv.Atoi(portStr)

	// Configure SMTPProvider
	defaultConfig := SMTPConfig{
		Host:     host,
		Port:     port,
		Username: "default@mango.in",
		Password: "default_password",
		From:     "default@mango.in",
	}

	senders := map[string]SMTPConfig{
		"support@mango.in": {
			Host:     host,
			Port:     port,
			Username: "support@mango.in",
			Password: "support_password",
			From:     "support@mango.in",
		},
	}

	provider := NewSMTPProvider(defaultConfig, senders)

	t.Run("Send using default sender", func(t *testing.T) {
		n := &Notification{
			ID:        "n_default",
			Recipient: "customer@example.com",
			Channel:   ChannelEmail,
			Subject:   "Welcome to Mango Farms",
			Body:      "Hello World",
		}
		err := provider.Send(ctx, n)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	})

	t.Run("Send using support sender override", func(t *testing.T) {
		n := &Notification{
			ID:        "n_support",
			Sender:    "support@mango.in",
			Recipient: "customer@example.com",
			Channel:   ChannelEmail,
			Subject:   "Re: inquiry",
			Body:      "Refund processed.",
		}
		err := provider.Send(ctx, n)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	})
}

func startMockSMTPServer(t *testing.T) (string, func()) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock SMTP server: %v", err)
	}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				reader := bufio.NewReader(c)
				writer := bufio.NewWriter(c)

				writeLine := func(line string) {
					writer.WriteString(line + "\r\n")
					writer.Flush()
				}

				writeLine("220 mock.smtp.server ESMTP Ready")

				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "EHLO") || strings.HasPrefix(line, "HELO") {
						writeLine("250-mock.smtp.server\r\n250 AUTH PLAIN")
					} else if strings.HasPrefix(line, "AUTH PLAIN") {
						writeLine("235 Authentication successful")
					} else if strings.HasPrefix(line, "MAIL FROM:") {
						writeLine("250 2.1.0 Ok")
					} else if strings.HasPrefix(line, "RCPT TO:") {
						writeLine("250 2.1.5 Ok")
					} else if line == "DATA" {
						writeLine("354 Start mail input; end with <CR><LF>.<CR><LF>")
						for {
							dataLine, err := reader.ReadString('\n')
							if err != nil {
								return
							}
							if strings.TrimSpace(dataLine) == "." {
								break
							}
						}
						writeLine("250 2.0.0 OK: queued")
					} else if line == "QUIT" {
						writeLine("221 2.0.0 Bye")
						return
					} else {
						writeLine("500 Command unrecognized")
					}
				}
			}(conn)
		}
	}()

	addr := l.Addr().String()
	cleanup := func() {
		l.Close()
	}
	return addr, cleanup
}
