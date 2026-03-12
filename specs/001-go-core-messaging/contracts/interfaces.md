# Contract: Exported Go Interfaces

**Feature**: 001-go-core-messaging
**Date**: 2026-03-11

This document defines the complete public API contract for `reactive-commons-go`.
These interfaces are the stability boundary — downstream consumers depend on them.

---

## Package: `pkg/async`

### Interface: `DomainEventBus`

```go
package async

import "context"

// DomainEventBus publishes domain events to all subscribed services.
// Implementations MUST block until the broker acknowledges the publish
// (publisher confirm semantics).
type DomainEventBus interface {
    // Emit publishes event to all subscribers. Returns error if the broker
    // does not acknowledge within the context deadline.
    Emit(ctx context.Context, event DomainEvent[any]) error
}
```

---

### Interface: `DirectAsyncGateway`

```go
package async

import (
    "context"
    "encoding/json"
)

// DirectAsyncGateway sends commands and queries to named target services.
type DirectAsyncGateway interface {
    // SendCommand delivers cmd to targetService. Blocks until broker confirms.
    // Returns error on broker failure or context cancellation.
    SendCommand(ctx context.Context, cmd Command[any], targetService string) error

    // RequestReply sends query to targetService and waits for the typed response.
    // Returns (raw JSON bytes, nil) on success.
    // Returns (nil, error) on timeout (ctx deadline exceeded) or broker failure.
    RequestReply(ctx context.Context, query AsyncQuery[any], targetService string) (json.RawMessage, error)

    // Reply sends a query response to the original caller.
    // from MUST be the From value received in the QueryHandler invocation.
    // Pass nil response to send a completion-only signal.
    Reply(ctx context.Context, response any, from From) error
}
```

---

### Interface: `HandlerRegistry`

```go
package async

// HandlerRegistry registers message handlers for a service instance.
// All registration MUST occur before calling Application.Start().
// Dynamic registration after Start() is supported but requires the broker
// connection to be live; implementation MUST update queue bindings accordingly.
type HandlerRegistry interface {
    // ListenEvent registers handler for the named domain event type.
    // Returns error if a handler for eventName is already registered.
    ListenEvent(eventName string, handler EventHandler[any]) error

    // ListenCommand registers handler for the named command type.
    // Returns error if a handler for commandName is already registered.
    ListenCommand(commandName string, handler CommandHandler[any]) error

    // ServeQuery registers handler for the named query type.
    // Returns error if a handler for queryName is already registered.
    ServeQuery(queryName string, handler QueryHandler[any, any]) error

    // ListenNotification registers handler for the named notification type.
    // Returns error if a handler for notificationName is already registered.
    ListenNotification(notificationName string, handler NotificationHandler[any]) error
}
```

---

### Interface: `Application`

```go
package async

// Application is the top-level lifecycle interface for a reactive-commons instance.
type Application interface {
    // Registry returns the HandlerRegistry for registering message handlers.
    Registry() HandlerRegistry

    // EventBus returns the DomainEventBus for publishing domain events.
    EventBus() DomainEventBus

    // Gateway returns the DirectAsyncGateway for sending commands and queries.
    Gateway() DirectAsyncGateway

    // Start declares broker topology, starts all consumers, and begins
    // processing messages. Blocks until the context is cancelled.
    Start(ctx context.Context) error
}
```

---

## Package: `rabbit`

### Factory: `NewApplication`

```go
package rabbit

import "github.com/bancolombia/reactive-commons-go/pkg/async"

// NewApplication creates an Application backed by RabbitMQ with the given config.
// Returns error if config is invalid (missing AppName, unreachable host, etc.).
func NewApplication(cfg RabbitConfig) (async.Application, error)
```

---

## Handler Type Signatures

```go
package async

import "context"

// EventHandler handles a domain event. Return non-nil error to nack (redeliver/DLQ).
type EventHandler[T any] func(ctx context.Context, event DomainEvent[T]) error

// CommandHandler handles a command. Return non-nil error to nack (redeliver/DLQ).
type CommandHandler[T any] func(ctx context.Context, cmd Command[T]) error

// QueryHandler handles a query and returns the response.
// Return (nil, error) to send an error reply. Return (response, nil) to reply normally.
type QueryHandler[Req, Res any] func(ctx context.Context, query AsyncQuery[Req], from From) (Res, error)

// NotificationHandler handles a notification. Error is logged; no redelivery.
type NotificationHandler[T any] func(ctx context.Context, n Notification[T]) error
```

---

## Error Contract (`pkg/async/errors.go`)

| Error Type             | When returned                                           |
|------------------------|---------------------------------------------------------|
| `ErrDuplicateHandler`  | `ListenEvent/Command/ServeQuery/ListenNotification` called twice for the same name |
| `ErrQueryTimeout`      | `RequestReply` context deadline exceeded               |
| `ErrBrokerUnavailable` | Broker connection failed during publish/send           |
| `ErrDeserialize`       | Incoming message body cannot be unmarshaled into target type |

---

## Usage Example (minimal)

```go
app, err := rabbit.NewApplication(rabbit.RabbitConfig{
    AppName:  "order-service",
    Host:     "localhost",
    Password: "guest",
})
if err != nil {
    log.Fatal(err)
}

// Register handlers BEFORE Start()
app.Registry().ListenEvent("order.created", func(ctx context.Context, e async.DomainEvent[any]) error {
    fmt.Println("received:", e.Name)
    return nil
})

// Start blocks until ctx is cancelled
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
log.Fatal(app.Start(ctx))
```
