# Research: Go Core Messaging Library — RabbitMQ Wire Protocol

**Feature**: 001-go-core-messaging
**Date**: 2026-03-11
**Source**: Direct analysis of `reactive-commons-java` source code

---

## Decision 1: RabbitMQ Client Library

**Decision**: Use `github.com/rabbitmq/amqp091-go`

**Rationale**: Official RabbitMQ-maintained Go AMQP 0-9-1 client. Actively maintained,
production-proven, and supports all required features (publisher confirms, consumer prefetch,
connection recovery hooks). It is the direct successor to `streadway/amqp`.

**Alternatives considered**:
- `streadway/amqp`: archived/unmaintained; superseded by `amqp091-go`
- `Azure/go-amqp`: AMQP 1.0 protocol — incompatible with RabbitMQ's default AMQP 0-9-1 port

---

## Decision 2: Exchange Topology (verified from Java source)

**Decision**: Replicate the Java exchange topology exactly.

| Exchange Name    | Type   | Durable | Purpose                                      |
|------------------|--------|---------|----------------------------------------------|
| `domainEvents`   | topic  | true    | Domain event pub/sub (routing key = event name) |
| `directMessages` | direct | true    | Commands and queries (routing key = service name) |
| `globalReply`    | direct | false   | Query reply correlation (routing key = reply queue name) |

**Rationale**: These names are the Java defaults (`EventsProps.exchange`, `DirectProps.exchange`,
`GlobalProps.exchange`). Matching them exactly achieves wire compatibility with zero configuration
for mixed Java/Go deployments.

**Source**: `starters/async-rabbit-starter/.../EventsProps.java`,
`DirectProps.java`, `GlobalProps.java`

---

## Decision 3: Queue Naming Conventions (verified from Java source)

**Decision**: Mirror Java's `NameGenerator` and `BrokerConfigProps` logic exactly.

| Queue                  | Name Pattern                               | Durable | Purpose                    |
|------------------------|--------------------------------------------|---------|----------------------------|
| Events queue           | `{appName}.subsEvents`                     | true    | Receives domain events      |
| Commands queue         | `{appName}` (no suffix by default)         | true    | Receives commands           |
| Queries queue          | `{appName}.query`                          | true    | Receives query requests     |
| Notifications queue    | `{appName}.notification.{base64UUID}`      | false   | Receives notifications (temp) |
| Reply queue            | `{appName}.replies.{base64UUID}`           | false   | Receives query replies (temp) |

Temporary queues (notifications, replies) use auto-delete and exclusive flags; their names are
generated once per application instance using a URL-safe base64-encoded UUID suffix.

**Source**: `BrokerConfigProps.java`, `NameGenerator.java`, `EventsProps.java`, `DirectProps.java`

---

## Decision 4: Queue Bindings

| Queue               | Exchange          | Routing Key              | Notes                          |
|---------------------|-------------------|--------------------------|--------------------------------|
| Events queue        | `domainEvents`    | each registered event name | One binding per handler        |
| Commands queue      | `directMessages`  | `{appName}`              | Receives all commands          |
| Queries queue       | `directMessages`  | `{appName}.query`        | Routing key = queue name       |
| Notifications queue | `domainEvents`    | each notification name   | Same exchange, different queue |
| Reply queue         | `globalReply`     | reply queue name (UUID)  | Unique per instance            |

---

## Decision 5: Message Envelope JSON Structure (verified from Java source)

The message body is always UTF-8 encoded JSON. The top-level structure mirrors the Java domain
objects serialized by Jackson.

### Domain Event
```json
{
  "name": "order.created",
  "eventId": "550e8400-e29b-41d4-a716-446655440000",
  "data": { ...application payload... }
}
```

### Command
```json
{
  "name": "send-invoice",
  "commandId": "550e8400-e29b-41d4-a716-446655440000",
  "data": { ...application payload... }
}
```

### Query Request
```json
{
  "resource": "get-product",
  "queryData": { ...application payload... }
}
```

### Query Reply
The reply body is the raw JSON serialization of the response object (not wrapped in an envelope).
A completion-only signal (nil response) sends a JSON `null` body with header
`x-empty-completion: true`.

**Source**: `DomainEvent.java`, `Command.java`, `AsyncQuery.java` (domain classes);
`JacksonMessageConverterTest.java` (serialization verification)

---

## Decision 6: AMQP Message Properties

Every published message sets these AMQP basic properties:

| Property          | Value                                      |
|-------------------|--------------------------------------------|
| `content-type`    | `application/json`                         |
| `delivery-mode`   | `2` (persistent) for events/commands; `1` (transient) for queries/replies |
| `app-id`          | source application name                    |
| `message-id`      | random UUID (no dashes)                    |
| `timestamp`       | Unix epoch milliseconds                    |

Default durability by message type (from `BrokerConfig`):
- Events: persistent (delivery-mode 2)
- Commands: persistent (delivery-mode 2)
- Queries: transient (delivery-mode 1)
- Replies: transient (delivery-mode 1)

---

## Decision 7: AMQP Headers for Queries and Replies

| Header Key                | Java Constant          | Direction     | Value                                    |
|---------------------------|------------------------|---------------|------------------------------------------|
| `x-reply_id`              | `REPLY_ID`             | Request→      | Reply queue name (routing key in globalReply) |
| `x-correlation-id`        | `CORRELATION_ID`       | Both          | UUID linking request to reply            |
| `x-serveQuery-id`         | `SERVED_QUERY_ID`      | Request→      | Query resource name (e.g., `get-product`) |
| `x-reply-timeout-millis`  | `REPLY_TIMEOUT_MILLIS` | Request→      | Timeout in milliseconds (default 15000)  |
| `x-empty-completion`      | `COMPLETION_ONLY_SIGNAL`| Reply→       | `"true"` when response payload is nil   |
| `sourceApplication`       | `SOURCE_APPLICATION`   | Both          | Publishing application name             |

**Source**: `Headers.java`, `RabbitDirectAsyncGateway.java`

---

## Decision 8: Query Reply Routing (Correlation)

**Decision**: Use an in-process `sync.Map` keyed by correlation ID, with `chan []byte` values.

**Flow**:
1. Sender generates `correlationID` = UUID (no dashes)
2. Sender registers `chan []byte` in router map before publishing
3. Sender publishes query with headers: `x-reply_id`, `x-correlation-id`, `x-serveQuery-id`, `x-reply-timeout-millis`
4. Query routing key = `{targetService}.query`
5. Handler receives query, processes, publishes reply to `globalReply` exchange
   with routing key = value of `x-reply_id` header from request
6. Reply consumer reads `x-correlation-id`, looks up chan in router, sends body
7. Sender's goroutine receives from chan or fires timeout via `context.WithTimeout`

**Source**: `ReactiveReplyRouter` pattern in Java; adapted to Go channels

---

## Decision 9: Connection & Channel Management Strategy

**Decision**: Single connection, goroutine-per-consumer model with manual reconnection.

- One AMQP connection per `RabbitApplication` instance
- Dedicated channel per consumer goroutine (one per queue listener)
- Dedicated channel pool for publishers (configurable, default 4)
- On connection loss: exponential backoff reconnection loop (max 30s between attempts)
- Prefetch count: configurable, default 250 (mirrors Java default)

**Rationale**: `amqp091-go` channels are not thread-safe; dedicated channels per goroutine
avoids synchronization overhead. Connection-level reconnect is simpler and sufficient.

---

## Decision 10: Dead Letter Queue (DLQ) Support

**Decision**: Declare DLQ for commands and events queues by default; disabled by default for
queries and notifications.

- DLQ exchange: `{originalExchange}.DLQ` (direct type, durable)
- DLQ queue: `{originalQueue}.DLQ` (durable, with TTL for retry)
- Default retry delay: 1000ms (mirrors Java default `retryDelay`)
- Default max retries: configurable; unlimited if DLQ disabled

**Source**: `TopologyCreator.declareDLQ()`, `GenericMessageListener`

---

## Decision 11: Serialization

**Decision**: Use Go's standard `encoding/json` package.

- No struct tags required beyond standard `json:"fieldName"` conventions
- Field names must match Java Jackson serialization output (camelCase)
- `DomainEvent`, `Command`, `AsyncQuery` Go structs use generic type parameter `[T any]`
  with `json.RawMessage` for the data/queryData field to enable two-phase deserialization
  (envelope first, then payload into user-defined type)

---

## Decision 12: Testing Strategy

**Decision**: Two-layer testing approach.

**Unit tests** (`tests/unit/`):
- Test JSON serialization/deserialization with fixture bytes
- Test name generation logic
- Test reply router channel management
- Test topology declaration logic with mock AMQP connection

**Integration tests** (`tests/integration/`):
- Use `testcontainers-go` to spin up a real RabbitMQ 3.x container
- One test file per pattern (events, commands, queries, notifications)
- Java interop fixture test: publish a message with Java-format bytes; assert Go handler
  receives and deserializes correctly
- All integration tests run with `go test -tags integration ./tests/integration/...`

**Source**: reactive-commons-java test patterns; Go testing conventions
