package rcommons

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func setupReactiveCommonsForCommandsBench() *ReactiveCommons {
	config := DomainDefinition{
		Name:              "perf-test-service",
		UseDomainEvents:   false,
		UseDirectQueries:  false,
		UseDirectCommands: true,
		ConnectionConfig: RabbitMQConfig{
			Host:                "localhost",
			Port:                5672,
			User:                "guest",
			Password:            "guest",
			VirtualHost:         "/",
			Secure:              false,
			SeparateConnections: true,
		},
	}
	return NewReactiveCommons(config)
}

// Benchmark sending commands
func BenchmarkSendCommand(b *testing.B) {
	// ========== ONE-TIME SETUP (NOT MEASURED) ==========
	rc := setupReactiveCommonsForCommandsBench()
	registry := NewRegistry()
	handler := func(command any) error {
		return nil
	}
	registry.HandleCommand("PerformActionCommand", handler)
	go rc.Start(registry)

	time.Sleep(1 * time.Second) // Wait for connection

	// Pre-create command structure to avoid measuring struct creation
	commandTemplate := Command[any]{
		Name:      "PerformActionCommand",
		CommandId: "",
		Data:      42,
	}

	commandOptions := CommandOptions{
		Domain:      "perf-test-service",
		TargetName:  "perf-test-service",
		DelayMillis: 2,
	}

	// ========== RESET TIMER - START MEASURING HERE ==========
	b.ResetTimer()
	b.ReportAllocs()
	// ========== MEASURED CODE ==========
	for b.Loop() {
		// Only UUID generation and SendCommand are measured
		commandTemplate.CommandId = uuid.NewString()

		err := rc.SendCommand(commandTemplate, commandOptions)
		if err != nil {
			b.Fatalf("Failed to send command: %v", err)
		}
	}

	// ========== CLEANUP (NOT MEASURED) ==========
	b.StopTimer()

	// allow some time for pending commands to be processed before shutting down
	time.Sleep(2 * time.Second)

}

func BenchmarkReceiveCommands(b *testing.B) {

	received := 0
	var mu sync.Mutex

	// ========== ONE-TIME SETUP (NOT MEASURED) ==========
	rc := setupReactiveCommonsForCommandsBench()
	registry := NewRegistry()
	handler := func(command any) error {
		mu.Lock()
		received++
		mu.Unlock()
		return nil
	}
	registry.HandleCommand("PerformActionCommand", handler)
	go rc.Start(registry)

	time.Sleep(1 * time.Second) // Wait for connection

	// Pre-create command structure to avoid measuring struct creation
	commandTemplate := Command[any]{
		Name:      "PerformActionCommand",
		CommandId: "",
		Data:      42,
	}

	commandOptions := CommandOptions{
		Domain:      "perf-test-service",
		TargetName:  "perf-test-service",
		DelayMillis: 2,
	}
	// ========== RESET TIMER - START MEASURING HERE ==========
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// Only UUID generation and SendCommand are measured
		commandTemplate.CommandId = uuid.NewString()

		err := rc.SendCommand(commandTemplate, commandOptions)
		if err != nil {
			b.Fatalf("Failed to send command: %v", err)
		}
	}

	// ========== CLEANUP (NOT MEASURED) ==========
	b.StopTimer()

	// Wait for all commands to be processed
	time.Sleep(3 * time.Second)

	mu.Lock()
	finalReceived := received
	mu.Unlock()

	b.ReportMetric(float64(finalReceived), "commands_received")
}
