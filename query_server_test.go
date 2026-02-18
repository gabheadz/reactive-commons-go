package rcommons

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// mockRabbitClientServer is a mock implementation of RabbitClientInterface for testing query server
type mockRabbitClientServer struct {
	publishJsonFunc      func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error
	publishJsonQueryFunc func(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error
	consumeOneFunc       func(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error)
	createChannelFunc    func(name string) (*amqp091.Channel, error)
	getChannelFunc       func(name string) (*amqp091.Channel, error)
	closeFunc            func() error
}

func (m *mockRabbitClientServer) PublishJson(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
	if m.publishJsonFunc != nil {
		return m.publishJsonFunc(exchange, key, msg, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientServer) PublishJsonQuery(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error {
	if m.publishJsonQueryFunc != nil {
		return m.publishJsonQueryFunc(exchange, key, msg, replyQ, correlationId, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientServer) ConsumeOne(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error) {
	if m.consumeOneFunc != nil {
		return m.consumeOneFunc(channelKey, queueName, correlationId, timeout)
	}
	return []byte("mock response"), nil
}

func (m *mockRabbitClientServer) CreateChannel(name string) (*amqp091.Channel, error) {
	if m.createChannelFunc != nil {
		return m.createChannelFunc(name)
	}
	return nil, nil
}

func (m *mockRabbitClientServer) GetChannel(name string) (*amqp091.Channel, error) {
	if m.getChannelFunc != nil {
		return m.getChannelFunc(name)
	}
	return nil, errors.New("mock: channel not found")
}

func (m *mockRabbitClientServer) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestNewQueryServer(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
		GlobalExchange:    "global",
	}

	registry := NewRegistry()

	// Create a mock client
	mockClient := &mockRabbitClientServer{}

	server := newQueryServer(mockClient, domain, registry)

	if server == nil {
		t.Fatal("newQueryServer() returned nil")
	}

	if server.domain.Name != domain.Name {
		t.Errorf("Expected domain name '%s', got '%s'", domain.Name, server.domain.Name)
	}

	if server.registry != registry {
		t.Error("Registry not set correctly")
	}

	if server.domain.DirectExchange != domain.DirectExchange {
		t.Errorf("Expected DirectExchange '%s', got '%s'", domain.DirectExchange, server.domain.DirectExchange)
	}
}

func TestRabbitQueryServer_ServeQueries(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
	}

	// Create a mock client
	mockClient := &mockRabbitClientServer{}
	registry := NewRegistry()

	handlers := map[string]QueryHandler{
		"resource1": func(query any) (any, error) {
			return "result1", nil
		},
		"resource2": func(query any) (any, error) {
			return "result2", nil
		},
	}
	registry.QueryHandlers = handlers

	server := newQueryServer(mockClient, domain, registry)

	// This will fail due to nil client, but tests the structure
	// In a real scenario, this would set up consumers
	err := server.serveQueries()
	if err == nil {
		t.Errorf("Expected error for resource with nil client")
	}
}

func TestRabbitQueryServer_ServeQuery_NilClient(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
	}

	registry := NewRegistry()
	// Create a mock client
	mockClient := &mockRabbitClientServer{}

	handler := func(query any) (any, error) {
		return "result", nil
	}
	registry.ServeQuery("test-resource", handler)

	server := newQueryServer(mockClient, domain, registry)

	err := server.serveQueries()

	if err == nil {
		t.Error("Expected error when client is nil, got nil")
	}
}

func TestRabbitQueryServer_ProcessQueryMessage_ValidQuery(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
		GlobalExchange:    "global",
	}

	registry := NewRegistry()
	handlerCalled := false
	expectedResult := "test result"

	handler := func(query any) (any, error) {
		handlerCalled = true
		return expectedResult, nil
	}

	registry.ServeQuery("test-resource", handler)

	// Create a mock client
	mockClient := &mockRabbitClientServer{}
	server := newQueryServer(mockClient, domain, registry)

	// Create a mock query message
	query := AsyncQuery[any]{
		Resource:  "test-resource",
		QueryData: "test data",
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	// Create mock delivery
	msg := amqp091.Delivery{
		Body:          queryBytes,
		ReplyTo:       "reply-queue",
		CorrelationId: "correlation-123",
	}

	// Process the message (will fail on reply due to nil client)
	server.processQueryMessage(msg)

	// Verify handler was called
	if !handlerCalled {
		t.Error("Handler should have been called")
	}
}

func TestRabbitQueryServer_ProcessQueryMessage_InvalidJSON(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
	}

	registry := NewRegistry()

	// Create a mock client
	mockClient := &mockRabbitClientServer{}
	server := newQueryServer(mockClient, domain, registry)

	// Create mock delivery with invalid JSON
	msg := amqp091.Delivery{
		Body:          []byte("invalid json"),
		ReplyTo:       "reply-queue",
		CorrelationId: "correlation-123",
	}

	// Process the message - should handle error gracefully
	server.processQueryMessage(msg)

	// Test passes if no panic occurs
}

func TestRabbitQueryServer_ProcessQueryMessage_NoHandler(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
	}

	registry := NewRegistry()
	// Create a mock client
	mockClient := &mockRabbitClientServer{}
	server := newQueryServer(mockClient, domain, registry)

	// Create a query for a resource with no handler
	query := AsyncQuery[any]{
		Resource:  "non-existent-resource",
		QueryData: "test data",
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	msg := amqp091.Delivery{
		Body:          queryBytes,
		ReplyTo:       "reply-queue",
		CorrelationId: "correlation-123",
	}

	// Process the message - should handle missing handler gracefully
	server.processQueryMessage(msg)

	// Test passes if no panic occurs
}

func TestRabbitQueryServer_ProcessQueryMessage_HandlerError(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
	}

	registry := NewRegistry()
	expectedError := errors.New("handler error")

	handler := func(query any) (any, error) {
		return nil, expectedError
	}

	registry.ServeQuery("error-resource", handler)

	// Create a mock client
	mockClient := &mockRabbitClientServer{}
	server := newQueryServer(mockClient, domain, registry)

	query := AsyncQuery[any]{
		Resource:  "error-resource",
		QueryData: "test data",
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	msg := amqp091.Delivery{
		Body:          queryBytes,
		ReplyTo:       "reply-queue",
		CorrelationId: "correlation-123",
	}

	// Process the message - should handle handler error gracefully
	server.processQueryMessage(msg)

	// Test passes if no panic occurs
}

func TestRabbitQueryServer_SendReply_NilClient(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		GlobalExchange: "global",
	}

	registry := NewRegistry()

	// Create a mock that returns an error on PublishJson
	mockClient := &mockRabbitClientServer{
		publishJsonFunc: func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
			return errors.New("mock: publish failed")
		},
	}

	server := newQueryServer(mockClient, domain, registry)

	err := server.sendReply("reply-queue", "correlation-123", "test response")

	if err == nil {
		t.Error("Expected error from mock client, got nil")
	}

	if err.Error() != "mock: publish failed" {
		t.Errorf("Expected 'mock: publish failed', got '%s'", err.Error())
	}
}

func TestRabbitQueryServer_DomainConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		domain DomainDefinition
	}{
		{
			name: "Basic configuration",
			domain: DomainDefinition{
				Name:              "app1",
				DirectExchange:    "directMessages",
				DirectQuerySuffix: "query",
				GlobalExchange:    "globalReply",
			},
		},
		{
			name: "Custom configuration",
			domain: DomainDefinition{
				Name:              "app2",
				DirectExchange:    "custom.direct",
				DirectQuerySuffix: "custom.query",
				GlobalExchange:    "custom.global",
			},
		},
		{
			name: "Minimal configuration",
			domain: DomainDefinition{
				Name:              "app3",
				DirectExchange:    "direct",
				DirectQuerySuffix: "",
				GlobalExchange:    "global",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			server := newQueryServer(nil, tt.domain, registry)

			if server == nil {
				t.Fatal("newQueryServer() returned nil")
			}

			if server.domain.Name != tt.domain.Name {
				t.Errorf("Expected domain name '%s', got '%s'", tt.domain.Name, server.domain.Name)
			}

			if server.domain.DirectExchange != tt.domain.DirectExchange {
				t.Errorf("Expected DirectExchange '%s', got '%s'", tt.domain.DirectExchange, server.domain.DirectExchange)
			}

			if server.domain.GlobalExchange != tt.domain.GlobalExchange {
				t.Errorf("Expected GlobalExchange '%s', got '%s'", tt.domain.GlobalExchange, server.domain.GlobalExchange)
			}
		})
	}
}

func TestRabbitQueryServer_MultipleHandlers(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
	}

	registry := NewRegistry()

	// Register multiple handlers
	handler1Called := false
	handler2Called := false
	handler3Called := false

	registry.ServeQuery("resource1", func(query any) (any, error) {
		handler1Called = true
		return "result1", nil
	})

	registry.ServeQuery("resource2", func(query any) (any, error) {
		handler2Called = true
		return "result2", nil
	})

	registry.ServeQuery("resource3", func(query any) (any, error) {
		handler3Called = true
		return "result3", nil
	})

	server := newQueryServer(nil, domain, registry)

	if len(registry.QueryHandlers) != 3 {
		t.Errorf("Expected 3 handlers, got %d", len(registry.QueryHandlers))
	}

	// Test each handler
	for resource, handler := range registry.QueryHandlers {
		result, err := handler("test")
		if err != nil {
			t.Errorf("Handler for %s returned error: %v", resource, err)
		}
		if result == nil {
			t.Errorf("Handler for %s returned nil result", resource)
		}
	}

	if !handler1Called || !handler2Called || !handler3Called {
		t.Error("Not all handlers were called")
	}

	if server.registry != registry {
		t.Error("Server registry not set correctly")
	}
}

func TestRabbitQueryServer_ResponseDataTypes(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
		GlobalExchange:    "global",
	}

	registry := NewRegistry()

	tests := []struct {
		name         string
		responseData any
	}{
		{
			name:         "String response",
			responseData: "test string",
		},
		{
			name:         "Integer response",
			responseData: 42,
		},
		{
			name: "Map response",
			responseData: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
			},
		},
		{
			name: "Struct response",
			responseData: struct {
				Name   string
				Status string
			}{
				Name:   "John",
				Status: "active",
			},
		},
		{
			name:         "Nil response",
			responseData: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock client
			mockClient := &mockRabbitClientServer{}
			server := newQueryServer(mockClient, domain, registry)

			// Test that response data can be marshaled
			_, err := json.Marshal(tt.responseData)
			if err != nil {
				t.Errorf("Failed to marshal response data: %v", err)
			}

			// Test sendReply with different data types
			err = server.sendReply("reply-queue", "correlation-123", tt.responseData)
			if err != nil {
				t.Error("Expected no error, got err")
			}
		})
	}
}

func TestRabbitQueryServer_QueueNameCalculation(t *testing.T) {
	tests := []struct {
		name              string
		appName           string
		directQuerySuffix string
	}{
		{
			name:              "Standard suffix",
			appName:           "myapp",
			directQuerySuffix: "query",
		},
		{
			name:              "Custom suffix",
			appName:           "service1",
			directQuerySuffix: "custom.query",
		},
		{
			name:              "Empty suffix",
			appName:           "app2",
			directQuerySuffix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueName := calculateQueueName(tt.appName, tt.directQuerySuffix, false)

			if queueName == "" {
				t.Error("Queue name should not be empty")
			}

			// Queue name should contain the app name
			if len(queueName) < len(tt.appName) {
				t.Errorf("Queue name '%s' is shorter than app name '%s'", queueName, tt.appName)
			}
		})
	}
}

func TestRabbitQueryServer_RegistryUpdate(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
	}

	registry1 := NewRegistry()
	registry2 := NewRegistry()

	server := newQueryServer(nil, domain, registry1)

	if server.registry != registry1 {
		t.Error("Initial registry not set correctly")
	}

	// Update registry
	server.registry = registry2

	if server.registry != registry2 {
		t.Error("Registry not updated correctly")
	}

	if server.registry == registry1 {
		t.Error("Registry should have changed")
	}
}

func TestRabbitQueryServer_ConcurrentHandlers(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
	}

	registry := NewRegistry()

	// Register handlers that could be called concurrently
	for i := 0; i < 10; i++ {
		resource := "resource-" + string(rune(i+'0'))
		registry.ServeQuery(resource, func(query any) (any, error) {
			return "result", nil
		})
	}

	// Create a mock client
	mockClient := &mockRabbitClientServer{}
	server := newQueryServer(mockClient, domain, registry)

	if len(registry.QueryHandlers) != 10 {
		t.Errorf("Expected 10 handlers, got %d", len(registry.QueryHandlers))
	}

	if server.registry != registry {
		t.Error("Registry not set correctly")
	}
}

func TestRabbitQueryServer_EmptyRegistry(t *testing.T) {
	domain := DomainDefinition{
		Name:              "test-app",
		DirectExchange:    "direct",
		DirectQuerySuffix: "query",
	}

	registry := NewRegistry()
	// Create a mock client
	mockClient := &mockRabbitClientServer{}
	server := newQueryServer(mockClient, domain, registry)

	if server == nil {
		t.Fatal("Server should not be nil with empty registry")
	}

	if len(registry.QueryHandlers) != 0 {
		t.Errorf("Expected 0 handlers, got %d", len(registry.QueryHandlers))
	}
}
