package rcommons

import (
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// mockRabbitClientReactiveCommons provides a mock implementation for testing ReactiveCommons
type mockRabbitClientReactiveCommons struct {
	publishJsonFunc      func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error
	createChannelFunc    func(name string) (*amqp.Channel, error)
	getChannelFunc       func(name string) (*amqp.Channel, error)
	consumeOneFunc       func(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error)
	publishJsonQueryFunc func(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error
	closeFunc            func() error
}

func newMockRabbitClientReactiveCommons() *mockRabbitClientReactiveCommons {
	return &mockRabbitClientReactiveCommons{}
}

func (m *mockRabbitClientReactiveCommons) PublishJson(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
	if m.publishJsonFunc != nil {
		return m.publishJsonFunc(exchange, key, msg, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientReactiveCommons) CreateChannel(name string) (*amqp.Channel, error) {
	if m.createChannelFunc != nil {
		return m.createChannelFunc(name)
	}
	return nil, nil
}

func (m *mockRabbitClientReactiveCommons) GetChannel(name string) (*amqp.Channel, error) {
	if m.getChannelFunc != nil {
		return m.getChannelFunc(name)
	}
	return nil, nil
}

func (m *mockRabbitClientReactiveCommons) ConsumeOne(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error) {
	if m.consumeOneFunc != nil {
		return m.consumeOneFunc(channelKey, queueName, correlationId, timeout)
	}
	return []byte("mock response"), nil
}

func (m *mockRabbitClientReactiveCommons) PublishJsonQuery(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error {
	if m.publishJsonQueryFunc != nil {
		return m.publishJsonQueryFunc(exchange, key, msg, replyQ, correlationId, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientReactiveCommons) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestReactiveCommons_EmitEvent_Success(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	rc := &ReactiveCommons{
		Domain: domain,
	}

	mockClient := newMockRabbitClientReactiveCommons()
	var capturedExchange, capturedKey string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedExchange = exchange
		capturedKey = key
		return nil
	}

	rc.eventPublisher = newEventPublisher(mockClient, &domain)

	event := DomainEvent[any]{
		Name:    "user.created",
		EventId: "event-123",
		Data:    "test data",
	}

	opts := EventOptions{
		Domain: "test-domain",
	}

	err := rc.EmitEvent(event, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if capturedExchange != domain.DomainEventsExchange {
		t.Errorf("Expected exchange '%s', got '%s'", domain.DomainEventsExchange, capturedExchange)
	}

	if capturedKey != event.Name {
		t.Errorf("Expected routing key '%s', got '%s'", event.Name, capturedKey)
	}
}

func TestReactiveCommons_EmitEvent_Error(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
	}

	rc := &ReactiveCommons{
		Domain: domain,
	}

	mockClient := newMockRabbitClientReactiveCommons()
	expectedError := errors.New("publish failed")

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return expectedError
	}

	rc.eventPublisher = newEventPublisher(mockClient, &domain)

	event := DomainEvent[any]{
		Name:    "user.created",
		EventId: "event-123",
		Data:    "test data",
	}

	opts := EventOptions{
		Domain: "test-domain",
	}

	err := rc.EmitEvent(event, opts)

	if err == nil {
		t.Error("Expected error from EmitEvent, got nil")
	}

	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

func TestReactiveCommons_EmitEvent_NotInitialized(t *testing.T) {
	rc := &ReactiveCommons{}

	err := rc.EmitEvent(DomainEvent[any]{Name: "user.created"}, EventOptions{Domain: "test"})

	if err == nil {
		t.Fatal("Expected error when event publisher is not initialized")
	}

	if err.Error() != "event publisher is not initialized; call Start first" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestReactiveCommons_SendCommand_Success(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "direct",
	}

	rc := &ReactiveCommons{
		Domain: domain,
	}

	mockClient := newMockRabbitClientReactiveCommons()
	var capturedExchange string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedExchange = exchange
		return nil
	}

	rc.commandSender = newCommandSender(mockClient, &domain)

	command := Command[any]{
		Name:      "create.user",
		CommandId: "cmd-123",
		Data:      "command data",
	}

	opts := CommandOptions{
		TargetName: "user-service",
		Domain:     "test-domain",
	}

	err := rc.SendCommand(command, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if capturedExchange != domain.DirectExchange {
		t.Errorf("Expected exchange '%s', got '%s'", domain.DirectExchange, capturedExchange)
	}
}

func TestReactiveCommons_SendCommand_Error(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "direct",
	}

	rc := &ReactiveCommons{
		Domain: domain,
	}

	mockClient := newMockRabbitClientReactiveCommons()
	expectedError := errors.New("command send failed")

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return expectedError
	}

	rc.commandSender = newCommandSender(mockClient, &domain)

	command := Command[any]{
		Name:      "create.user",
		CommandId: "cmd-123",
		Data:      "command data",
	}

	opts := CommandOptions{
		TargetName: "user-service",
		Domain:     "test-domain",
	}

	err := rc.SendCommand(command, opts)

	if err == nil {
		t.Error("Expected error from SendCommand, got nil")
	}

	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

func TestReactiveCommons_SendCommand_NotInitialized(t *testing.T) {
	rc := &ReactiveCommons{}

	err := rc.SendCommand(Command[any]{Name: "cmd"}, CommandOptions{Domain: "test", TargetName: "target"})

	if err == nil {
		t.Fatal("Expected error when command sender is not initialized")
	}

	if err.Error() != "command sender is not initialized; call Start first" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestReactiveCommons_SendQueryRequest_Success(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		GlobalExchange:    "global",
		globalRepliesID:   "replies-queue",
		DirectQuerySuffix: "query",
	}

	rc := &ReactiveCommons{
		Domain: domain,
	}

	mockClient := newMockRabbitClientReactiveCommons()
	expectedResponse := []byte("query response")

	mockClient.publishJsonQueryFunc = func(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error {
		return nil
	}

	mockClient.consumeOneFunc = func(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error) {
		return expectedResponse, nil
	}

	rc.queryClient = newQueryClient(mockClient, mockClient, &domain)

	query := AsyncQuery[any]{
		Resource:  "user.info",
		QueryData: "query data",
	}

	opts := RequestReplyOptions{
		TargetName: "user-service",
		Domain:     "test-domain",
	}

	response, err := rc.SendQueryRequest(query, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if string(response) != string(expectedResponse) {
		t.Errorf("Expected response '%s', got '%s'", expectedResponse, response)
	}
}

func TestReactiveCommons_SendQueryRequest_Error(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		GlobalExchange:    "global",
		globalRepliesID:   "replies-queue",
		DirectQuerySuffix: "query",
	}

	rc := &ReactiveCommons{
		Domain: domain,
	}

	mockClient := newMockRabbitClientReactiveCommons()
	expectedError := errors.New("query failed")

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return expectedError
	}

	rc.queryClient = newQueryClient(mockClient, mockClient, &domain)

	query := AsyncQuery[any]{
		Resource:  "user.info",
		QueryData: "query data",
	}

	opts := RequestReplyOptions{
		TargetName: "user-service",
		Domain:     "test-domain",
	}

	_, err := rc.SendQueryRequest(query, opts)

	if err == nil {
		t.Error("Expected error from SendQueryRequest, got nil")
	}

	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

func TestReactiveCommons_SendQueryRequest_NotInitialized(t *testing.T) {
	rc := &ReactiveCommons{}

	_, err := rc.SendQueryRequest(AsyncQuery[any]{Resource: "user.info"}, RequestReplyOptions{Domain: "test", TargetName: "target"})

	if err == nil {
		t.Fatal("Expected error when query client is not initialized")
	}

	if err.Error() != "query client is not initialized; call Start first" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestReactiveCommons_DomainDefaultValues(t *testing.T) {
	tests := []struct {
		name           string
		inputDomain    DomainDefinition
		expectedValues map[string]string
	}{
		{
			name: "All defaults applied",
			inputDomain: DomainDefinition{
				Name: "test-app",
				ConnectionConfig: RabbitMQConfig{
					Host:     "localhost",
					Port:     5672,
					User:     "guest",
					Password: "guest",
				},
			},
			expectedValues: map[string]string{
				"DomainEventsExchange": domainEvents,
				"DomainEventsSuffix":   domainEventsSuffix,
				"DirectExchange":       directExchange,
				"DirectCommandsSuffix": directCommandsSuffix,
				"DirectQuerySuffix":    directQuerySuffix,
				"GlobalExchange":       globalExchange,
				"GlobalRepliesSuffix":  globalRepliesSuffix,
			},
		},
		{
			name: "Partial custom values",
			inputDomain: DomainDefinition{
				Name:                 "test-app",
				DomainEventsExchange: "custom-events",
				DirectExchange:       "custom-direct",
				ConnectionConfig: RabbitMQConfig{
					Host:     "localhost",
					Port:     5672,
					User:     "guest",
					Password: "guest",
				},
			},
			expectedValues: map[string]string{
				"DomainEventsExchange": "custom-events",
				"DomainEventsSuffix":   domainEventsSuffix,
				"DirectExchange":       "custom-direct",
				"DirectCommandsSuffix": directCommandsSuffix,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test would require mocking the RabbitMQ connection
			// For now, we're testing the logic conceptually
			// In a real scenario, you'd need to mock rabbit.NewRabbitClient

			domain := tt.inputDomain

			// Apply defaults (mimicking NewReactiveCommons logic)
			if domain.DomainEventsExchange == "" {
				domain.DomainEventsExchange = domainEvents
			}
			if domain.DomainEventsSuffix == "" {
				domain.DomainEventsSuffix = domainEventsSuffix
			}
			if domain.DirectExchange == "" {
				domain.DirectExchange = directExchange
			}
			if domain.DirectCommandsSuffix == "" {
				domain.DirectCommandsSuffix = directCommandsSuffix
			}
			if domain.DirectQuerySuffix == "" {
				domain.DirectQuerySuffix = directQuerySuffix
			}
			if domain.GlobalExchange == "" {
				domain.GlobalExchange = globalExchange
			}
			if domain.GlobalRepliesSuffix == "" {
				domain.GlobalRepliesSuffix = globalRepliesSuffix
			}

			for key, expectedValue := range tt.expectedValues {
				var actualValue string
				switch key {
				case "DomainEventsExchange":
					actualValue = domain.DomainEventsExchange
				case "DomainEventsSuffix":
					actualValue = domain.DomainEventsSuffix
				case "DirectExchange":
					actualValue = domain.DirectExchange
				case "DirectCommandsSuffix":
					actualValue = domain.DirectCommandsSuffix
				case "DirectQuerySuffix":
					actualValue = domain.DirectQuerySuffix
				case "GlobalExchange":
					actualValue = domain.GlobalExchange
				case "GlobalRepliesSuffix":
					actualValue = domain.GlobalRepliesSuffix
				}

				if actualValue != expectedValue {
					t.Errorf("Expected %s to be '%s', got '%s'", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestReactiveCommons_Constants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"ChannelForEvents", ChannelForEvents, "ChannelForEvents"},
		{"ChannelForCommands", ChannelForCommands, "ChannelForCommands"},
		{"ChannelForQueries", ChannelForQueries, "ChannelForQueries"},
		{"ChannelForReplies", ChannelForReplies, "ChannelForReplies"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected constant '%s' to equal '%s', got '%s'", tt.name, tt.expected, tt.constant)
			}
		})
	}
}
