package rcommons

import (
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type mockRabbitClientPublisher struct {
	publishJsonFunc      func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error
	createChannelFunc    func(name string) (*amqp.Channel, error)
	getChannelFunc       func(name string) (*amqp.Channel, error)
	consumeOneFunc       func(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error)
	publishJsonQueryFunc func(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error
	closeFunc            func() error
}

func newMockRabbitClientPublisher() *mockRabbitClientPublisher {
	return &mockRabbitClientPublisher{}
}

func (m *mockRabbitClientPublisher) PublishJson(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
	if m.publishJsonFunc != nil {
		return m.publishJsonFunc(exchange, key, msg, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientPublisher) CreateChannel(name string) (*amqp.Channel, error) {
	if m.createChannelFunc != nil {
		return m.createChannelFunc(name)
	}
	return nil, nil
}

func (m *mockRabbitClientPublisher) GetChannel(name string) (*amqp.Channel, error) {
	if m.getChannelFunc != nil {
		return m.getChannelFunc(name)
	}
	return nil, nil
}

func (m *mockRabbitClientPublisher) ConsumeOne(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error) {
	if m.consumeOneFunc != nil {
		return m.consumeOneFunc(channelKey, queueName, correlationId, timeout)
	}
	return []byte("mock response"), nil
}

func (m *mockRabbitClientPublisher) PublishJsonQuery(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error {
	if m.publishJsonQueryFunc != nil {
		return m.publishJsonQueryFunc(exchange, key, msg, replyQ, correlationId, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientPublisher) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestNewEventPublisher(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	publisher := newEventPublisher(nil, domain)

	if publisher == nil {
		t.Fatal("newEventPublisher() returned nil")
	}

	if publisher.domain.Name != domain.Name {
		t.Errorf("Expected domain name '%s', got '%s'", domain.Name, publisher.domain.Name)
	}

	if publisher.domain.DomainEventsExchange != domain.DomainEventsExchange {
		t.Errorf("Expected DomainEventsExchange '%s', got '%s'", domain.DomainEventsExchange, publisher.domain.DomainEventsExchange)
	}
}

func TestRabbitEventPublisher_EmitEvent_MissingEventName(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	publisher := newEventPublisher(newMockRabbitClientPublisher(), domain)

	event := DomainEvent[any]{
		Name:    "",
		EventId: "event-123",
		Data:    "test data",
	}

	opts := EventOptions{
		Domain: "test-domain",
	}

	err := publisher.emitEvent(event, opts)

	if err == nil {
		t.Error("Expected error for missing event name, got nil")
	}

	expectedMsg := "event name and domain are required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRabbitEventPublisher_EmitEvent_MissingDomain(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	publisher := newEventPublisher(newMockRabbitClientPublisher(), domain)

	event := DomainEvent[any]{
		Name:    "user.created",
		EventId: "event-123",
		Data:    "test data",
	}

	opts := EventOptions{
		Domain: "",
	}

	err := publisher.emitEvent(event, opts)

	if err == nil {
		t.Error("Expected error for missing domain, got nil")
	}

	expectedMsg := "event name and domain are required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRabbitEventPublisher_EmitEvent_Success(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	mockClient := newMockRabbitClientPublisher()
	var capturedExchange, capturedKey, capturedChannelKey string
	var capturedHeaders map[string]string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedExchange = exchange
		capturedKey = key
		capturedChannelKey = channelKey
		capturedHeaders = headers
		return nil
	}

	publisher := newEventPublisher(mockClient, domain)

	event := DomainEvent[any]{
		Name:    "user.created",
		EventId: "event-123",
		Data:    map[string]string{"userId": "456"},
	}

	opts := EventOptions{
		Domain: "test-domain",
	}

	err := publisher.emitEvent(event, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if capturedExchange != domain.DomainEventsExchange {
		t.Errorf("Expected exchange '%s', got '%s'", domain.DomainEventsExchange, capturedExchange)
	}

	if capturedKey != event.Name {
		t.Errorf("Expected routing key '%s', got '%s'", event.Name, capturedKey)
	}

	if capturedChannelKey != ChannelForEvents {
		t.Errorf("Expected channel key '%s', got '%s'", ChannelForEvents, capturedChannelKey)
	}

	if capturedHeaders["sourceApplication"] != domain.Name {
		t.Errorf("Expected sourceApplication '%s', got '%s'", domain.Name, capturedHeaders["sourceApplication"])
	}

	if capturedHeaders["message_id"] != event.EventId {
		t.Errorf("Expected message_id '%s', got '%s'", event.EventId, capturedHeaders["message_id"])
	}

	if capturedHeaders["delivery_mode"] != "persistent" {
		t.Errorf("Expected delivery_mode 'persistent', got '%s'", capturedHeaders["delivery_mode"])
	}

	if capturedHeaders["app_id"] != domain.Name {
		t.Errorf("Expected app_id '%s', got '%s'", domain.Name, capturedHeaders["app_id"])
	}

	if capturedHeaders["timestamp"] == "" {
		t.Error("Expected timestamp header to be set")
	}
}

func TestRabbitEventPublisher_EmitEvent_PublishError(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	mockClient := newMockRabbitClientPublisher()
	expectedError := errors.New("publish failed")

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return expectedError
	}

	publisher := newEventPublisher(mockClient, domain)

	event := DomainEvent[any]{
		Name:    "user.created",
		EventId: "event-123",
		Data:    "test data",
	}

	opts := EventOptions{
		Domain: "test-domain",
	}

	err := publisher.emitEvent(event, opts)

	if err == nil {
		t.Error("Expected error from publish, got nil")
	}

	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

func TestRabbitEventPublisher_EmitEvent_DifferentDataTypes(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	mockClient := newMockRabbitClientPublisher()
	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return nil
	}

	publisher := newEventPublisher(mockClient, domain)

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
			event := DomainEvent[any]{
				Name:    "test.event",
				EventId: "event-123",
				Data:    tt.eventData,
			}

			opts := EventOptions{
				Domain: "test-domain",
			}

			err := publisher.emitEvent(event, opts)

			if err != nil {
				t.Errorf("Expected no error for %s, got: %v", tt.name, err)
			}
		})
	}
}

func TestRabbitEventPublisher_EmitEvent_HeadersValidation(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "my-service",
		DomainEventsExchange: "domain.events",
	}

	mockClient := newMockRabbitClientPublisher()
	var capturedHeaders map[string]string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedHeaders = headers
		return nil
	}

	publisher := newEventPublisher(mockClient, domain)

	event := DomainEvent[any]{
		Name:    "order.placed",
		EventId: "order-789",
		Data:    "order data",
	}

	opts := EventOptions{
		Domain: "orders",
	}

	err := publisher.emitEvent(event, opts)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	requiredHeaders := []string{"sourceApplication", "delivery_mode", "message_id", "timestamp", "app_id"}
	for _, header := range requiredHeaders {
		if _, exists := capturedHeaders[header]; !exists {
			t.Errorf("Expected header '%s' to be present", header)
		}
	}

	if capturedHeaders["sourceApplication"] != "my-service" {
		t.Errorf("Expected sourceApplication 'my-service', got '%s'", capturedHeaders["sourceApplication"])
	}

	if capturedHeaders["app_id"] != "my-service" {
		t.Errorf("Expected app_id 'my-service', got '%s'", capturedHeaders["app_id"])
	}

	if capturedHeaders["message_id"] != "order-789" {
		t.Errorf("Expected message_id 'order-789', got '%s'", capturedHeaders["message_id"])
	}

	if capturedHeaders["delivery_mode"] != "persistent" {
		t.Errorf("Expected delivery_mode 'persistent', got '%s'", capturedHeaders["delivery_mode"])
	}
}

func TestRabbitEventPublisher_EmitEvent_MultipleEvents(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	mockClient := newMockRabbitClientPublisher()
	publishCount := 0

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		publishCount++
		return nil
	}

	publisher := newEventPublisher(mockClient, domain)

	events := []DomainEvent[any]{
		{Name: "event1", EventId: "id1", Data: "data1"},
		{Name: "event2", EventId: "id2", Data: "data2"},
		{Name: "event3", EventId: "id3", Data: "data3"},
	}

	opts := EventOptions{Domain: "test-domain"}

	for _, event := range events {
		err := publisher.emitEvent(event, opts)
		if err != nil {
			t.Errorf("Expected no error for event %s, got: %v", event.Name, err)
		}
	}

	if publishCount != len(events) {
		t.Errorf("Expected %d publish calls, got %d", len(events), publishCount)
	}
}

func TestRabbitEventPublisher_DomainConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		domain DomainDefinition
	}{
		{
			name: "Basic configuration",
			domain: DomainDefinition{
				Name:                 "app1",
				DomainEventsExchange: "events.exchange",
			},
		},
		{
			name: "Custom configuration",
			domain: DomainDefinition{
				Name:                 "app2",
				DomainEventsExchange: "custom.events",
			},
		},
		{
			name: "Empty exchange",
			domain: DomainDefinition{
				Name:                 "app3",
				DomainEventsExchange: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := newEventPublisher(nil, tt.domain)

			if publisher == nil {
				t.Fatal("newEventPublisher() returned nil")
			}

			if publisher.domain.Name != tt.domain.Name {
				t.Errorf("Expected domain name '%s', got '%s'", tt.domain.Name, publisher.domain.Name)
			}

			if publisher.domain.DomainEventsExchange != tt.domain.DomainEventsExchange {
				t.Errorf("Expected DomainEventsExchange '%s', got '%s'", tt.domain.DomainEventsExchange, publisher.domain.DomainEventsExchange)
			}
		})
	}
}
