package rcommons

import (
	"errors"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	if registry.EventHandlers == nil {
		t.Error("EventHandlers map not initialized")
	}

	if registry.CommandHandlers == nil {
		t.Error("CommandHandlers map not initialized")
	}

	if registry.QueryHandlers == nil {
		t.Error("QueryHandlers map not initialized")
	}

	if len(registry.EventHandlers) != 0 {
		t.Errorf("Expected empty EventHandlers, got %d entries", len(registry.EventHandlers))
	}

	if len(registry.CommandHandlers) != 0 {
		t.Errorf("Expected empty CommandHandlers, got %d entries", len(registry.CommandHandlers))
	}

	if len(registry.QueryHandlers) != 0 {
		t.Errorf("Expected empty QueryHandlers, got %d entries", len(registry.QueryHandlers))
	}
}

func TestListenEvent(t *testing.T) {
	registry := NewRegistry()
	called := false

	handler := func(event any) error {
		called = true
		return nil
	}

	registry.ListenEvent("test.event", handler)

	if len(registry.EventHandlers) != 1 {
		t.Errorf("Expected 1 event handler, got %d", len(registry.EventHandlers))
	}

	if _, exists := registry.EventHandlers["test.event"]; !exists {
		t.Error("Event handler not registered with correct name")
	}

	// Test handler execution
	err := registry.EventHandlers["test.event"]("test data")
	if err != nil {
		t.Errorf("Handler returned error: %v", err)
	}

	if !called {
		t.Error("Handler was not called")
	}
}

func TestListenEventTyped(t *testing.T) {
	registry := NewRegistry()
	var receivedEvent string

	handler := func(event string) error {
		receivedEvent = event
		return nil
	}

	ListenEventTyped(registry, "typed.event", handler)

	if len(registry.EventHandlers) != 1 {
		t.Errorf("Expected 1 event handler, got %d", len(registry.EventHandlers))
	}

	// Test with correct type
	err := registry.EventHandlers["typed.event"]("hello")
	if err != nil {
		t.Errorf("Handler returned error: %v", err)
	}

	if receivedEvent != "hello" {
		t.Errorf("Expected 'hello', got '%s'", receivedEvent)
	}

	// Test with incorrect type
	err = registry.EventHandlers["typed.event"](123)
	if err == nil {
		t.Error("Expected error for type mismatch, got nil")
	}
}

func TestListenEvents(t *testing.T) {
	registry := NewRegistry()

	handlers := map[string]EventHandler{
		"event1": func(event any) error { return nil },
		"event2": func(event any) error { return nil },
		"event3": func(event any) error { return nil },
	}

	registry.ListenEvents(handlers)

	if len(registry.EventHandlers) != 3 {
		t.Errorf("Expected 3 event handlers, got %d", len(registry.EventHandlers))
	}

	for name := range handlers {
		if _, exists := registry.EventHandlers[name]; !exists {
			t.Errorf("Event handler '%s' not registered", name)
		}
	}
}

func TestHandleCommand(t *testing.T) {
	registry := NewRegistry()
	called := false

	handler := func(command any) error {
		called = true
		return nil
	}

	registry.HandleCommand("test.command", handler)

	if len(registry.CommandHandlers) != 1 {
		t.Errorf("Expected 1 command handler, got %d", len(registry.CommandHandlers))
	}

	if _, exists := registry.CommandHandlers["test.command"]; !exists {
		t.Error("Command handler not registered with correct name")
	}

	// Test handler execution
	err := registry.CommandHandlers["test.command"]("test command")
	if err != nil {
		t.Errorf("Handler returned error: %v", err)
	}

	if !called {
		t.Error("Handler was not called")
	}
}

func TestHandleCommands(t *testing.T) {
	registry := NewRegistry()

	handlers := map[string]CommandHandler{
		"command1": func(command any) error { return nil },
		"command2": func(command any) error { return nil },
	}

	registry.HandleCommands(handlers)

	if len(registry.CommandHandlers) != 2 {
		t.Errorf("Expected 2 command handlers, got %d", len(registry.CommandHandlers))
	}

	for name := range handlers {
		if _, exists := registry.CommandHandlers[name]; !exists {
			t.Errorf("Command handler '%s' not registered", name)
		}
	}
}

func TestServeQuery(t *testing.T) {
	registry := NewRegistry()
	called := false

	handler := func(query any) (any, error) {
		called = true
		return "result", nil
	}

	registry.ServeQuery("test.query", handler)

	if len(registry.QueryHandlers) != 1 {
		t.Errorf("Expected 1 query handler, got %d", len(registry.QueryHandlers))
	}

	if _, exists := registry.QueryHandlers["test.query"]; !exists {
		t.Error("Query handler not registered with correct name")
	}

	// Test handler execution
	result, err := registry.QueryHandlers["test.query"]("test query")
	if err != nil {
		t.Errorf("Handler returned error: %v", err)
	}

	if result != "result" {
		t.Errorf("Expected 'result', got '%v'", result)
	}

	if !called {
		t.Error("Handler was not called")
	}
}

func TestServeQueries(t *testing.T) {
	registry := NewRegistry()

	handlers := map[string]QueryHandler{
		"query1": func(query any) (any, error) { return nil, nil },
		"query2": func(query any) (any, error) { return nil, nil },
	}

	registry.ServeQueries(handlers)

	if len(registry.QueryHandlers) != 2 {
		t.Errorf("Expected 2 query handlers, got %d", len(registry.QueryHandlers))
	}

	for name := range handlers {
		if _, exists := registry.QueryHandlers[name]; !exists {
			t.Errorf("Query handler '%s' not registered", name)
		}
	}
}

func TestHandlerOverwrite(t *testing.T) {
	registry := NewRegistry()

	firstCalled := false
	secondCalled := false

	firstHandler := func(event any) error {
		firstCalled = true
		return nil
	}

	secondHandler := func(event any) error {
		secondCalled = true
		return nil
	}

	registry.ListenEvent("test.event", firstHandler)
	registry.ListenEvent("test.event", secondHandler)

	if len(registry.EventHandlers) != 1 {
		t.Errorf("Expected 1 event handler, got %d", len(registry.EventHandlers))
	}

	registry.EventHandlers["test.event"](nil)

	if firstCalled {
		t.Error("First handler should not have been called")
	}

	if !secondCalled {
		t.Error("Second handler should have been called")
	}
}

func TestHandlerErrors(t *testing.T) {
	registry := NewRegistry()
	expectedErr := errors.New("handler error")

	eventHandler := func(event any) error {
		return expectedErr
	}

	commandHandler := func(command any) error {
		return expectedErr
	}

	queryHandler := func(query any) (any, error) {
		return nil, expectedErr
	}

	registry.ListenEvent("error.event", eventHandler)
	registry.HandleCommand("error.command", commandHandler)
	registry.ServeQuery("error.query", queryHandler)

	if err := registry.EventHandlers["error.event"](nil); err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if err := registry.CommandHandlers["error.command"](nil); err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if _, err := registry.QueryHandlers["error.query"](nil); err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}
