package rcommons

import (
	"errors"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry.GetEventHandlers() == nil {
		t.Error("EventHandlers map not initialized")
	}

	if registry.GetCommandHandlers() == nil {
		t.Error("CommandHandlers map not initialized")
	}

	if registry.GetQueryHandlers() == nil {
		t.Error("QueryHandlers map not initialized")
	}

	if registry.EventHandlersCount() != 0 {
		t.Errorf("Expected empty EventHandlers, got %d entries", registry.EventHandlersCount())
	}

	if registry.CommandHandlersCount() != 0 {
		t.Errorf("Expected empty CommandHandlers, got %d entries", registry.CommandHandlersCount())
	}

	if registry.QueryHandlersCount() != 0 {
		t.Errorf("Expected empty QueryHandlers, got %d entries", registry.QueryHandlersCount())
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

	if registry.EventHandlersCount() != 1 {
		t.Errorf("Expected 1 event handler, got %d", registry.EventHandlersCount())
	}

	if _, exists := registry.GetEventHandler("test.event"); !exists {
		t.Error("Event handler not registered with correct name")
	}

	// Test handler execution
	eventHandlerFn, _ := registry.GetEventHandler("test.event")
	err := eventHandlerFn("test data")
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

	if registry.EventHandlersCount() != 1 {
		t.Errorf("Expected 1 event handler, got %d", registry.EventHandlersCount())
	}

	// Test with correct type
	typedHandlerFn, _ := registry.GetEventHandler("typed.event")
	err := typedHandlerFn("hello")
	if err != nil {
		t.Errorf("Handler returned error: %v", err)
	}

	if receivedEvent != "hello" {
		t.Errorf("Expected 'hello', got '%s'", receivedEvent)
	}

	// Test with incorrect type
	err = typedHandlerFn(123)
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

	if registry.EventHandlersCount() != 3 {
		t.Errorf("Expected 3 event handlers, got %d", registry.EventHandlersCount())
	}

	for name := range handlers {
		if _, exists := registry.GetEventHandler(name); !exists {
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

	if registry.CommandHandlersCount() != 1 {
		t.Errorf("Expected 1 command handler, got %d", registry.CommandHandlersCount())
	}

	if _, exists := registry.GetCommandHandler("test.command"); !exists {
		t.Error("Command handler not registered with correct name")
	}

	// Test handler execution
	handlerFn, _ := registry.GetCommandHandler("test.command")
	err := handlerFn("test command")
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

	if registry.CommandHandlersCount() != 2 {
		t.Errorf("Expected 2 command handlers, got %d", registry.CommandHandlersCount())
	}

	for name := range handlers {
		if _, exists := registry.GetCommandHandler(name); !exists {
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

	if registry.QueryHandlersCount() != 1 {
		t.Errorf("Expected 1 query handler, got %d", registry.QueryHandlersCount())
	}

	if _, exists := registry.GetQueryHandler("test.query"); !exists {
		t.Error("Query handler not registered with correct name")
	}

	// Test handler execution
	queryFn, _ := registry.GetQueryHandler("test.query")
	result, err := queryFn("test query")
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

	if registry.QueryHandlersCount() != 2 {
		t.Errorf("Expected 2 query handlers, got %d", registry.QueryHandlersCount())
	}

	for name := range handlers {
		if _, exists := registry.GetQueryHandler(name); !exists {
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

	if registry.EventHandlersCount() != 1 {
		t.Errorf("Expected 1 event handler, got %d", registry.EventHandlersCount())
	}

	handler, _ := registry.GetEventHandler("test.event")
	handler(nil)

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

	eventFn, _ := registry.GetEventHandler("error.event")
	if err := eventFn(nil); err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	commandFn, _ := registry.GetCommandHandler("error.command")
	if err := commandFn(nil); err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	queryFn, _ := registry.GetQueryHandler("error.query")
	if _, err := queryFn(nil); err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}
