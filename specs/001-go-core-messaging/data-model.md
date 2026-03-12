# Data Model: Go Core Messaging Library

**Feature**: 001-go-core-messaging
**Date**: 2026-03-11

---

## Core Domain Types (`pkg/async/types.go`)

### DomainEvent

Represents a fact that happened in a domain. Published to all interested subscribers.

```go
// DomainEvent is a named, immutable record of something that happened.
// Wire JSON: {"name":"...","eventId":"...","data":{...}}
type DomainEvent[T any] struct {
    Name    string `json:"name"`
    EventID string `json:"eventId"`
    Data    T      `json:"data"`
}
```

| Field   | Type     | Required | Description                                  |
|---------|----------|----------|----------------------------------------------|
| Name    | string   | yes      | Event type identifier (e.g. `order.created`) |
| EventID | string   | yes      | Unique event ID (UUID)                       |
| Data    | generic  | yes      | Application-defined payload                  |

---

### Command

A named instruction directed at a specific target service.

```go
// Command is a point-to-point instruction sent to a named service.
// Wire JSON: {"name":"...","commandId":"...","data":{...}}
type Command[T any] struct {
    Name      string `json:"name"`
    CommandID string `json:"commandId"`
    Data      T      `json:"data"`
}
```

| Field     | Type     | Required | Description                                 |
|-----------|----------|----------|---------------------------------------------|
| Name      | string   | yes      | Command type identifier (e.g. `send-invoice`) |
| CommandID | string   | yes      | Unique command ID (UUID)                    |
| Data      | generic  | yes      | Application-defined payload                 |

---

### AsyncQuery

A named request-for-information sent to a specific service, expecting a typed reply.

```go
// AsyncQuery is a request/reply message. The resource identifies the query type.
// Wire JSON: {"resource":"...","queryData":{...}}
type AsyncQuery[T any] struct {
    Resource  string `json:"resource"`
    QueryData T      `json:"queryData"`
}
```

| Field     | Type     | Required | Description                                   |
|-----------|----------|----------|-----------------------------------------------|
| Resource  | string   | yes      | Query type identifier (e.g. `get-product`)    |
| QueryData | generic  | yes      | Application-defined query parameters          |

---

### Notification

A broadcast signal with non-durable delivery semantics.

```go
// Notification is a non-durable broadcast message.
// Wire JSON identical to DomainEvent: {"name":"...","eventId":"...","data":{...}}
type Notification[T any] struct {
    Name    string `json:"name"`
    EventID string `json:"eventId"`
    Data    T      `json:"data"`
}
```

**Note**: Notifications share the same wire format as domain events; they differ only in
queue durability and delivery semantics (non-persistent, auto-delete queues).

---

### From (Query Reply Context)

Carries the metadata needed to route a query reply back to the caller.

```go
// From holds reply routing metadata extracted from an incoming query.
type From struct {
    CorrelationID string // x-correlation-id header value
    ReplyID       string // x-reply_id header value (reply queue routing key)
}
```

---

### RawEnvelope (internal use)

Used by the internal converter for two-phase deserialization.

```go
// rawEnvelope holds a partially-deserialized message for deferred payload parsing.
type rawEnvelope struct {
    Name      string          `json:"name,omitempty"`
    EventID   string          `json:"eventId,omitempty"`
    CommandID string          `json:"commandId,omitempty"`
    Resource  string          `json:"resource,omitempty"`
    Data      json.RawMessage `json:"data,omitempty"`
    QueryData json.RawMessage `json:"queryData,omitempty"`
}
```

---

## Handler Types (`pkg/async/handlers.go`)

```go
// EventHandler processes a received domain event. Returns an error to trigger redelivery.
type EventHandler[T any] func(ctx context.Context, event DomainEvent[T]) error

// CommandHandler processes a received command. Returns an error to trigger DLQ routing.
type CommandHandler[T any] func(ctx context.Context, cmd Command[T]) error

// QueryHandler processes a query and returns the response payload or an error.
type QueryHandler[Req, Res any] func(ctx context.Context, query AsyncQuery[Req]) (Res, error)

// NotificationHandler processes a received notification.
type NotificationHandler[T any] func(ctx context.Context, n Notification[T]) error
```

---

## Core Interfaces (`pkg/async/`)

### HandlerRegistry

```go
// HandlerRegistry is the runtime catalog of message handlers for a service instance.
type HandlerRegistry interface {
    // ListenEvent registers a handler for the named domain event type.
    ListenEvent(eventName string, handler EventHandler[any]) error

    // ListenCommand registers a handler for the named command type.
    ListenCommand(commandName string, handler CommandHandler[any]) error

    // ServeQuery registers a handler that responds to the named query type.
    ServeQuery(queryName string, handler QueryHandler[any, any]) error

    // ListenNotification registers a handler for the named notification type.
    ListenNotification(notificationName string, handler NotificationHandler[any]) error
}
```

### DomainEventBus

```go
// DomainEventBus publishes domain events to all subscribers.
type DomainEventBus interface {
    // Emit publishes a domain event. Blocks until broker acknowledgement (publisher confirm).
    Emit(ctx context.Context, event DomainEvent[any]) error
}
```

### DirectAsyncGateway

```go
// DirectAsyncGateway sends commands and executes queries targeting specific services.
type DirectAsyncGateway interface {
    // SendCommand delivers a command to the named target service.
    SendCommand(ctx context.Context, cmd Command[any], targetService string) error

    // RequestReply sends a query and waits for a typed response.
    // Blocks until response received or ctx deadline exceeded.
    RequestReply(ctx context.Context, query AsyncQuery[any], targetService string) (json.RawMessage, error)

    // Reply sends a query response back to the caller.
    Reply(ctx context.Context, response any, from From) error
}
```

---

## Configuration Types (`rabbit/config.go`)

```go
// RabbitConfig holds all configuration for a RabbitMQ-backed application instance.
type RabbitConfig struct {
    // Connection
    Host        string // default: "localhost"
    Port        int    // default: 5672
    Username    string // default: "guest"
    Password    string
    VirtualHost string // default: "/"
    AppName     string // required; used for queue naming

    // Exchange names (defaults match reactive-commons-java)
    DomainEventsExchange  string // default: "domainEvents"
    DirectMessagesExchange string // default: "directMessages"
    GlobalReplyExchange   string // default: "globalReply"

    // Behaviour
    PrefetchCount     int           // default: 250
    ReplyTimeout      time.Duration // default: 15s
    PersistentEvents  bool          // default: true
    PersistentCommands bool         // default: true
    PersistentQueries  bool         // default: false
    WithDLQRetry      bool          // default: false
    RetryDelay        time.Duration // default: 1s (used if WithDLQRetry=true)
}
```

---

## State Transitions

### Query Lifecycle

```
Caller                    ReplyRouter              QueryHandler (remote)
  |                           |                            |
  |--register(correlationID)->|                            |
  |  (creates chan []byte)     |                            |
  |                           |                            |
  |--publish query (AMQP)------------------------------------>|
  |  headers: x-correlation-id, x-reply_id, x-serveQuery-id  |
  |                           |                            |
  |                           |                  processes |
  |                           |<--reply (AMQP)-------------|
  |                           |  header: x-correlation-id  |
  |                           |                            |
  |<--receive from chan--------|                            |
  |  (or ctx timeout fires)   |                            |
```

### Connection Recovery

```
Connected → [broker down] → Reconnecting (exponential backoff, max 30s)
         ↑                         |
         └─────────────────────────┘
              [broker up] → re-declare topology → re-start consumers
```

---

## Wire Format Reference Summary

| Message Type   | Exchange          | Routing Key                 | JSON Root Fields                              |
|----------------|-------------------|-----------------------------|-----------------------------------------------|
| Domain Event   | `domainEvents`    | event name                  | `name`, `eventId`, `data`                     |
| Command        | `directMessages`  | target service name         | `name`, `commandId`, `data`                   |
| Query Request  | `directMessages`  | `{targetService}.query`     | `resource`, `queryData`                       |
| Query Reply    | `globalReply`     | value of `x-reply_id`       | raw response JSON (or `null`)                 |
| Notification   | `domainEvents`    | notification name           | `name`, `eventId`, `data`                     |
