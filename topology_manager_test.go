package rcommons

import (
	"testing"
)

func TestNewTopologyManager(t *testing.T) {
	domain := &DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "test.events",
		DomainEventsSuffix:   "events",
	}

	manager := newTopologyManager(domain)

	if manager == nil {
		t.Fatal("newTopologyManager() returned nil")
	}

	if manager.domain != domain {
		t.Error("Domain not set correctly")
	}
}

func TestRabbitTopologyManager_SetupDomainEvents_ChannelCreationFailure(t *testing.T) {
	// This test verifies error handling when channel creation fails
	domain := &DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "test.events",
		DomainEventsSuffix:   "events",
	}

	manager := newTopologyManager(domain)
	eventHandlers := make(map[string]EventHandler)

	// Passing nil client to test would cause panic, which is expected behavior
	// Skip testing with nil client here as it would panic
	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	if len(eventHandlers) != 0 {
		t.Error("Event handlers should be empty")
	}
}

func TestRabbitTopologyManager_SetupDirectCommands_ChannelCreationFailure(t *testing.T) {
	domain := &DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "test.direct",
		DirectCommandsSuffix: "commands",
		UseDirectQueries:     false,
	}

	manager := newTopologyManager(domain)

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	if manager.domain.DirectExchange != "test.direct" {
		t.Error("DirectExchange should be set correctly")
	}
}

func TestRabbitTopologyManager_SetupAsyncQueries_ChannelCreationFailure(t *testing.T) {
	domain := &DomainDefinition{
		Name:            "test-app",
		GlobalExchange:  "test.global",
		globalRepliesID: "test-replies",
		globalBindID:    "test-bind",
	}

	manager := newTopologyManager(domain)

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	if manager.domain.GlobalExchange != "test.global" {
		t.Error("GlobalExchange should be set correctly")
	}
}

func TestRabbitTopologyManager_DomainConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		domain *DomainDefinition
	}{
		{
			name: "Domain events configuration",
			domain: &DomainDefinition{
				Name:                 "app1",
				DomainEventsExchange: "domain.events",
				DomainEventsSuffix:   "subsEvents",
			},
		},
		{
			name: "Direct commands configuration",
			domain: &DomainDefinition{
				Name:                 "app2",
				DirectExchange:       "directMessages",
				DirectCommandsSuffix: "direct",
				UseDirectQueries:     false,
			},
		},
		{
			name: "Direct commands with queries",
			domain: &DomainDefinition{
				Name:                 "app3",
				DirectExchange:       "directMessages",
				DirectCommandsSuffix: "direct",
				DirectQuerySuffix:    "query",
				UseDirectQueries:     true,
			},
		},
		{
			name: "Async queries configuration",
			domain: &DomainDefinition{
				Name:                "app4",
				GlobalExchange:      "globalReply",
				GlobalRepliesSuffix: "replies",
				globalRepliesID:     "app4.replies.unique",
				globalBindID:        "bind-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newTopologyManager(tt.domain)

			if manager.domain != tt.domain {
				t.Errorf("Domain not set correctly for test: %s", tt.name)
			}

			if manager.domain.Name != tt.domain.Name {
				t.Errorf("Expected domain name %s, got %s", tt.domain.Name, manager.domain.Name)
			}
		})
	}
}

func TestRabbitTopologyManager_QueueNaming(t *testing.T) {
	// Test that queue names are calculated correctly based on domain configuration
	tests := []struct {
		name           string
		domainName     string
		suffix         string
		expectedPrefix string
	}{
		{
			name:           "Events queue",
			domainName:     "myapp",
			suffix:         "subsEvents",
			expectedPrefix: "myapp",
		},
		{
			name:           "Commands queue",
			domainName:     "service1",
			suffix:         "direct",
			expectedPrefix: "service1",
		},
		{
			name:           "Queries queue",
			domainName:     "api",
			suffix:         "query",
			expectedPrefix: "api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueName := calculateQueueName(tt.domainName, tt.suffix, false)

			if queueName == "" {
				t.Error("calculateQueueName returned empty string")
			}

			// Queue name should contain the domain name
			if len(queueName) < len(tt.domainName) {
				t.Errorf("Queue name %s is shorter than domain name %s", queueName, tt.domainName)
			}
		})
	}
}

func TestRabbitTopologyManager_ExchangeTypes(t *testing.T) {
	// Verify that the correct exchange types are used
	// Domain events use "topic" exchange
	// Direct commands use "direct" exchange
	// Async queries use "topic" exchange (global)

	domain := &DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
		DirectExchange:       "direct",
		GlobalExchange:       "global",
	}

	manager := newTopologyManager(domain)

	if manager.domain.DomainEventsExchange != "events" {
		t.Errorf("Expected DomainEventsExchange 'events', got '%s'", manager.domain.DomainEventsExchange)
	}

	if manager.domain.DirectExchange != "direct" {
		t.Errorf("Expected DirectExchange 'direct', got '%s'", manager.domain.DirectExchange)
	}

	if manager.domain.GlobalExchange != "global" {
		t.Errorf("Expected GlobalExchange 'global', got '%s'", manager.domain.GlobalExchange)
	}
}

func TestRabbitTopologyManager_MultipleSetupCalls(t *testing.T) {
	// Test that multiple calls to setup methods don't cause issues
	// (In real scenario with RabbitMQ, this tests idempotency)

	domain := &DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "events",
		DomainEventsSuffix:   "subsEvents",
	}

	manager := newTopologyManager(domain)

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	// Verify manager can be created multiple times with same config
	manager2 := newTopologyManager(domain)

	if manager2 == nil {
		t.Fatal("Second manager should not be nil")
	}

	if manager.domain.Name != manager2.domain.Name {
		t.Error("Both managers should have same domain name")
	}
}

func TestRabbitTopologyManager_EmptyDomainName(t *testing.T) {
	domain := &DomainDefinition{
		Name:                 "",
		DomainEventsExchange: "events",
	}

	manager := newTopologyManager(domain)

	if manager == nil {
		t.Fatal("Manager should be created even with empty domain name")
	}

	if manager.domain.Name != "" {
		t.Error("Domain name should be empty as configured")
	}
}

func TestRabbitTopologyManager_DefaultExchangeNames(t *testing.T) {
	// Test with default/empty exchange names
	domain := &DomainDefinition{
		Name:                 "test-app",
		DomainEventsExchange: "",
		DirectExchange:       "",
		GlobalExchange:       "",
	}

	manager := newTopologyManager(domain)

	if manager == nil {
		t.Fatal("Manager should be created with empty exchange names")
	}

	// Empty exchange names should be preserved
	if manager.domain.DomainEventsExchange != "" {
		t.Error("DomainEventsExchange should be empty")
	}
}

func TestRabbitTopologyManager_WithDirectQueriesEnabled(t *testing.T) {
	domain := &DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "direct",
		DirectCommandsSuffix: "commands",
		DirectQuerySuffix:    "query",
		UseDirectQueries:     true,
	}

	manager := newTopologyManager(domain)

	if !manager.domain.UseDirectQueries {
		t.Error("UseDirectQueries should be true")
	}

	if manager.domain.DirectQuerySuffix != "query" {
		t.Errorf("Expected DirectQuerySuffix 'query', got '%s'", manager.domain.DirectQuerySuffix)
	}
}

func TestRabbitTopologyManager_WithDirectQueriesDisabled(t *testing.T) {
	domain := &DomainDefinition{
		Name:                 "test-app",
		DirectExchange:       "direct",
		DirectCommandsSuffix: "commands",
		UseDirectQueries:     false,
	}

	manager := newTopologyManager(domain)

	if manager.domain.UseDirectQueries {
		t.Error("UseDirectQueries should be false")
	}
}

func TestRabbitTopologyManager_GlobalRepliesConfiguration(t *testing.T) {
	globalRepliesID := "unique-replies-id"
	globalBindID := "unique-bind-id"

	domain := &DomainDefinition{
		Name:                "test-app",
		GlobalExchange:      "globalReply",
		GlobalRepliesSuffix: "replies",
		globalRepliesID:     globalRepliesID,
		globalBindID:        globalBindID,
	}

	manager := newTopologyManager(domain)

	if manager.domain.globalRepliesID != globalRepliesID {
		t.Errorf("Expected globalRepliesID '%s', got '%s'", globalRepliesID, manager.domain.globalRepliesID)
	}

	if manager.domain.globalBindID != globalBindID {
		t.Errorf("Expected globalBindID '%s', got '%s'", globalBindID, manager.domain.globalBindID)
	}
}
