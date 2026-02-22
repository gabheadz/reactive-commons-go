package rcommons

import (
	// "context"
	// "fmt"

	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type TestEvent struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Value   int    `json:"value"`
}

func setupReactiveCommons() *ReactiveCommons {
	config := DomainDefinition{
		Name:              "perf-test-service",
		UseDomainEvents:   true,
		UseDirectQueries:  false,
		UseDirectCommands: false,
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

// Benchmark sending events
func BenchmarkSendEvent(b *testing.B) {
	// ========== ONE-TIME SETUP (NOT MEASURED) ==========
	rc := setupReactiveCommons()
	registry := NewRegistry()
	handler := func(event any) error {
		// b.Logf("Event received <<<<<<<<<< %v", event)
		return nil
	}
	registry.ListenEvent("PerformanceEventGenerated", handler)
	go rc.Start(registry)

	time.Sleep(1 * time.Second) // Wait for connection

	// Pre-create event structure to avoid measuring struct creation
	eventTemplate := DomainEvent[any]{
		Name: "PerformanceEventGenerated",
		Data: time.Now().Local(),
	}

	eventOptions := EventOptions{
		Domain:     "perf-test-service",
		TargetName: "App1",
	}

	// ========== RESET TIMER - START MEASURING HERE ==========
	b.ResetTimer()

	// ========== MEASURED CODE (runs b.N times) ==========
	for b.Loop() {
		// Only UUID generation and EmitEvent are measured
		eventTemplate.EventId = uuid.NewString()

		err := rc.EmitEvent(eventTemplate, eventOptions)
		if err != nil {
			b.Fatalf("Failed to send event: %v", err)
		}
	}

	// ========== CLEANUP (NOT MEASURED) ==========
	b.StopTimer()

	// Wait for all events to be processed before exiting
	time.Sleep(2 * time.Second)
}

// Benchmark sending events concurrently
func BenchmarkSendEventConcurrent(b *testing.B) {
	rc := setupReactiveCommons()
	registry := NewRegistry()
	handler := func(event any) error {
		return nil
	}
	registry.ListenEvent("PerformanceEventGenerated", handler)
	go rc.Start(registry)

	time.Sleep(1 * time.Second) // Wait for connection

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			event := DomainEvent[any]{
				Name:    "PerformanceEventGenerated",
				EventId: uuid.NewString(),
				Data:    time.Now().Local(),
			}

			eventOptions := EventOptions{
				Domain:     "perf-test-service",
				TargetName: "App1",
			}

			err := rc.EmitEvent(event, eventOptions)
			if err != nil {
				b.Fatalf("Failed to send event: %v", err)
			}
		}
	})
}

// Benchmark receiving events
func BenchmarkReceiveEvent(b *testing.B) {

	received := 0
	var mu sync.Mutex

	// ========== ONE-TIME SETUP (NOT MEASURED) ==========
	rc := setupReactiveCommons()
	registry := NewRegistry()
	handler := func(event any) error {
		mu.Lock()
		received++
		mu.Unlock()
		return nil
	}
	registry.ListenEvent("PerformanceEventGenerated", handler)
	go rc.Start(registry)

	time.Sleep(1 * time.Second) // Wait for connection

	// Pre-create event structure to avoid measuring struct creation
	eventTemplate := DomainEvent[any]{
		Name: "PerformanceEventGenerated",
		Data: time.Now().Local(),
	}

	eventOptions := EventOptions{
		Domain:     "perf-test-service",
		TargetName: "App1",
	}

	b.ResetTimer()
	for b.Loop() {
		err := rc.EmitEvent(eventTemplate, eventOptions)
		if err != nil {
			b.Fatalf("Failed to send event: %v", err)
		}
	}

	// Wait for all events to be processed
	time.Sleep(3 * time.Second)

	mu.Lock()
	finalReceived := received
	mu.Unlock()

	b.ReportMetric(float64(finalReceived), "events_received")
}

// Benchmark end-to-end: send and receive
func BenchmarkSendReceiveRoundtrip(b *testing.B) {
	var wg sync.WaitGroup

	rc := setupReactiveCommons()
	registry := NewRegistry()

	handler := func(event any) error {
		wg.Done()
		return nil
	}
	registry.ListenEvent("PerformanceEventGenerated", handler)
	go rc.Start(registry)

	time.Sleep(1 * time.Second)

	eventTemplate := DomainEvent[any]{
		Name:    "PerformanceEventGenerated",
		EventId: uuid.NewString(),
		Data:    time.Now().Local(),
	}

	eventOptions := EventOptions{
		Domain:     "perf-test-service",
		TargetName: "App1",
	}

	b.ResetTimer()
	for b.Loop() {
		wg.Add(1)
		err := rc.EmitEvent(eventTemplate, eventOptions)
		if err != nil {
			b.Fatalf("Failed to send event: %v", err)
		}
		wg.Wait()
	}

	time.Sleep(5 * time.Second)
}
