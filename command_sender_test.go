package rcommons

import (
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type mockRabbitClientCommandSender struct {
	publishJsonFunc      func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error
	createChannelFunc    func(name string) error
	getChannelFunc       func(name string) error
	consumeOneFunc       func(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error)
	publishJsonQueryFunc func(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error
	closeFunc            func() error
}

func newMockRabbitClientCommandSender() *mockRabbitClientCommandSender {
	return &mockRabbitClientCommandSender{}
}

func (m *mockRabbitClientCommandSender) PublishJson(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
	if m.publishJsonFunc != nil {
		return m.publishJsonFunc(exchange, key, msg, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientCommandSender) CreateChannel(name string) (*amqp.Channel, error) {
	if m.createChannelFunc != nil {
		expectedErr := m.createChannelFunc(name)
		if expectedErr != nil {
			return nil, expectedErr
		}
	}
	return nil, nil
}

func (m *mockRabbitClientCommandSender) GetChannel(name string) (*amqp.Channel, error) {
	if m.getChannelFunc != nil {
		expectedErr := m.getChannelFunc(name)
		if expectedErr != nil {
			return nil, expectedErr
		}
	}
	return nil, nil
}

func (m *mockRabbitClientCommandSender) ConsumeOne(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error) {
	if m.consumeOneFunc != nil {
		return m.consumeOneFunc(channelKey, queueName, correlationId, timeout)
	}
	return []byte("mock response"), nil
}

func (m *mockRabbitClientCommandSender) PublishJsonQuery(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error {
	if m.publishJsonQueryFunc != nil {
		return m.publishJsonQueryFunc(exchange, key, msg, replyQ, correlationId, channelKey, headers)
	}
	return nil
}

func (m *mockRabbitClientCommandSender) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestNewCommandSender(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()

	sender := newCommandSender(mockClient, &domain)

	if sender == nil {
		t.Fatal("newCommandSender() returned nil")
	}

	if sender.domain.Name != domain.Name {
		t.Errorf("Expected domain name '%s', got '%s'", domain.Name, sender.domain.Name)
	}

	if sender.domain.DirectExchange != domain.DirectExchange {
		t.Errorf("Expected DirectExchange '%s', got '%s'", domain.DirectExchange, sender.domain.DirectExchange)
	}

	if sender.client != mockClient {
		t.Error("Expected client to be set correctly")
	}
}

func TestRabbitCommandSender_SendCommand_Success(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	var capturedExchange, capturedKey, capturedChannelKey string
	var capturedHeaders map[string]string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedExchange = exchange
		capturedKey = key
		capturedChannelKey = channelKey
		capturedHeaders = headers
		return nil
	}

	sender := newCommandSender(mockClient, &domain)

	command := Command[any]{
		Name:      "user.create",
		CommandId: "cmd-123",
		Data:      map[string]string{"userId": "456"},
	}

	opts := CommandOptions{
		Domain:     "test-domain",
		TargetName: "user-service",
	}

	err := sender.sendCommand(command, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if capturedExchange != domain.DirectExchange {
		t.Errorf("Expected exchange '%s', got '%s'", domain.DirectExchange, capturedExchange)
	}

	if capturedKey != opts.TargetName {
		t.Errorf("Expected routing key '%s', got '%s'", opts.TargetName, capturedKey)
	}

	if capturedChannelKey != ChannelForCommands {
		t.Errorf("Expected channel key '%s', got '%s'", ChannelForCommands, capturedChannelKey)
	}

	if capturedHeaders["sourceApplication"] != domain.Name {
		t.Errorf("Expected sourceApplication '%s', got '%s'", domain.Name, capturedHeaders["sourceApplication"])
	}

	if capturedHeaders["message_id"] != command.CommandId {
		t.Errorf("Expected message_id '%s', got '%s'", command.CommandId, capturedHeaders["message_id"])
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

func TestRabbitCommandSender_SendCommand_MissingCommandName(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	sender := newCommandSender(newMockRabbitClientCommandSender(), &domain)

	command := Command[any]{
		Name:      "",
		CommandId: "cmd-123",
		Data:      "test data",
	}

	opts := CommandOptions{
		Domain:     "test-domain",
		TargetName: "user-service",
	}

	err := sender.sendCommand(command, opts)

	if err == nil {
		t.Error("Expected error for missing command name, got nil")
	}

	expectedMsg := "command name, domain, and target are required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRabbitCommandSender_SendCommand_MissingDomain(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	sender := newCommandSender(newMockRabbitClientCommandSender(), &domain)

	command := Command[any]{
		Name:      "user.create",
		CommandId: "cmd-123",
		Data:      "test data",
	}

	opts := CommandOptions{
		Domain:     "",
		TargetName: "user-service",
	}

	err := sender.sendCommand(command, opts)

	if err == nil {
		t.Error("Expected error for missing domain, got nil")
	}

	expectedMsg := "command name, domain, and target are required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRabbitCommandSender_SendCommand_MissingTargetName(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	sender := newCommandSender(newMockRabbitClientCommandSender(), &domain)

	command := Command[any]{
		Name:      "user.create",
		CommandId: "cmd-123",
		Data:      "test data",
	}

	opts := CommandOptions{
		Domain:     "test-domain",
		TargetName: "",
	}

	err := sender.sendCommand(command, opts)

	if err == nil {
		t.Error("Expected error for missing target name, got nil")
	}

	expectedMsg := "command name, domain, and target are required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRabbitCommandSender_SendCommand_PublishError(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	expectedError := errors.New("publish failed")

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return expectedError
	}

	sender := newCommandSender(mockClient, &domain)

	command := Command[any]{
		Name:      "user.create",
		CommandId: "cmd-123",
		Data:      "test data",
	}

	opts := CommandOptions{
		Domain:     "test-domain",
		TargetName: "user-service",
	}

	err := sender.sendCommand(command, opts)

	if err == nil {
		t.Error("Expected error from publish, got nil")
	}

	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

func TestRabbitCommandSender_SendCommand_DifferentDataTypes(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return nil
	}

	sender := newCommandSender(mockClient, &domain)

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
			command := Command[any]{
				Name:      "test.command",
				CommandId: "cmd-123",
				Data:      tt.commandData,
			}

			opts := CommandOptions{
				Domain:     "test-domain",
				TargetName: "test-service",
			}

			err := sender.sendCommand(command, opts)

			if err != nil {
				t.Errorf("Expected no error for %s, got: %v", tt.name, err)
			}
		})
	}
}

func TestRabbitCommandSender_SendCommand_HeadersValidation(t *testing.T) {
	domain := DomainDefinition{
		Name:           "my-service",
		DirectExchange: "domain.commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	var capturedHeaders map[string]string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedHeaders = headers
		return nil
	}

	sender := newCommandSender(mockClient, &domain)

	command := Command[any]{
		Name:      "order.create",
		CommandId: "order-789",
		Data:      "order data",
	}

	opts := CommandOptions{
		Domain:     "orders",
		TargetName: "order-service",
	}

	err := sender.sendCommand(command, opts)

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

func TestRabbitCommandSender_SendCommand_MultipleCommands(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	publishCount := 0

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		publishCount++
		return nil
	}

	sender := newCommandSender(mockClient, &domain)

	commands := []Command[any]{
		{Name: "cmd1", CommandId: "id1", Data: "data1"},
		{Name: "cmd2", CommandId: "id2", Data: "data2"},
		{Name: "cmd3", CommandId: "id3", Data: "data3"},
	}

	opts := CommandOptions{Domain: "test-domain", TargetName: "test-service"}

	for _, command := range commands {
		err := sender.sendCommand(command, opts)
		if err != nil {
			t.Errorf("Expected no error for command %s, got: %v", command.Name, err)
		}
	}

	if publishCount != len(commands) {
		t.Errorf("Expected %d publish calls, got %d", len(commands), publishCount)
	}
}

func TestRabbitCommandSender_SendCommand_CommandIdPreservation(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	var capturedMessageId string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedMessageId = headers["message_id"]
		return nil
	}

	sender := newCommandSender(mockClient, &domain)

	expectedCommandId := "unique-cmd-id-12345"
	command := Command[any]{
		Name:      "test.command",
		CommandId: expectedCommandId,
		Data:      "test data",
	}

	opts := CommandOptions{
		Domain:     "test-domain",
		TargetName: "test-service",
	}

	err := sender.sendCommand(command, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if capturedMessageId != expectedCommandId {
		t.Errorf("Expected command ID '%s', got '%s'", expectedCommandId, capturedMessageId)
	}
}

func TestRabbitCommandSender_SendCommand_ComplexCommandData(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return nil
	}

	sender := newCommandSender(mockClient, &domain)

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

	opts := CommandOptions{
		Domain:     "orders",
		TargetName: "order-service",
	}

	err := sender.sendCommand(command, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestRabbitCommandSender_DomainConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		domain DomainDefinition
	}{
		{
			name: "Basic configuration",
			domain: DomainDefinition{
				Name:           "app1",
				DirectExchange: "commands.exchange",
			},
		},
		{
			name: "Custom configuration",
			domain: DomainDefinition{
				Name:           "app2",
				DirectExchange: "custom.commands",
			},
		},
		{
			name: "Empty exchange",
			domain: DomainDefinition{
				Name:           "app3",
				DirectExchange: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockRabbitClientCommandSender()
			var capturedExchange string

			mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
				capturedExchange = exchange
				return nil
			}

			sender := newCommandSender(mockClient, &tt.domain)

			command := Command[any]{
				Name:      "test.command",
				CommandId: "cmd-123",
				Data:      "test",
			}

			opts := CommandOptions{
				Domain:     "test-domain",
				TargetName: "test-service",
			}

			sender.sendCommand(command, opts)

			if capturedExchange != tt.domain.DirectExchange {
				t.Errorf("Expected exchange '%s', got '%s'", tt.domain.DirectExchange, capturedExchange)
			}
		})
	}
}

func TestRabbitCommandSender_TargetNameRouting(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	var capturedRoutingKey string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedRoutingKey = key
		return nil
	}

	sender := newCommandSender(mockClient, &domain)

	tests := []string{
		"user-service",
		"order-service",
		"payment-service",
		"notification-service",
	}

	for _, targetName := range tests {
		command := Command[any]{
			Name:      "test.command",
			CommandId: "cmd-123",
			Data:      "test",
		}

		opts := CommandOptions{
			Domain:     "test-domain",
			TargetName: targetName,
		}

		err := sender.sendCommand(command, opts)

		if err != nil {
			t.Errorf("Expected no error for target '%s', got: %v", targetName, err)
		}

		if capturedRoutingKey != targetName {
			t.Errorf("Expected routing key '%s', got '%s'", targetName, capturedRoutingKey)
		}
	}
}

func TestRabbitCommandSender_AllValidationsRequired(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	sender := newCommandSender(newMockRabbitClientCommandSender(), &domain)

	command := Command[any]{
		Name:      "user.create",
		CommandId: "cmd-123",
		Data:      "test data",
	}

	opts := CommandOptions{
		Domain:     "test-domain",
		TargetName: "user-service",
	}

	// Test with all fields present - should succeed
	mockClient := newMockRabbitClientCommandSender()
	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		return nil
	}
	sender.client = mockClient

	err := sender.sendCommand(command, opts)
	if err != nil {
		t.Errorf("Expected no error with all fields, got: %v", err)
	}

	// Test missing one field at a time
	testCases := []struct {
		name       string
		command    Command[any]
		opts       CommandOptions
		shouldFail bool
	}{
		{
			name:       "Empty command name",
			command:    Command[any]{"", "cmd-123", "data"},
			opts:       opts,
			shouldFail: true,
		},
		{
			name:       "Empty domain",
			command:    command,
			opts:       CommandOptions{TargetName: "service", Domain: ""},
			shouldFail: true,
		},
		{
			name:       "Empty target name",
			command:    command,
			opts:       CommandOptions{TargetName: "", Domain: "domain"},
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := sender.sendCommand(tc.command, tc.opts)
			if tc.shouldFail && err == nil {
				t.Error("Expected error for validation failure")
			}
			if !tc.shouldFail && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestRabbitCommandSender_SendCommand_DomainNameInHeaders(t *testing.T) {
	domain := DomainDefinition{
		Name:           "my-domain-service",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	var capturedHeaders map[string]string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedHeaders = headers
		return nil
	}

	sender := newCommandSender(mockClient, &domain)

	command := Command[any]{
		Name:      "test.command",
		CommandId: "cmd-123",
		Data:      "test",
	}

	opts := CommandOptions{
		Domain:     "test-domain",
		TargetName: "test-service",
	}

	err := sender.sendCommand(command, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if capturedHeaders["sourceApplication"] != "my-domain-service" {
		t.Errorf("Expected sourceApplication to be domain name 'my-domain-service', got '%s'", capturedHeaders["sourceApplication"])
	}

	if capturedHeaders["app_id"] != "my-domain-service" {
		t.Errorf("Expected app_id to be domain name 'my-domain-service', got '%s'", capturedHeaders["app_id"])
	}
}

func TestRabbitCommandSender_TimestampHeader(t *testing.T) {
	domain := DomainDefinition{
		Name:           "test-app",
		DirectExchange: "commands",
	}

	mockClient := newMockRabbitClientCommandSender()
	var capturedTimestamp string

	mockClient.publishJsonFunc = func(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {
		capturedTimestamp = headers["timestamp"]
		return nil
	}

	sender := newCommandSender(mockClient, &domain)

	command := Command[any]{
		Name:      "test.command",
		CommandId: "cmd-123",
		Data:      "test",
	}

	opts := CommandOptions{
		Domain:     "test-domain",
		TargetName: "test-service",
	}

	err := sender.sendCommand(command, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if capturedTimestamp == "" {
		t.Error("Expected timestamp header to be set")
	}
}

func TestMockRabbitClientCommandSender_Methods(t *testing.T) {
	mock := newMockRabbitClientCommandSender()

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
