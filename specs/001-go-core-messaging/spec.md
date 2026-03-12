# Feature Specification: Go Core Messaging Library

**Feature Branch**: `001-go-core-messaging`
**Created**: 2026-03-11
**Status**: Draft
**Input**: Build a Go equivalent of reactive-commons-java, maintaining the same message patterns: Domain Events (pub/sub), Commands (point-to-point), Queries (async request/reply), and Notifications.

## Clarifications

### Session 2026-03-11

- Q: Does `DomainEvent` carry a `timestamp` field in its JSON body? → A: No — timestamp is an AMQP transport property only; JSON body contains `name`, `eventId`, `data` to match Java wire format.
- Q: How is a query handler error communicated back to the caller on the wire? → A: Error reply uses JSON body `{"errorMessage":"<description>"}` plus AMQP header `x-reply-error: true`; the caller receives a Go `error` wrapping the message string.
- Q: When multiple instances of the same service run simultaneously, what delivery semantics apply per pattern? → A: Commands: load-balanced (competing consumers — one instance handles each message); Events: load-balanced (competing consumers — one instance handles each event); Notifications: fan-out (all instances receive every notification).
- Q: What is the logging contract for the library? → A: Accept an injectable `*slog.Logger` (Go 1.21+ stdlib); default to `slog.Default()` when none provided. No third-party logging dependency.
- Q: What is the back-pressure strategy when all prefetch slots are occupied by slow handlers? → A: Prefetch count is the sole back-pressure control; when slots are exhausted the AMQP channel naturally stalls delivery. No additional library-level semaphore in v1. Users tune `RabbitConfig.Prefetch` to control concurrency.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Publish and Receive Domain Events (Priority: P1)

A developer building a Go microservice wants to publish domain events (things that happened)
so that other services can react to them independently. The publisher does not know or care which
services are listening. Subscribers register interest in specific event types by name and receive
a typed payload whenever a matching event is published. When multiple instances of the same
service are subscribed, only one instance processes each event (load-balanced / competing consumers).

**Why this priority**: Domain events are the most common pattern in event-driven microservices.
This story alone delivers a fully working pub/sub backbone and is the foundation all other patterns
build on.

**Independent Test**: Can be validated by running two isolated Go services — one that publishes a
named event and one that registers a handler for it — and asserting the handler is invoked with the
correct payload. No other patterns are required.

**Acceptance Scenarios**:

1. **Given** a service has registered a handler for event type `"order.created"`,
   **When** another service publishes an `"order.created"` event with an order payload,
   **Then** the handler is invoked exactly once with the correct payload content.

2. **Given** two services have each registered a handler for the same event type,
   **When** a publisher emits that event,
   **Then** both handlers are invoked independently.

3. **Given** a service has NOT registered a handler for `"order.updated"`,
   **When** that event is published,
   **Then** the service ignores it without errors.

4. **Given** the message broker is temporarily unavailable,
   **When** a publisher attempts to emit an event,
   **Then** the library returns a clear error and the application can retry or handle it.

---

### User Story 2 - Send and Handle Commands (Priority: P2)

A developer wants to send a point-to-point command to a specific named service and have that
service execute the corresponding business action. Unlike events, commands have a designated
target and the sender expects the target service to act on the instruction — but does not need
a response back.

**Why this priority**: Commands are essential for task delegation between services (e.g., "send
invoice", "charge payment"). This pattern is the second most common interaction type and has no
dependency on query or notification patterns.

**Independent Test**: Validated by running a sender and a receiver service. The sender dispatches
a named command; the receiver's registered handler is asserted to have been invoked with the
correct payload. No response is expected.

**Acceptance Scenarios**:

1. **Given** service B has registered a handler for command `"send-invoice"`,
   **When** service A sends a `"send-invoice"` command targeted at service B,
   **Then** service B's handler is invoked with the correct payload.

2. **Given** a command is sent to a service that has NO registered handler for it,
   **When** the message is delivered,
   **Then** the library discards it gracefully and logs a warning — no crash, no blocking.

3. **Given** a command is dispatched while the target service is offline,
   **When** the target service reconnects,
   **Then** the command is delivered and the handler is invoked (broker-side durability assumed).

---

### User Story 3 - Execute Async Queries (Priority: P3)

A developer needs to ask another service a question and wait for a response — an async
request/reply interaction. The querying service sends a query with a payload and expects a
typed response back within an acceptable wait time. This pattern allows services to fetch
information from each other without tight coupling.

**Why this priority**: Queries are critical for read-side service composition (e.g., "get product
details", "validate coupon"). They depend on the basic messaging infrastructure established in
P1/P2 but are independently testable.

**Independent Test**: Validated by a querying service that sends a named query and awaits a
result, while a responder service has registered a handler that returns a hardcoded response.
The querying service asserts the received response matches what the responder returned.

**Acceptance Scenarios**:

1. **Given** service B has registered a query handler for `"get-product"`,
   **When** service A sends a `"get-product"` query with a product ID,
   **Then** service A receives the product data returned by service B's handler.

2. **Given** a query is sent but no handler responds within the configured timeout,
   **When** the timeout expires,
   **Then** the library returns a timeout error to the caller — no indefinite blocking.

3. **Given** the query handler returns an error,
   **When** service A receives the reply,
   **Then** service A receives a Go `error` (message from the handler); the reply body is
   `{"errorMessage":"<description>"}` with AMQP header `x-reply-error: true` — no silent
   empty response.

4. **Given** multiple concurrent queries are in flight,
   **When** responses arrive out of order,
   **Then** each response is correctly correlated to its original query.

---

### User Story 4 - Broadcast and Receive Notifications (Priority: P4)

A developer wants to send operational or informational notifications — messages that are
broadcast broadly but are not domain events tied to a business fact. Notifications may be
transient (fire-and-forget, no durable delivery required) and are typically used for
coordination signals, health updates, or cross-cutting concerns. Unlike domain events,
notifications fan-out to ALL running instances of a subscribed service.

**Why this priority**: Notifications round out the four-pattern library. They share infrastructure
with domain events but have distinct delivery semantics (non-durable, broadcast). They can be
delivered independently of the other patterns.

**Independent Test**: Validated by a notifier service sending a named notification and one or
more subscriber services asserting their handlers are invoked. Durability is explicitly NOT
required — if a subscriber is offline when the notification fires, it is acceptable to miss it.

**Acceptance Scenarios**:

1. **Given** a service has registered a notification handler for `"cache-invalidated"`,
   **When** another service broadcasts a `"cache-invalidated"` notification,
   **Then** the handler is invoked with the notification payload.

2. **Given** no service is subscribed to `"cache-invalidated"` at the time of broadcast,
   **When** the notification is sent,
   **Then** the library sends without error and the message is discarded silently.

3. **Given** a subscriber comes online after a notification was sent,
   **Then** the subscriber does NOT receive that missed notification (non-durable semantics).

---

### User Story 5 - Interoperate with Java Services (Priority: P5)

A developer running a mixed Java/Go microservices deployment wants Go services and Java services
to exchange messages transparently. A Go service publishing a domain event should be consumable
by a Java service using reactive-commons-java, and vice versa — without any message translation
layer.

**Why this priority**: This is a key differentiator of this library over building a standalone Go
solution. It enables incremental migration: teams can migrate services one at a time without
breaking existing Java consumers.

**Independent Test**: Validated by running a Java service (using reactive-commons-java) and a Go
service side by side on the same broker. Assert that a message published by the Java service is
received by the Go service handler, and a message published by the Go service is received by the
Java service handler.

**Acceptance Scenarios**:

1. **Given** a Java service publishes a domain event in the reactive-commons standard format,
   **When** a Go service with a matching handler is running,
   **Then** the Go handler is invoked with the correct deserialized payload.

2. **Given** a Go service publishes a domain event,
   **When** a Java service with a matching handler is running,
   **Then** the Java handler is invoked correctly — no message format translation needed.

3. **Given** a Go service sends a command to a Java service,
   **When** the Java service has a registered handler,
   **Then** the Java handler executes the command successfully.

---

### Edge Cases

- What happens when a handler panics or throws an unhandled error during message processing?
  The library MUST recover gracefully (no process crash) and log the failure via the configured
  `slog.Logger`; the message should be negatively acknowledged so the broker can redeliver or
  route to a dead-letter queue.
- What happens when the broker connection drops mid-stream?
  The library MUST attempt reconnection automatically and resume consuming when connectivity
  is restored.
- What happens when a message payload cannot be deserialized into the expected type?
  The library MUST deliver a descriptive deserialization error to the handler or discard with
  a logged error — it MUST NOT crash.
- What happens when two handlers are registered for the same event/command name?
  The library MUST raise a clear error at registration time — duplicate registrations are not
  allowed.
- What happens when a query reply arrives after the caller has already timed out?
  The late reply MUST be silently discarded without affecting other in-flight queries.
- What happens when all prefetch slots are consumed by slow handlers?
  The AMQP channel stalls naturally — no new messages are delivered until in-flight ones are
  acknowledged. This is the intended back-pressure behavior. Users MUST tune `RabbitConfig.Prefetch`
  to match their concurrency needs; no additional library-level semaphore exists in v1.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The library MUST allow applications to publish named domain events with an
  arbitrary typed payload to any number of interested subscribers.
- **FR-002**: The library MUST allow applications to register named handlers for domain events
  and receive a typed payload upon delivery.
- **FR-003**: The library MUST allow applications to send named commands with a typed payload
  to a specific target service by name.
- **FR-004**: The library MUST allow applications to register named command handlers and execute
  business logic upon receipt.
- **FR-005**: The library MUST allow applications to send named queries with a typed payload and
  receive a typed response from a remote handler.
- **FR-006**: The library MUST allow applications to register named query handlers that return a
  typed response to the caller.
- **FR-007**: The library MUST allow applications to publish named notifications with a typed
  payload in a non-durable, broadcast fashion.
- **FR-008**: The library MUST allow applications to register named notification handlers.
- **FR-009**: The library MUST allow handler registration both at startup and dynamically at
  runtime without service restart.
- **FR-010**: Query execution MUST support a configurable timeout; the library MUST return a
  timeout error when the deadline is exceeded.
- **FR-011**: The library MUST recover from broker connection failures automatically and resume
  processing without application intervention.
- **FR-016**: When a query handler returns an error, the library MUST send an error reply
  envelope (`{"errorMessage":"<description>"}`) with AMQP header `x-reply-error: true`; the
  querying service MUST receive a Go `error` (not a timeout and not a nil response).
- **FR-012**: The library MUST support graceful shutdown: in-flight message processing MUST
  complete before the application exits; no messages MUST be lost during shutdown.
- **FR-013**: Message envelopes exchanged over the broker MUST be structurally identical to those
  produced by reactive-commons-java, enabling transparent interoperability.
- **FR-014**: Handler errors (including panics) MUST be isolated — one failing handler MUST NOT
  affect other handlers or crash the application.
- **FR-015**: The library MUST support at minimum one production-grade message broker.
- **FR-017**: Domain events and commands MUST use competing-consumer delivery (load-balanced
  across instances of the same service — exactly one instance processes each message). Notifications
  MUST use fan-out delivery (all running instances of every subscribed service receive the message).
- **FR-018**: The library MUST accept an optional `*slog.Logger` (Go standard library) at
  construction time and use it for all internal diagnostic output (connection lifecycle, handler
  errors, retry attempts). When no logger is provided, the library MUST default to `slog.Default()`.
  No third-party logging library MUST be introduced as a dependency.
- **FR-019**: Back-pressure is controlled exclusively via `RabbitConfig.Prefetch` (default 250).
  When all prefetch slots are occupied, the AMQP channel naturally stalls delivery of new messages
  until in-flight messages are acknowledged. No additional library-level concurrency semaphore is
  provided in v1.

### Key Entities

- **Domain Event**: A named record of something that happened. Carries an event name, a unique
  ID, and a typed payload (`name`, `eventId`, `data`). When multiple instances of the same
  service subscribe, only one instance processes each event (competing consumers / load-balanced).
  Timestamp is an AMQP transport property only, NOT a JSON body field.
- **Command**: A named instruction directed at a specific service. Carries a command name, a
  unique ID, and a typed payload. Delivered to exactly one instance of the target service
  (load-balanced / competing consumers; 1-to-1 per-service-instance).
- **Query**: A named request for information sent to a specific service. Carries a query name,
  a correlation ID, and a typed request payload. Returns a typed response to the caller.
- **Notification**: A named broadcast signal. Carries a notification name and a typed payload.
  Delivered to ALL currently connected instances of every subscribed service (fan-out); non-durable.
- **Message Envelope**: The wire-level wrapper around any message type. Contains routing
  metadata (name, source service, correlation ID) and the serialized payload. Format is shared
  with reactive-commons-java for interoperability.
- **Handler**: A developer-registered function associated with a message name. Invoked by the
  library when a matching message arrives; returns a response payload (queries only) or an error.
- **Handler Registry**: The runtime catalog of all registered handlers within a service instance.
  Supports dynamic addition of handlers after startup.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can integrate the library into a new service, register a domain event
  handler, and successfully receive a published event in under 15 minutes using the provided
  documentation and examples.
- **SC-002**: A Go service and a Java service running side-by-side on the same broker exchange
  domain events, commands, and queries without any message translation or adapter layer.
- **SC-003**: The library processes at least 1,000 messages per second per pattern type (events,
  commands, queries) without message loss under sustained load.
- **SC-004**: Query response latency (round-trip, excluding network) adds no more than 5ms of
  overhead compared to direct broker round-trip time.
- **SC-005**: The library achieves zero message loss during graceful shutdown when at most 100
  messages are in-flight at shutdown time.
- **SC-006**: A developer familiar with reactive-commons-java can map every Java concept to its
  Go counterpart within 30 minutes by reading the library documentation alone.
- **SC-007**: 100% of registered handler errors (including panics) are recovered and logged
  without crashing the consuming application.

## Assumptions

- The initial implementation targets at least one message broker; RabbitMQ is assumed as the
  first implementation based on the Java project's primary broker support.
- Durable delivery (broker-side persistence) is assumed to be the default for events and
  commands; notifications are explicitly non-durable.
- Services are identified by a string name that is unique within a deployment; the library does
  not manage service discovery.
- Message payloads are serialized as JSON; binary or custom serialization formats are out of
  scope for this initial version.
- The library is distributed as an importable Go module, not as a standalone service or binary.
