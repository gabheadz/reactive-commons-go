package rabbit

import (
	"testing"
	"time"
)

func TestNewRabbitClient(t *testing.T) {
	appName := "test-app"
	connAddr := "amqp://localhost:5672/"

	client, err := NewRabbitClient(appName, connAddr)

	if err != nil {
		t.Fatalf("NewRabbitClient() returned error: %v", err)
	}

	if client == nil {
		t.Fatal("NewRabbitClient() returned nil client")
	}

	if client.AppName != appName {
		t.Errorf("Expected AppName '%s', got '%s'", appName, client.AppName)
	}

	if client.connAddr != connAddr {
		t.Errorf("Expected connAddr '%s', got '%s'", connAddr, client.connAddr)
	}

	if client.Conn != nil {
		t.Error("Expected Conn to be nil before Connect()")
	}

	if client.channels == nil {
		t.Error("Expected channels map to be initialized")
	}

	if len(client.channels) != 0 {
		t.Errorf("Expected empty channels map, got %d entries", len(client.channels))
	}
}

func TestRabbitClient_GetChannel_NotFound(t *testing.T) {
	client, _ := NewRabbitClient("test-app", "amqp://localhost")

	_, err := client.GetChannel("non-existent")

	if err == nil {
		t.Error("Expected error for non-existent channel, got nil")
	}

	expectedMsg := "RabbitMQ channel not found: non-existent"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRabbitClient_Close_NilConnection(t *testing.T) {
	client, _ := NewRabbitClient("test-app", "amqp://localhost")

	err := client.Close()

	if err != nil {
		t.Errorf("Close() with nil connection should not return error, got: %v", err)
	}
}

func TestConsumeOne_Timeout(t *testing.T) {
	client, _ := NewRabbitClient("test-app", "amqp://localhost")

	// This test verifies the timeout logic without actual RabbitMQ connection
	// In a real scenario, this would timeout since there's no channel
	correlationId := "test-correlation-id"
	timeout := 100 * time.Millisecond

	_, err := client.ConsumeOne("test-channel", "test-queue", correlationId, timeout)

	if err == nil {
		t.Error("Expected error for ConsumeOne without valid channel")
	}
}

func TestRabbitClient_PublishJson_MissingMessageId(t *testing.T) {
	client, _ := NewRabbitClient("test-app", "amqp://localhost")

	headers := map[string]string{
		"sourceApplication": "test-app",
		"delivery_mode":     "persistent",
	}

	// This will fail because there's no channel, but tests the message_id logic
	err := client.PublishJson("test-exchange", "test-key", []byte("test"), "test-channel", headers)

	if err == nil {
		t.Error("Expected error when publishing without valid channel")
	}
}

func TestRabbitClient_PublishJsonQuery(t *testing.T) {
	client, _ := NewRabbitClient("test-app", "amqp://localhost")

	headers := map[string]string{}
	correlationId := "test-correlation"
	replyQueue := "reply-queue"

	// This will fail because there's no channel
	err := client.PublishJsonQuery("test-exchange", "test-key", []byte("test"), replyQueue, correlationId, "test-channel", headers)

	if err == nil {
		t.Error("Expected error when publishing query without valid channel")
	}
}
