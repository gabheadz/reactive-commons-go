package rcommons

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
	amqp "github.com/rabbitmq/amqp091-go"
)

type mockRabbitClientListener struct {
	publishJsonFunc      func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error
	createChannelFunc    func(name string) (*amqp.Channel, error)
	getChannelFunc       func(name string) (*amqp.Channel, error)
	consumeOneFunc       func(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error)
	publishJsonQueryFunc func(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error
	closeFunc            func() error
}

func newMockRabbitClientListener() *mockRabbitClientListener {
	return &mockRabbitClientListener{}
}

func (m *mockRabbitClientListener) PublishJson(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
	if m.publishJsonFunc != nil {
		return m.publishJsonFunc(exchange, key, msg, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientListener) CreateChannel(name string) (*amqp.Channel, error) {
	if m.createChannelFunc != nil {
		return m.createChannelFunc(name)
	}
	return nil, nil
}

func (m *mockRabbitClientListener) GetChannel(name string) (*amqp.Channel, error) {
	if m.getChannelFunc != nil {
		return m.getChannelFunc(name)
	}
	return nil, nil
}

func (m *mockRabbitClientListener) ConsumeOne(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error) {
	if m.consumeOneFunc != nil {
		return m.consumeOneFunc(channelKey, queueName, correlationId, timeout)
	}
	return []byte("mock response"), nil
}

func (m *mockRabbitClientListener) PublishJsonQuery(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error {
	if m.publishJsonQueryFunc != nil {
		return m.publishJsonQueryFunc(exchange, key, msg, replyQ, correlationId, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientListener) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// Mock acknowledger for amqp.Delivery
type mockAcknowledger struct {
	ackCalled  bool
	nackCalled bool
	requeue    bool
}

func (m *mockAcknowledger) Ack(tag uint64, multiple bool) error {
	m.ackCalled = true
	return nil
}

func (m *mockAcknowledger) Nack(tag uint64, multiple bool, requeue bool) error {
	m.nackCalled = true
	m.requeue = requeue
	return nil
}

func (m *mockAcknowledger) Reject(tag uint64, requeue bool) error {
	return nil
}

// Helper to create a delivery with mock acknowledger
func newMockDelivery(body []byte) (amqp.Delivery, *mockAcknowledger) {
	acker := &mockAcknowledger{}
	delivery := amqp.Delivery{
		Body:         body,
		Acknowledger: acker,
	}
	return delivery, acker
}

func TestNewEventListener(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
		DomainEventsSuffix:   "subsEvents",
	}

	registry := NewRegistry()
	mockClient := newMockRabbitClientListener()

	listener := newEventListener(mockClient, domain, registry)

	if listener == nil {
		t.Fatal("newEventListener() returned nil")
	}

	if listener.domain.Name != domain.Name {
		t.Errorf("Expected domain name '%s', got '%s'", domain.Name, listener.domain.Name)
	}

	if listener.domain.DomainEventsExchange != domain.DomainEventsExchange {
		t.Errorf("Expected DomainEventsExchange '%s', got '%s'", domain.DomainEventsExchange, listener.domain.DomainEventsExchange)
	}

	if listener.registry != registry {
		t.Error("Expected registry to be set correctly")
	}

	if listener.client != mockClient {
		t.Error("Expected client to be set correctly")
	}
}

func TestRabbitEventListener_ProcessEventMessage_Success(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()
	handlerCalled := false
	var capturedEvent any

	handler := func(event any) error {
		handlerCalled = true
		capturedEvent = event
		return nil
	}

	registry.ListenEvent("user.created", handler)

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	event := DomainEvent[any]{
		Name:    "user.created",
		EventId: "event-123",
		Data:    map[string]string{"userId": "456"},
	}

	eventBytes, _ := json.Marshal(event)
	mockMsg, mockAck := newMockDelivery(eventBytes)

	listener.processEventMessage(mockMsg)

	if !handlerCalled {
		t.Error("Expected handler to be called")
	}

	if !mockAck.ackCalled {
		t.Error("Expected message to be acknowledged")
	}

	if mockAck.nackCalled {
		t.Error("Expected message not to be nacked")
	}

	eventData, ok := capturedEvent.(DomainEvent[any])
	if !ok {
		t.Fatal("Expected captured event to be DomainEvent[any]")
	}

	if eventData.Name != "user.created" {
		t.Errorf("Expected event name 'user.created', got '%s'", eventData.Name)
	}

	if eventData.EventId != "event-123" {
		t.Errorf("Expected event ID 'event-123', got '%s'", eventData.EventId)
	}
}

func TestRabbitEventListener_ProcessEventMessage_HandlerError(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()
	expectedErr := errors.New("handler error")

	handler := func(event any) error {
		return expectedErr
	}

	registry.ListenEvent("user.created", handler)

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	event := DomainEvent[any]{
		Name:    "user.created",
		EventId: "event-123",
		Data:    "test data",
	}

	eventBytes, _ := json.Marshal(event)
	mockMsg, mockAck := newMockDelivery(eventBytes)

	listener.processEventMessage(mockMsg)

	if !mockAck.nackCalled {
		t.Error("Expected message to be nacked")
	}

	if !mockAck.requeue {
		t.Error("Expected message to be requeued")
	}

	if mockAck.ackCalled {
		t.Error("Expected message not to be acknowledged")
	}
}

func TestRabbitEventListener_ProcessEventMessage_InvalidJson(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()
	handlerCalled := false

	handler := func(event any) error {
		handlerCalled = true
		return nil
	}

	registry.ListenEvent("user.created", handler)

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	invalidJson := []byte("invalid json data")
	mockMsg, mockAck := newMockDelivery(invalidJson)

	listener.processEventMessage(mockMsg)

	if handlerCalled {
		t.Error("Expected handler not to be called with invalid JSON")
	}

	if !mockAck.nackCalled {
		t.Error("Expected message to be nacked")
	}

	if mockAck.requeue {
		t.Error("Expected message not to be requeued for invalid JSON")
	}
}

func TestRabbitEventListener_ProcessEventMessage_NoHandler(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()
	// Don't register any handler

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	event := DomainEvent[any]{
		Name:    "user.created",
		EventId: "event-123",
		Data:    "test data",
	}

	eventBytes, _ := json.Marshal(event)
	mockMsg, mockAck := newMockDelivery(eventBytes)

	listener.processEventMessage(mockMsg)

	if !mockAck.nackCalled {
		t.Error("Expected message to be nacked when no handler exists")
	}

	if mockAck.requeue {
		t.Error("Expected message not to be requeued when no handler exists")
	}

	if mockAck.ackCalled {
		t.Error("Expected message not to be acknowledged when no handler exists")
	}
}

func TestRabbitEventListener_ProcessEventMessage_DifferentEventTypes(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	tests := []struct {
		name      string
		eventData any
	}{
		{
			name:      "String data",
			eventData: "test string",
		},
		{
			name:      "Integer data",
			eventData: 42,
		},
		{
			name: "Map data",
			eventData: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
			},
		},
		{
			name: "Struct data",
			eventData: struct {
				Name string
				Age  int
			}{
				Name: "John",
				Age:  30,
			},
		},
		{
			name:      "Nil data",
			eventData: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			handlerCalled := false

			handler := func(event any) error {
				handlerCalled = true
				return nil
			}

			registry.ListenEvent("test.event", handler)

			listener := newEventListener(newMockRabbitClientListener(), domain, registry)

			event := DomainEvent[any]{
				Name:    "test.event",
				EventId: "event-123",
				Data:    tt.eventData,
			}

			eventBytes, _ := json.Marshal(event)
			mockMsg, mockAck := newMockDelivery(eventBytes)

			listener.processEventMessage(mockMsg)

			if !handlerCalled {
				t.Errorf("Expected handler to be called for %s", tt.name)
			}

			if !mockAck.ackCalled {
				t.Errorf("Expected message to be acknowledged for %s", tt.name)
			}
		})
	}
}

func TestRabbitEventListener_ProcessEventMessage_MultipleEvents(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()
	processedEvents := []string{}

	handler := func(event any) error {
		evt := event.(DomainEvent[any])
		processedEvents = append(processedEvents, evt.Name)
		return nil
	}

	registry.ListenEvent("event1", handler)
	registry.ListenEvent("event2", handler)
	registry.ListenEvent("event3", handler)

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	events := []DomainEvent[any]{
		{Name: "event1", EventId: "id1", Data: "data1"},
		{Name: "event2", EventId: "id2", Data: "data2"},
		{Name: "event3", EventId: "id3", Data: "data3"},
	}

	for i, event := range events {
		eventBytes, _ := json.Marshal(event)
		mockMsg, mockAck := newMockDelivery(eventBytes)

		listener.processEventMessage(mockMsg)

		if !mockAck.ackCalled {
			t.Errorf("Expected message %d to be acknowledged", i)
		}
	}

	if len(processedEvents) != 3 {
		t.Errorf("Expected 3 processed events, got %d", len(processedEvents))
	}

	for i, eventName := range []string{"event1", "event2", "event3"} {
		if processedEvents[i] != eventName {
			t.Errorf("Expected event %d to be '%s', got '%s'", i, eventName, processedEvents[i])
		}
	}
}

func TestRabbitEventListener_RegistryIntegration(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()

	// Initially no handler
	if registry.HasEventHandler("user.created") {
		t.Error("Expected no handler to exist initially")
	}

	handler := func(event any) error {
		return nil
	}

	// Register handler
	registry.ListenEvent("user.created", handler)

	if !registry.HasEventHandler("user.created") {
		t.Error("Expected handler to exist after registration")
	}

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	event := DomainEvent[any]{
		Name:    "user.created",
		EventId: "event-123",
		Data:    "test data",
	}

	eventBytes, _ := json.Marshal(event)
	mockMsg, mockAck := newMockDelivery(eventBytes)

	listener.processEventMessage(mockMsg)

	if !mockAck.ackCalled {
		t.Error("Expected message to be acknowledged with registered handler")
	}
}

func TestRabbitEventListener_EmptyEventData(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()
	handlerCalled := false

	handler := func(event any) error {
		handlerCalled = true
		evt := event.(DomainEvent[any])
		if evt.Data != nil {
			t.Error("Expected data to be nil")
		}
		return nil
	}

	registry.ListenEvent("test.event", handler)

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	event := DomainEvent[any]{
		Name:    "test.event",
		EventId: "event-123",
		Data:    nil,
	}

	eventBytes, _ := json.Marshal(event)
	mockMsg, mockAck := newMockDelivery(eventBytes)

	listener.processEventMessage(mockMsg)

	if !handlerCalled {
		t.Error("Expected handler to be called")
	}

	if !mockAck.ackCalled {
		t.Error("Expected message to be acknowledged")
	}
}

func TestRabbitEventListener_HandlerPanic(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()

	handler := func(event any) error {
		// Return an error instead of panicking
		return errors.New("critical error")
	}

	registry.ListenEvent("test.event", handler)

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	event := DomainEvent[any]{
		Name:    "test.event",
		EventId: "event-123",
		Data:    "test data",
	}

	eventBytes, _ := json.Marshal(event)
	mockMsg, mockAck := newMockDelivery(eventBytes)

	// Should not panic
	listener.processEventMessage(mockMsg)

	if !mockAck.nackCalled {
		t.Error("Expected message to be nacked on handler error")
	}

	if !mockAck.requeue {
		t.Error("Expected message to be requeued on handler error")
	}
}

func TestRabbitEventListener_EventIdPreservation(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()
	var capturedEventId string

	handler := func(event any) error {
		evt := event.(DomainEvent[any])
		capturedEventId = evt.EventId
		return nil
	}

	registry.ListenEvent("test.event", handler)

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	expectedEventId := "unique-event-id-12345"
	event := DomainEvent[any]{
		Name:    "test.event",
		EventId: expectedEventId,
		Data:    "test data",
	}

	eventBytes, _ := json.Marshal(event)
	mockMsg, _ := newMockDelivery(eventBytes)

	listener.processEventMessage(mockMsg)

	if capturedEventId != expectedEventId {
		t.Errorf("Expected event ID '%s', got '%s'", expectedEventId, capturedEventId)
	}
}

func TestRabbitEventListener_ComplexEventData(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	registry := NewRegistry()
	var capturedData any

	handler := func(event any) error {
		evt := event.(DomainEvent[any])
		capturedData = evt.Data
		return nil
	}

	registry.ListenEvent("order.placed", handler)

	listener := newEventListener(newMockRabbitClientListener(), domain, registry)

	complexData := map[string]interface{}{
		"orderId": "order-123",
		"items": []map[string]interface{}{
			{"productId": "prod-1", "quantity": 2},
			{"productId": "prod-2", "quantity": 1},
		},
		"total":    99.99,
		"customer": map[string]string{"id": "cust-456", "name": "John Doe"},
	}

	event := DomainEvent[any]{
		Name:    "order.placed",
		EventId: "event-789",
		Data:    complexData,
	}

	eventBytes, _ := json.Marshal(event)
	mockMsg, _ := newMockDelivery(eventBytes)

	listener.processEventMessage(mockMsg)

	if capturedData == nil {
		t.Fatal("Expected captured data to not be nil")
	}

	dataMap, ok := capturedData.(map[string]interface{})
	if !ok {
		t.Fatal("Expected captured data to be a map")
	}

	if dataMap["orderId"] != "order-123" {
		t.Errorf("Expected orderId 'order-123', got '%v'", dataMap["orderId"])
	}
}

// Test to verify proper mock construction
func TestMockRabbitClientListener_Methods(t *testing.T) {
	mock := newMockRabbitClientListener()

	// Test default behavior (no-op)
	err := mock.PublishJson("", "", nil, "", nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	err = mock.Close()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test with custom function
	expectedError := errors.New("test error")
	mock.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return expectedError
	}

	err = mock.PublishJson("", "", nil, "", nil)
	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

func TestCalculateQueueName_Integration(t *testing.T) {
	// Test the queue name calculation used in listenEvent
	domain := DomainDefinition{
		Name:               "test-app",
		DomainEventsSuffix: "subsEvents",
	}

	queueName := calculateQueueName(domain.Name, domain.DomainEventsSuffix, false)
	expected := "test-app.subsEvents"

	if queueName != expected {
		t.Errorf("Expected queue name '%s', got '%s'", expected, queueName)
	}
}

func TestRabbitEventListener_DomainConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		domain DomainDefinition
	}{
		{
			name: "Basic configuration",
			domain: DomainDefinition{
				Name:                 "app1",
				DomainEventsExchange: "events.exchange",
				DomainEventsSuffix:   "events",
			},
		},
		{
			name: "Custom configuration",
			domain: DomainDefinition{
				Name:                 "app2",
				DomainEventsExchange: "custom.events",
				DomainEventsSuffix:   "subsEvents",
			},
		},
		{
			name: "Empty suffix",
			domain: DomainDefinition{
				Name:                 "app3",
				DomainEventsExchange: "events",
				DomainEventsSuffix:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			listener := newEventListener(nil, tt.domain, registry)

			if listener == nil {
				t.Fatal("newEventListener() returned nil")
			}

			if listener.domain.Name != tt.domain.Name {
				t.Errorf("Expected domain name '%s', got '%s'", tt.domain.Name, listener.domain.Name)
			}

			if listener.domain.DomainEventsExchange != tt.domain.DomainEventsExchange {
				t.Errorf("Expected DomainEventsExchange '%s', got '%s'", tt.domain.DomainEventsExchange, listener.domain.DomainEventsExchange)
			}
		})
	}
}

// Test mock delivery creation
func TestMockDelivery_Creation(t *testing.T) {
	testBody := []byte("test body")
	delivery, acker := newMockDelivery(testBody)

	if string(delivery.Body) != string(testBody) {
		t.Errorf("Expected body to be '%s', got '%s'", testBody, delivery.Body)
	}

	if acker.ackCalled {
		t.Error("Expected ackCalled to be false initially")
	}

	if acker.nackCalled {
		t.Error("Expected nackCalled to be false initially")
	}

	// Test Ack
	err := delivery.Ack(false)
	if err != nil {
		t.Errorf("Expected no error from Ack, got %v", err)
	}

	if !acker.ackCalled {
		t.Error("Expected ackCalled to be true after calling Ack")
	}

	// Test Nack
	acker2 := &mockAcknowledger{}
	delivery2 := amqp.Delivery{
		Body:         testBody,
		Acknowledger: acker2,
	}

	err = delivery2.Nack(false, true)
	if err != nil {
		t.Errorf("Expected no error from Nack, got %v", err)
	}

	if !acker2.nackCalled {
		t.Error("Expected nackCalled to be true after calling Nack")
	}

	if !acker2.requeue {
		t.Error("Expected requeue to be true")
	}
}

// Test for proper package initialization
func init() {
	// Ensure rabbit package is available
	_ = rabbit.RabbitChannel{}
}
