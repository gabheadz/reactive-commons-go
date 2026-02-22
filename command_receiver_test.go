package rcommons

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
	amqp "github.com/rabbitmq/amqp091-go"
)

type mockRabbitClientCommandReceiver struct {
	publishJsonFunc      func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error
	createChannelFunc    func(name string) (*amqp.Channel, error)
	getChannelFunc       func(name string) (*amqp.Channel, error)
	consumeOneFunc       func(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error)
	publishJsonQueryFunc func(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error
	closeFunc            func() error
}

func newMockRabbitClientCommandReceiver() *mockRabbitClientCommandReceiver {
	return &mockRabbitClientCommandReceiver{}
}

func (m *mockRabbitClientCommandReceiver) PublishJson(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
	if m.publishJsonFunc != nil {
		return m.publishJsonFunc(exchange, key, msg, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientCommandReceiver) CreateChannel(name string) (*amqp.Channel, error) {
	if m.createChannelFunc != nil {
		return m.createChannelFunc(name)
	}
	return nil, nil
}

func (m *mockRabbitClientCommandReceiver) GetChannel(name string) (*amqp.Channel, error) {
	if m.getChannelFunc != nil {
		return m.getChannelFunc(name)
	}
	return nil, nil
}

func (m *mockRabbitClientCommandReceiver) ConsumeOne(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error) {
	if m.consumeOneFunc != nil {
		return m.consumeOneFunc(channelKey, queueName, correlationId, timeout)
	}
	return []byte("mock response"), nil
}

func (m *mockRabbitClientCommandReceiver) PublishJsonQuery(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error {
	if m.publishJsonQueryFunc != nil {
		return m.publishJsonQueryFunc(exchange, key, msg, replyQ, correlationId, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientCommandReceiver) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// Mock acknowledger for amqp.Delivery
type mockAcknowledgerCommand struct {
	ackCalled  bool
	nackCalled bool
	requeue    bool
}

func (m *mockAcknowledgerCommand) Ack(tag uint64, multiple bool) error {
	m.ackCalled = true
	return nil
}

func (m *mockAcknowledgerCommand) Nack(tag uint64, multiple bool, requeue bool) error {
	m.nackCalled = true
	m.requeue = requeue
	return nil
}

func (m *mockAcknowledgerCommand) Reject(tag uint64, requeue bool) error {
	return nil
}

// Helper to create a delivery with mock acknowledger
func newMockDeliveryCommand(body []byte) (amqp.Delivery, *mockAcknowledgerCommand) {
	acker := &mockAcknowledgerCommand{}
	delivery := amqp.Delivery{
		Body:         body,
		Acknowledger: acker,
	}
	return delivery, acker
}

func TestNewCommandReceiver(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	mockClient := newMockRabbitClientCommandReceiver()

	receiver := newCommandReceiver(mockClient, &domain, &registry)

	if receiver == nil {
		t.Fatal("newCommandReceiver() returned nil")
	}

	if receiver.domain.Name != domain.Name {
		t.Errorf("Expected domain name '%s', got '%s'", domain.Name, receiver.domain.Name)
	}

	if receiver.domain.DirectExchange != domain.DirectExchange {
		t.Errorf("Expected DirectExchange '%s', got '%s'", domain.DirectExchange, receiver.domain.DirectExchange)
	}

	if receiver.registry != &registry {
		t.Error("Expected registry to be set correctly")
	}

	if receiver.client != mockClient {
		t.Error("Expected client to be set correctly")
	}

	expectedQueueName := "test-app.direct"
	if receiver.queueName != expectedQueueName {
		t.Errorf("Expected queue name '%s', got '%s'", expectedQueueName, receiver.queueName)
	}
}

func TestRabbitCommandReceiver_ProcessCommandMessage_Success(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	handlerCalled := false
	var capturedCommand any

	handler := func(command any) error {
		handlerCalled = true
		capturedCommand = command
		return nil
	}

	registry.HandleCommand("user.create", handler)

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	command := Command[any]{
		Name:      "user.create",
		CommandId: "cmd-123",
		Data:      map[string]string{"userId": "456"},
	}

	commandBytes, _ := json.Marshal(command)
	mockMsg, mockAck := newMockDeliveryCommand(commandBytes)

	receiver.processCommandMessage(mockMsg)

	if !handlerCalled {
		t.Error("Expected handler to be called")
	}

	if !mockAck.ackCalled {
		t.Error("Expected message to be acknowledged")
	}

	if mockAck.nackCalled {
		t.Error("Expected message not to be nacked")
	}

	cmdData, ok := capturedCommand.(Command[any])
	if !ok {
		t.Fatal("Expected captured command to be Command[any]")
	}

	if cmdData.Name != "user.create" {
		t.Errorf("Expected command name 'user.create', got '%s'", cmdData.Name)
	}

	if cmdData.CommandId != "cmd-123" {
		t.Errorf("Expected command ID 'cmd-123', got '%s'", cmdData.CommandId)
	}
}

func TestRabbitCommandReceiver_ProcessCommandMessage_HandlerError(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	expectedErr := errors.New("handler error")

	handler := func(command any) error {
		return expectedErr
	}

	registry.HandleCommand("user.create", handler)

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	command := Command[any]{
		Name:      "user.create",
		CommandId: "cmd-123",
		Data:      "test data",
	}

	commandBytes, _ := json.Marshal(command)
	mockMsg, mockAck := newMockDeliveryCommand(commandBytes)

	receiver.processCommandMessage(mockMsg)

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

func TestRabbitCommandReceiver_ProcessCommandMessage_InvalidJson(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	handlerCalled := false

	handler := func(command any) error {
		handlerCalled = true
		return nil
	}

	registry.HandleCommand("user.create", handler)

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	invalidJson := []byte("invalid json data")
	mockMsg, mockAck := newMockDeliveryCommand(invalidJson)

	receiver.processCommandMessage(mockMsg)

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

func TestRabbitCommandReceiver_ProcessCommandMessage_NoHandler(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	// Don't register any handler

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	command := Command[any]{
		Name:      "user.create",
		CommandId: "cmd-123",
		Data:      "test data",
	}

	commandBytes, _ := json.Marshal(command)
	mockMsg, mockAck := newMockDeliveryCommand(commandBytes)

	receiver.processCommandMessage(mockMsg)

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

func TestRabbitCommandReceiver_ProcessCommandMessage_DifferentDataTypes(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	tests := []struct {
		name        string
		commandData any
	}{
		{
			name:        "String data",
			commandData: "test string",
		},
		{
			name:        "Integer data",
			commandData: 42,
		},
		{
			name: "Map data",
			commandData: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
			},
		},
		{
			name: "Struct data",
			commandData: struct {
				Name string
				Age  int
			}{
				Name: "John",
				Age:  30,
			},
		},
		{
			name:        "Nil data",
			commandData: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			handlerCalled := false

			handler := func(command any) error {
				handlerCalled = true
				return nil
			}

			registry.HandleCommand("test.command", handler)

			receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

			command := Command[any]{
				Name:      "test.command",
				CommandId: "cmd-123",
				Data:      tt.commandData,
			}

			commandBytes, _ := json.Marshal(command)
			mockMsg, mockAck := newMockDeliveryCommand(commandBytes)

			receiver.processCommandMessage(mockMsg)

			if !handlerCalled {
				t.Errorf("Expected handler to be called for %s", tt.name)
			}

			if !mockAck.ackCalled {
				t.Errorf("Expected message to be acknowledged for %s", tt.name)
			}
		})
	}
}

func TestRabbitCommandReceiver_ProcessCommandMessage_MultipleCommands(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	processedCommands := []string{}

	handler := func(command any) error {
		cmd := command.(Command[any])
		processedCommands = append(processedCommands, cmd.Name)
		return nil
	}

	registry.HandleCommand("cmd1", handler)
	registry.HandleCommand("cmd2", handler)
	registry.HandleCommand("cmd3", handler)

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	commands := []Command[any]{
		{Name: "cmd1", CommandId: "id1", Data: "data1"},
		{Name: "cmd2", CommandId: "id2", Data: "data2"},
		{Name: "cmd3", CommandId: "id3", Data: "data3"},
	}

	for i, command := range commands {
		commandBytes, _ := json.Marshal(command)
		mockMsg, mockAck := newMockDeliveryCommand(commandBytes)

		receiver.processCommandMessage(mockMsg)

		if !mockAck.ackCalled {
			t.Errorf("Expected message %d to be acknowledged", i)
		}
	}

	if len(processedCommands) != 3 {
		t.Errorf("Expected 3 processed commands, got %d", len(processedCommands))
	}

	for i, cmdName := range []string{"cmd1", "cmd2", "cmd3"} {
		if processedCommands[i] != cmdName {
			t.Errorf("Expected command %d to be '%s', got '%s'", i, cmdName, processedCommands[i])
		}
	}
}

func TestRabbitCommandReceiver_RegistryIntegration(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()

	// Initially no handler
	if handler, exists := registry.GetCommandHandler("user.create"); exists && handler != nil {
		t.Error("Expected no handler to exist initially")
	}

	handler := func(command any) error {
		return nil
	}

	// Register handler
	registry.HandleCommand("user.create", handler)

	if handler, exists := registry.GetCommandHandler("user.create"); !exists || handler == nil {
		t.Error("Expected handler to exist after registration")
	}

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	command := Command[any]{
		Name:      "user.create",
		CommandId: "cmd-123",
		Data:      "test data",
	}

	commandBytes, _ := json.Marshal(command)
	mockMsg, mockAck := newMockDeliveryCommand(commandBytes)

	receiver.processCommandMessage(mockMsg)

	if !mockAck.ackCalled {
		t.Error("Expected message to be acknowledged with registered handler")
	}
}

func TestRabbitCommandReceiver_EmptyCommandData(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	handlerCalled := false

	handler := func(command any) error {
		handlerCalled = true
		cmd := command.(Command[any])
		if cmd.Data != nil {
			t.Error("Expected data to be nil")
		}
		return nil
	}

	registry.HandleCommand("test.command", handler)

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	command := Command[any]{
		Name:      "test.command",
		CommandId: "cmd-123",
		Data:      nil,
	}

	commandBytes, _ := json.Marshal(command)
	mockMsg, mockAck := newMockDeliveryCommand(commandBytes)

	receiver.processCommandMessage(mockMsg)

	if !handlerCalled {
		t.Error("Expected handler to be called")
	}

	if !mockAck.ackCalled {
		t.Error("Expected message to be acknowledged")
	}
}

func TestRabbitCommandReceiver_CommandIdPreservation(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	var capturedCommandId string

	handler := func(command any) error {
		cmd := command.(Command[any])
		capturedCommandId = cmd.CommandId
		return nil
	}

	registry.HandleCommand("test.command", handler)

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	expectedCommandId := "unique-cmd-id-12345"
	command := Command[any]{
		Name:      "test.command",
		CommandId: expectedCommandId,
		Data:      "test data",
	}

	commandBytes, _ := json.Marshal(command)
	mockMsg, _ := newMockDeliveryCommand(commandBytes)

	receiver.processCommandMessage(mockMsg)

	if capturedCommandId != expectedCommandId {
		t.Errorf("Expected command ID '%s', got '%s'", expectedCommandId, capturedCommandId)
	}
}

func TestRabbitCommandReceiver_ComplexCommandData(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	var capturedData any

	handler := func(command any) error {
		cmd := command.(Command[any])
		capturedData = cmd.Data
		return nil
	}

	registry.HandleCommand("order.create", handler)

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	complexData := map[string]interface{}{
		"orderId": "order-123",
		"items": []map[string]interface{}{
			{"productId": "prod-1", "quantity": 2},
			{"productId": "prod-2", "quantity": 1},
		},
		"total":    99.99,
		"customer": map[string]string{"id": "cust-456", "name": "John Doe"},
	}

	command := Command[any]{
		Name:      "order.create",
		CommandId: "cmd-789",
		Data:      complexData,
	}

	commandBytes, _ := json.Marshal(command)
	mockMsg, _ := newMockDeliveryCommand(commandBytes)

	receiver.processCommandMessage(mockMsg)

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

func TestRabbitCommandReceiver_BindingCalledOnStartListening(t *testing.T) {
	// This test verifies that GetChannel is called when startListeningCommands is invoked
	// We can't fully test startListeningCommands without a real RabbitMQ connection,
	// but we can verify the setup logic

	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	mockClient := newMockRabbitClientCommandReceiver()

	getChannelCalled := false
	mockClient.getChannelFunc = func(name string) (*amqp.Channel, error) {
		getChannelCalled = true
		if name != ChannelForCommands {
			t.Errorf("Expected channel name '%s', got '%s'", ChannelForCommands, name)
		}
		return nil, errors.New("mock error to prevent full execution")
	}

	receiver := newCommandReceiver(mockClient, &domain, &registry)

	// This will panic due to the mock error, but we're just checking the initial call
	defer func() {
		if r := recover(); r != nil {
			// Expected panic from log.Panicf
			if !getChannelCalled {
				t.Error("Expected GetChannel to be called")
			}
		}
	}()

	receiver.startListeningCommands()
}

func TestMockRabbitClientCommandReceiver_Methods(t *testing.T) {
	mock := newMockRabbitClientCommandReceiver()

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

func TestCalculateQueueName_CommandReceiver(t *testing.T) {
	// Test the queue name calculation used in newCommandReceiver
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectCommandsSuffix: "direct",
	}

	queueName := calculateQueueName(domain.Name, domain.DirectCommandsSuffix, false)
	expected := "test-app.direct"

	if queueName != expected {
		t.Errorf("Expected queue name '%s', got '%s'", expected, queueName)
	}
}

func TestRabbitCommandReceiver_QueueNameInitialization(t *testing.T) {
	tests := []struct {
		name              string
		domainName        string
		commandsSuffix    string
		expectedQueueName string
	}{
		{
			name:              "Basic configuration",
			domainName:        "app1",
			commandsSuffix:    "commands",
			expectedQueueName: "app1.commands",
		},
		{
			name:              "Custom suffix",
			domainName:        "app2",
			commandsSuffix:    "direct.commands",
			expectedQueueName: "app2.direct.commands",
		},
		{
			name:              "Empty suffix",
			domainName:        "app3",
			commandsSuffix:    "",
			expectedQueueName: "app3.default.queue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain := DomainDefinition{
				Name:                 tt.domainName,
				DirectExchange:       "commands",
				DirectCommandsSuffix: tt.commandsSuffix,
			}

			registry := NewRegistry()
			receiver := newCommandReceiver(nil, &domain, &registry)

			if receiver.queueName != tt.expectedQueueName {
				t.Errorf("Expected queue name '%s', got '%s'", tt.expectedQueueName, receiver.queueName)
			}
		})
	}
}

func TestRabbitCommandReceiver_DomainConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		domain DomainDefinition
	}{
		{
			name: "Basic configuration",
			domain: DomainDefinition{
				Name:                 "app1",
				DirectExchange:       "commands.exchange",
				DirectCommandsSuffix: "commands",
			},
		},
		{
			name: "Custom configuration",
			domain: DomainDefinition{
				Name:                 "app2",
				DirectExchange:       "custom.commands",
				DirectCommandsSuffix: "subsCommands",
			},
		},
		{
			name: "Minimal configuration",
			domain: DomainDefinition{
				Name:                 "app3",
				DirectExchange:       "",
				DirectCommandsSuffix: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			receiver := newCommandReceiver(nil, &tt.domain, &registry)

			if receiver == nil {
				t.Fatal("newCommandReceiver() returned nil")
			}

			if receiver.domain.Name != tt.domain.Name {
				t.Errorf("Expected domain name '%s', got '%s'", tt.domain.Name, receiver.domain.Name)
			}

			if receiver.domain.DirectExchange != tt.domain.DirectExchange {
				t.Errorf("Expected DirectExchange '%s', got '%s'", tt.domain.DirectExchange, receiver.domain.DirectExchange)
			}
		})
	}
}

func TestMockAcknowledgerCommand_Creation(t *testing.T) {
	delivery, acker := newMockDeliveryCommand([]byte("test body"))

	if len(delivery.Body) == 0 {
		t.Error("Expected delivery body to be set")
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
	acker2 := &mockAcknowledgerCommand{}
	delivery2 := amqp.Delivery{
		Body:         []byte("test"),
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

func TestRabbitCommandReceiver_MultipleHandlers(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	callCount := make(map[string]int)

	handler1 := func(command any) error {
		callCount["cmd1"]++
		return nil
	}

	handler2 := func(command any) error {
		callCount["cmd2"]++
		return nil
	}

	handler3 := func(command any) error {
		callCount["cmd3"]++
		return nil
	}

	registry.HandleCommand("cmd1", handler1)
	registry.HandleCommand("cmd2", handler2)
	registry.HandleCommand("cmd3", handler3)

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	commands := []string{"cmd1", "cmd2", "cmd1", "cmd3", "cmd2"}

	for _, cmdName := range commands {
		command := Command[any]{
			Name:      cmdName,
			CommandId: "id-123",
			Data:      "test",
		}

		commandBytes, _ := json.Marshal(command)
		mockMsg, mockAck := newMockDeliveryCommand(commandBytes)

		receiver.processCommandMessage(mockMsg)

		if !mockAck.ackCalled {
			t.Errorf("Expected message for %s to be acknowledged", cmdName)
		}
	}

	if callCount["cmd1"] != 2 {
		t.Errorf("Expected handler1 to be called 2 times, got %d", callCount["cmd1"])
	}

	if callCount["cmd2"] != 2 {
		t.Errorf("Expected handler2 to be called 2 times, got %d", callCount["cmd2"])
	}

	if callCount["cmd3"] != 1 {
		t.Errorf("Expected handler3 to be called 1 time, got %d", callCount["cmd3"])
	}
}

func TestRabbitCommandReceiver_CommandNameMatching(t *testing.T) {
	domain := DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "commands",
		DirectCommandsSuffix: "direct",
	}

	registry := NewRegistry()
	var capturedName string

	handler := func(command any) error {
		cmd := command.(Command[any])
		capturedName = cmd.Name
		return nil
	}

	registry.HandleCommand("user.create", handler)

	receiver := newCommandReceiver(newMockRabbitClientCommandReceiver(), &domain, &registry)

	cmdName := "user.create"
	command := Command[any]{
		Name:      cmdName,
		CommandId: "cmd-123",
		Data:      "test",
	}

	commandBytes, _ := json.Marshal(command)
	mockMsg, mockAck := newMockDeliveryCommand(commandBytes)

	receiver.processCommandMessage(mockMsg)

	if capturedName != cmdName {
		t.Errorf("Expected command name '%s', got '%s'", cmdName, capturedName)
	}

	if !mockAck.ackCalled {
		t.Error("Expected message to be acknowledged")
	}
}

// Test to verify the package initialization
func init() {
	// Ensure rabbit package is available
	_ = rabbit.RabbitChannel{}
}
