package rabbit

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Mock channel for testing
type mockChannel struct {
	exchangeDeclared bool
	exchangeName     string
	exchangeType     string
	queueDeclared    bool
	queueName        string
	queueArgs        amqp.Table
	boundQueues      map[string]boundQueue
	unboundQueues    map[string]boundQueue
}

type boundQueue struct {
	name     string
	key      string
	exchange string
}

func (m *mockChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	m.exchangeDeclared = true
	m.exchangeName = name
	m.exchangeType = kind
	return nil
}

func (m *mockChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	m.queueDeclared = true
	m.queueName = name
	m.queueArgs = args
	return amqp.Queue{Name: name}, nil
}

func (m *mockChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	if m.boundQueues == nil {
		m.boundQueues = make(map[string]boundQueue)
	}
	m.boundQueues[name] = boundQueue{name: name, key: key, exchange: exchange}
	return nil
}

func (m *mockChannel) QueueUnbind(name, key, exchange string, args amqp.Table) error {
	if m.unboundQueues == nil {
		m.unboundQueues = make(map[string]boundQueue)
	}
	m.unboundQueues[name] = boundQueue{name: name, key: key, exchange: exchange}
	return nil
}

func TestDeclareExchange(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareExchange(mock, "test-exchange", "topic", true, false, false, false)

	if err != nil {
		t.Errorf("DeclareExchange() returned error: %v", err)
	}

	if !mock.exchangeDeclared {
		t.Error("Exchange was not declared")
	}

	if mock.exchangeName != "test-exchange" {
		t.Errorf("Expected exchange name 'test-exchange', got '%s'", mock.exchangeName)
	}

	if mock.exchangeType != "topic" {
		t.Errorf("Expected exchange type 'topic', got '%s'", mock.exchangeType)
	}
}

func TestDeclareExchange_Direct(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareExchange(mock, "direct-exchange", "direct", true, false, false, false)

	if err != nil {
		t.Errorf("DeclareExchange() returned error: %v", err)
	}

	if mock.exchangeType != "direct" {
		t.Errorf("Expected exchange type 'direct', got '%s'", mock.exchangeType)
	}
}

func TestDeclareExchange_Fanout(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareExchange(mock, "fanout-exchange", "fanout", false, true, false, false)

	if err != nil {
		t.Errorf("DeclareExchange() returned error: %v", err)
	}

	if mock.exchangeType != "fanout" {
		t.Errorf("Expected exchange type 'fanout', got '%s'", mock.exchangeType)
	}
}

func TestDeclareQueue(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareQueue(mock, "classic", "test-queue", true, false, false, false)

	if err != nil {
		t.Errorf("DeclareQueue() returned error: %v", err)
	}

	if !mock.queueDeclared {
		t.Error("Queue was not declared")
	}

	if mock.queueName != "test-queue" {
		t.Errorf("Expected queue name 'test-queue', got '%s'", mock.queueName)
	}

	if mock.queueArgs["x-queue-type"] != "classic" {
		t.Errorf("Expected queue type 'classic', got '%v'", mock.queueArgs["x-queue-type"])
	}
}

func TestDeclareQueue_Quorum(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareQueue(mock, "quorum", "quorum-queue", true, false, false, false)

	if err != nil {
		t.Errorf("DeclareQueue() returned error: %v", err)
	}

	if mock.queueArgs["x-queue-type"] != "quorum" {
		t.Errorf("Expected queue type 'quorum', got '%v'", mock.queueArgs["x-queue-type"])
	}
}

func TestDeclareQueue_EmptyType(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareQueue(mock, "", "no-type-queue", true, false, false, false)

	if err != nil {
		t.Errorf("DeclareQueue() returned error: %v", err)
	}

	if _, exists := mock.queueArgs["x-queue-type"]; exists {
		t.Error("Expected no x-queue-type argument when queueType is empty")
	}
}

func TestDeclareQueue_QuorumWithAutoDelete(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareQueue(mock, "quorum", "auto-delete-queue", true, true, false, false)

	if err != nil {
		t.Errorf("DeclareQueue() returned error: %v", err)
	}

	// Should fallback to classic when quorum is used with autoDelete
	if mock.queueArgs["x-queue-type"] != "classic" {
		t.Errorf("Expected queue type 'classic' (fallback), got '%v'", mock.queueArgs["x-queue-type"])
	}
}

func TestDeclareQueue_QuorumWithExclusive(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareQueue(mock, "quorum", "exclusive-queue", true, false, true, false)

	if err != nil {
		t.Errorf("DeclareQueue() returned error: %v", err)
	}

	// Should fallback to classic when quorum is used with exclusive
	if mock.queueArgs["x-queue-type"] != "classic" {
		t.Errorf("Expected queue type 'classic' (fallback), got '%v'", mock.queueArgs["x-queue-type"])
	}
}

func TestDeclareDLQ(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareDLQ(mock, "classic", "origin-queue", "retry-exchange", 5000, 0)

	if err != nil {
		t.Errorf("DeclareDLQ() returned error: %v", err)
	}

	if !mock.queueDeclared {
		t.Error("DLQ was not declared")
	}

	expectedName := "origin-queue.DLQ"
	if mock.queueName != expectedName {
		t.Errorf("Expected DLQ name '%s', got '%s'", expectedName, mock.queueName)
	}

	if mock.queueArgs["x-dead-letter-exchange"] != "retry-exchange" {
		t.Errorf("Expected dead letter exchange 'retry-exchange', got '%v'", mock.queueArgs["x-dead-letter-exchange"])
	}

	if mock.queueArgs["x-message-ttl"] != 5000 {
		t.Errorf("Expected message TTL 5000, got '%v'", mock.queueArgs["x-message-ttl"])
	}

	if mock.queueArgs["x-queue-type"] != "classic" {
		t.Errorf("Expected queue type 'classic', got '%v'", mock.queueArgs["x-queue-type"])
	}

	if _, exists := mock.queueArgs["x-max-length-bytes"]; exists {
		t.Error("Expected no x-max-length-bytes when maxLengthBytesOpt is 0")
	}
}

func TestDeclareDLQ_WithMaxLength(t *testing.T) {
	mock := &mockChannel{}

	err := DeclareDLQ(mock, "quorum", "origin-queue", "retry-exchange", 10000, 1048576)

	if err != nil {
		t.Errorf("DeclareDLQ() returned error: %v", err)
	}

	if mock.queueArgs["x-max-length-bytes"] != 1048576 {
		t.Errorf("Expected max length bytes 1048576, got '%v'", mock.queueArgs["x-max-length-bytes"])
	}

	if mock.queueArgs["x-queue-type"] != "quorum" {
		t.Errorf("Expected queue type 'quorum', got '%v'", mock.queueArgs["x-queue-type"])
	}
}

func TestBind(t *testing.T) {
	mock := &mockChannel{}

	err := Bind(mock, "test-queue", "routing.key", "test-exchange", false)

	if err != nil {
		t.Errorf("Bind() returned error: %v", err)
	}

	bound, exists := mock.boundQueues["test-queue"]
	if !exists {
		t.Error("Queue was not bound")
	}

	if bound.name != "test-queue" {
		t.Errorf("Expected bound queue name 'test-queue', got '%s'", bound.name)
	}

	if bound.key != "routing.key" {
		t.Errorf("Expected routing key 'routing.key', got '%s'", bound.key)
	}

	if bound.exchange != "test-exchange" {
		t.Errorf("Expected exchange 'test-exchange', got '%s'", bound.exchange)
	}
}

func TestBind_MultipleQueues(t *testing.T) {
	mock := &mockChannel{}

	Bind(mock, "queue1", "key1", "exchange1", false)
	Bind(mock, "queue2", "key2", "exchange2", false)

	if len(mock.boundQueues) != 2 {
		t.Errorf("Expected 2 bound queues, got %d", len(mock.boundQueues))
	}

	if _, exists := mock.boundQueues["queue1"]; !exists {
		t.Error("queue1 was not bound")
	}

	if _, exists := mock.boundQueues["queue2"]; !exists {
		t.Error("queue2 was not bound")
	}
}

func TestUnBind(t *testing.T) {
	mock := &mockChannel{}

	err := UnBind(mock, "test-queue", "routing.key", "test-exchange")

	if err != nil {
		t.Errorf("UnBind() returned error: %v", err)
	}

	unbound, exists := mock.unboundQueues["test-queue"]
	if !exists {
		t.Error("Queue was not unbound")
	}

	if unbound.name != "test-queue" {
		t.Errorf("Expected unbound queue name 'test-queue', got '%s'", unbound.name)
	}

	if unbound.key != "routing.key" {
		t.Errorf("Expected routing key 'routing.key', got '%s'", unbound.key)
	}

	if unbound.exchange != "test-exchange" {
		t.Errorf("Expected exchange 'test-exchange', got '%s'", unbound.exchange)
	}
}

func TestUnBind_MultipleQueues(t *testing.T) {
	mock := &mockChannel{}

	UnBind(mock, "queue1", "key1", "exchange1")
	UnBind(mock, "queue2", "key2", "exchange2")

	if len(mock.unboundQueues) != 2 {
		t.Errorf("Expected 2 unbound queues, got %d", len(mock.unboundQueues))
	}

	if _, exists := mock.unboundQueues["queue1"]; !exists {
		t.Error("queue1 was not unbound")
	}

	if _, exists := mock.unboundQueues["queue2"]; !exists {
		t.Error("queue2 was not unbound")
	}
}

func TestResolveQueueType_Quorum(t *testing.T) {
	result := resolveQueueType("quorum", false, false)
	if result != "quorum" {
		t.Errorf("Expected 'quorum', got '%s'", result)
	}
}

func TestResolveQueueType_QuorumWithAutoDelete(t *testing.T) {
	result := resolveQueueType("quorum", true, false)
	if result != "classic" {
		t.Errorf("Expected 'classic' when autoDelete is true, got '%s'", result)
	}
}

func TestResolveQueueType_QuorumWithExclusive(t *testing.T) {
	result := resolveQueueType("quorum", false, true)
	if result != "classic" {
		t.Errorf("Expected 'classic' when exclusive is true, got '%s'", result)
	}
}

func TestResolveQueueType_QuorumWithBoth(t *testing.T) {
	result := resolveQueueType("quorum", true, true)
	if result != "classic" {
		t.Errorf("Expected 'classic' when both autoDelete and exclusive are true, got '%s'", result)
	}
}

func TestResolveQueueType_Classic(t *testing.T) {
	result := resolveQueueType("classic", false, false)
	if result != "classic" {
		t.Errorf("Expected 'classic', got '%s'", result)
	}
}

func TestResolveQueueType_ClassicWithAutoDelete(t *testing.T) {
	result := resolveQueueType("classic", true, false)
	if result != "classic" {
		t.Errorf("Expected 'classic', got '%s'", result)
	}
}

func TestResolveQueueType_ClassicWithExclusive(t *testing.T) {
	result := resolveQueueType("classic", false, true)
	if result != "classic" {
		t.Errorf("Expected 'classic', got '%s'", result)
	}
}

func TestResolveQueueType_EmptyString(t *testing.T) {
	result := resolveQueueType("", false, false)
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestResolveQueueType_CustomType(t *testing.T) {
	result := resolveQueueType("stream", false, false)
	if result != "stream" {
		t.Errorf("Expected 'stream', got '%s'", result)
	}
}

func TestResolveQueueType_CustomTypeWithAutoDelete(t *testing.T) {
	result := resolveQueueType("stream", true, false)
	if result != "stream" {
		t.Errorf("Expected 'stream' (only quorum is affected by autoDelete), got '%s'", result)
	}
}
