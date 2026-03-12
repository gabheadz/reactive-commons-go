---

description: "Task list for reactive-commons-go RabbitMQ implementation"
---

# Tasks: Go Core Messaging Library (RabbitMQ)

**Input**: Design documents from `/specs/001-go-core-messaging/`
**Prerequisites**: plan.md ✅ | spec.md ✅ | research.md ✅ | data-model.md ✅ | contracts/ ✅

**Tests**: ⚠️ TDD is NON-NEGOTIABLE per Constitution Principle III. Test tasks MUST be written
before implementation tasks within each story. Tests MUST fail (RED) before implementation begins.

**Organization**: Tasks are grouped by user story to enable independent implementation and
testing. All file paths are relative to the repository root.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US5)
- Include exact file paths in descriptions
- **RED → GREEN**: Within each story, write failing tests before any implementation

---

## Phase 1: Setup

**Purpose**: Initialize the Go module and project scaffold

- [X] T001 Initialize Go module in `reactive-commons-go/` with `go mod init github.com/bancolombia/reactive-commons-go`
- [X] T002 [P] Add dependencies to `reactive-commons-go/go.mod`: `github.com/rabbitmq/amqp091-go`, `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go`
- [X] T003 [P] Create directory scaffold: `reactive-commons-go/pkg/async/`, `reactive-commons-go/pkg/headers/`, `reactive-commons-go/internal/rabbit/`, `reactive-commons-go/internal/utils/`, `reactive-commons-go/rabbit/`, `reactive-commons-go/tests/unit/`, `reactive-commons-go/tests/integration/`
- [X] T004 [P] Create `.golangci.yml` linting config in `reactive-commons-go/.golangci.yml` with `go vet`, `staticcheck`, `errcheck`, `govet` enabled
- [X] T005 [P] Create `reactive-commons-go/Makefile` with targets: `test`, `test-unit`, `test-integration`, `lint`, `build`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, interfaces, and shared infrastructure that ALL user stories depend on.

⚠️ **CRITICAL**: No user story work can begin until this phase is complete.

### Public API Types and Interfaces

- [X] T006 [P] Define core message types in `reactive-commons-go/pkg/async/types.go`: `DomainEvent[T]`, `Command[T]`, `AsyncQuery[T]`, `Notification[T]`, `From` structs with correct JSON tags matching Java wire format (`name`/`eventId`/`data`, `commandId`, `resource`/`queryData`)
- [X] T007 [P] Define handler function types in `reactive-commons-go/pkg/async/handlers.go`: `EventHandler[T]`, `CommandHandler[T]`, `QueryHandler[Req,Res]`, `NotificationHandler[T]`
- [X] T008 [P] Define sentinel error types in `reactive-commons-go/pkg/async/errors.go`: `ErrDuplicateHandler`, `ErrQueryTimeout`, `ErrBrokerUnavailable`, `ErrDeserialize`
- [X] T009 [P] Define wire header constants in `reactive-commons-go/pkg/headers/headers.go`: `ReplyID`, `CorrelationID`, `CompletionOnlySignal`, `ServedQueryID`, `SourceApplication`, `ReplyTimeoutMillis` — values must match Java `Headers.java` exactly
- [X] T010 Define `HandlerRegistry` interface in `reactive-commons-go/pkg/async/registry.go` with `ListenEvent`, `ListenCommand`, `ServeQuery`, `ListenNotification` — each returns error on duplicate registration
- [X] T011 [P] Define `DomainEventBus` interface in `reactive-commons-go/pkg/async/eventbus.go`: `Emit(ctx, DomainEvent[any]) error` and `EmitNotification(ctx, Notification[any]) error`
- [X] T012 [P] Define `DirectAsyncGateway` interface in `reactive-commons-go/pkg/async/gateway.go`: `SendCommand`, `RequestReply`, `Reply` with exact signatures from `contracts/interfaces.md`
- [X] T013 [P] Define `Application` interface in `reactive-commons-go/pkg/async/application.go`: `Registry()`, `EventBus()`, `Gateway()`, `Start(ctx) error`

### Shared Infrastructure

- [X] T014 Implement JSON two-phase converter in `reactive-commons-go/internal/rabbit/converter.go`: `ToMessage(any) ([]byte, error)`, `ReadDomainEvent([]byte, target) error`, `ReadCommand([]byte, target) error`, `ReadQuery([]byte, target) error` — uses `json.RawMessage` for deferred payload deserialization
- [X] T015 [P] Write unit tests (RED first) for converter in `reactive-commons-go/tests/unit/converter_test.go`: assert serialization of DomainEvent/Command/AsyncQuery matches Java fixture bytes from `contracts/wire-format.md`; assert deserialization round-trips correctly
- [X] T016 Implement queue/exchange name generator in `reactive-commons-go/internal/utils/namegen.go` mirroring Java `NameGenerator`: `FromNameWithSuffix(appName, suffix string) string` and `GenerateTempName(appName, suffix string) string` (URL-safe base64 UUID)
- [X] T017 [P] Write unit tests (RED first) for name generator in `reactive-commons-go/tests/unit/namegen_test.go`: assert `FromNameWithSuffix("svc","subsEvents")` → `"svc.subsEvents"`, assert temp name format `"svc.replies.<base64>"`, assert idempotent per instance
- [X] T018 Implement AMQP connection + channel pool in `reactive-commons-go/internal/rabbit/connection.go`: `NewConnection(cfg)`, `Channel() (*amqp.Channel, error)`, `Close()` — single connection, 4-channel publisher pool with round-robin, reconnect hook registration
- [X] T019 Implement topology creator in `reactive-commons-go/internal/rabbit/topology.go`: `DeclareExchanges()`, `DeclareEventsQueue(appName)`, `DeclareCommandsQueue(appName)`, `DeclareQueriesQueue(appName)`, `DeclareTempQueue(name)`, `BindQueue(queue, exchange, routingKey)`, `DeclareDLQ(queue, exchange)` — exact exchange names/types from `contracts/wire-format.md`
- [X] T020 Implement message sender in `reactive-commons-go/internal/rabbit/sender.go`: `SendWithConfirm(ctx, body, exchange, routingKey, headers, persistent) error` (publisher confirms), `SendNoConfirm(body, exchange, routingKey, headers, persistent)`
- [X] T021 Implement reply router in `reactive-commons-go/internal/rabbit/replyrouter.go`: `Register(correlationID) chan []byte`, `Route(correlationID, body []byte)`, `Deregister(correlationID)` — backed by `sync.Map`
- [X] T022 [P] Write unit tests (RED first) for reply router in `reactive-commons-go/tests/unit/replyrouter_test.go`: assert channel receives routed message, assert deregister cleans up, assert routing unknown correlationID is no-op
- [X] T023 Implement in-memory handler registry in `reactive-commons-go/internal/rabbit/registry.go`: stores handlers by name, returns `ErrDuplicateHandler` on duplicate, thread-safe reads after `Start()`
- [X] T024 Implement `RabbitConfig` struct with defaults in `reactive-commons-go/rabbit/config.go`: all fields from `data-model.md`; `Validate() error` must reject empty `AppName`

**Checkpoint**: Foundation ready — public API compiled, shared infra complete, all unit tests GREEN.

---

## Phase 3: User Story 1 — Domain Events (Priority: P1) 🎯 MVP

**Goal**: Publisher emits a named event → all registered subscribers receive it with correct payload.

**Independent Test**: Run `tests/integration/events_test.go` with a live RabbitMQ container.
Assert handler is invoked with exact payload. No other patterns required.

### Tests for User Story 1 (TDD — write first, must be RED before implementation) ⚠️

> **Write these tests FIRST. Confirm `go test` FAILS before writing any implementation.**

- [X] T025 [P] [US1] Write integration test: single subscriber receives event in `reactive-commons-go/tests/integration/events_test.go` — spin up RabbitMQ via testcontainers-go, register handler for `"order.created"`, emit event, assert handler called with correct `OrderID` field
- [X] T026 [P] [US1] Write integration test: two subscribers both receive same event in `reactive-commons-go/tests/integration/events_test.go` — two independent apps, same event type, assert both handlers fire
- [X] T027 [P] [US1] Write integration test: unregistered event is silently ignored in `reactive-commons-go/tests/integration/events_test.go` — emit `"order.updated"` with no handler registered, assert no error and no panic

### Implementation for User Story 1

- [X] T028 [US1] Implement RabbitMQ domain event bus in `reactive-commons-go/internal/rabbit/eventbus.go`: `Emit` publishes with publisher confirm to `domainEvents` exchange, routing key = event name, persistent delivery mode 2
- [X] T029 [US1] Implement domain event listener in `reactive-commons-go/internal/rabbit/eventlistener.go`: declare `{appName}.subsEvents` queue, bind each registered event name to `domainEvents` exchange, consume messages, deserialize envelope, dispatch to registered handler, nack on handler error with panic recovery
- [X] T030 [US1] Implement `NewRabbitApplication` factory in `reactive-commons-go/rabbit/builder.go`: wire `RabbitConfig` → `connection` → `topology` → `registry` → `eventbus` → `eventlistener`, implement `Application` interface, `Start(ctx)` declares topology then starts event listener goroutine

**Checkpoint**: `go test -tags integration ./tests/integration/ -run TestEvents` MUST pass GREEN.

---

## Phase 4: User Story 2 — Commands (Priority: P2)

**Goal**: Sender dispatches a named command to a target service → that service's handler executes with correct payload.

**Independent Test**: Run `tests/integration/commands_test.go`. Assert handler fires with correct payload.

### Tests for User Story 2 (TDD — RED first) ⚠️

> **Verify T031–T033 FAIL before writing T034–T036 implementation.**

- [X] T031 [P] [US2] Write integration test: command delivered to target service in `reactive-commons-go/tests/integration/commands_test.go` — two apps, sender calls `SendCommand` targeting receiver's `AppName`, assert receiver handler invoked with correct `InvoiceID`
- [X] T032 [P] [US2] Write integration test: command with no registered handler is discarded gracefully in `reactive-commons-go/tests/integration/commands_test.go` — send command to service that has no handler for it; assert no crash, no blocking
- [X] T033 [P] [US2] Write unit test: duplicate command handler registration returns `ErrDuplicateHandler` in `reactive-commons-go/tests/unit/registry_test.go`

### Implementation for User Story 2

- [X] T034 [US2] Implement `SendCommand` in `reactive-commons-go/internal/rabbit/gateway.go`: publish with confirm to `directMessages` exchange, routing key = `targetService` (or `targetService-delayed` if delay > 0), delivery mode from config
- [X] T035 [US2] Implement command listener in `reactive-commons-go/internal/rabbit/commandlistener.go`: declare `{appName}` queue bound to `directMessages`, consume messages, deserialize `Command` envelope, dispatch to registered handler by command name, nack on error, panic recovery
- [X] T036 [US2] Extend `reactive-commons-go/rabbit/builder.go` to wire `DirectAsyncGateway` with `SendCommand` support and start command listener goroutine in `Start()`

**Checkpoint**: `go test -tags integration ./tests/integration/ -run TestCommands` MUST pass GREEN.

---

## Phase 5: User Story 3 — Async Queries (Priority: P3)

**Goal**: Querying service sends a named query → handler on target service processes it → response returned to caller within timeout.

**Independent Test**: Run `tests/integration/queries_test.go`. Assert caller receives correct typed response. Assert timeout error when no handler responds.

### Tests for User Story 3 (TDD — RED first) ⚠️

> **Verify T037–T040 FAIL before writing T041–T045 implementation.**

- [X] T037 [P] [US3] Write integration test: successful query/reply round-trip in `reactive-commons-go/tests/integration/queries_test.go` — two apps, caller sends `"get-product"` query, handler returns `Product` struct, caller receives and deserializes correctly
- [X] T038 [P] [US3] Write integration test: query times out when no handler responds in `reactive-commons-go/tests/integration/queries_test.go` — send query to service with no handler, use 500ms timeout context, assert `ErrQueryTimeout` (or context deadline) returned
- [X] T039 [P] [US3] Write integration test: multiple concurrent queries are correctly correlated in `reactive-commons-go/tests/integration/queries_test.go` — fire 10 concurrent queries, assert each response matches its own request payload
- [X] T040 [P] [US3] Write unit test: late reply after timeout is silently discarded in `reactive-commons-go/tests/unit/replyrouter_test.go` — route message to already-deregistered correlationID, assert no panic, no channel leak

### Implementation for User Story 3

- [X] T041 [US3] Implement `RequestReply` in `reactive-commons-go/internal/rabbit/gateway.go`: generate correlationID, register reply channel in `replyrouter`, publish to `directMessages` exchange with routing key `{targetService}.query`, set headers (`x-reply_id`, `x-correlation-id`, `x-serveQuery-id`, `x-reply-timeout-millis`), block on channel receive with `context` deadline, deregister on timeout
- [X] T042 [US3] Implement `Reply` in `reactive-commons-go/internal/rabbit/gateway.go`: publish response body (or `null` + `x-empty-completion` header) to `globalReply` exchange, routing key = `from.ReplyID`, header `x-correlation-id` = `from.CorrelationID`
- [X] T043 [US3] Implement query listener in `reactive-commons-go/internal/rabbit/querylistener.go`: declare `{appName}.query` queue bound to `directMessages`, consume messages, deserialize `AsyncQuery` envelope, build `From` from AMQP headers, dispatch to `QueryHandler`, call `Reply` with result or error
- [X] T044 [US3] Implement reply queue consumer in `reactive-commons-go/internal/rabbit/replylistener.go`: declare temp reply queue `{appName}.replies.{uuid}`, bind to `globalReply` exchange, consume replies, extract `x-correlation-id` header, call `router.Route(correlationID, body)`
- [X] T045 [US3] Extend `reactive-commons-go/rabbit/builder.go` to wire query listener and reply listener goroutines in `Start()`, pass reply queue name into gateway as `x-reply_id` value

**Checkpoint**: `go test -tags integration ./tests/integration/ -run TestQueries` MUST pass GREEN.

---

## Phase 6: User Story 4 — Notifications (Priority: P4)

**Goal**: Notifier broadcasts a named notification → all currently connected subscribers receive it; non-durable (missed notifications are acceptable).

**Independent Test**: Run `tests/integration/notifications_test.go`. Assert handler fires for connected subscriber; assert late-joiner does NOT receive past notifications.

### Tests for User Story 4 (TDD — RED first) ⚠️

> **Verify T046–T048 FAIL before writing T049–T051 implementation.**

- [X] T046 [P] [US4] Write integration test: connected subscriber receives notification in `reactive-commons-go/tests/integration/notifications_test.go` — two apps, subscriber registers for `"cache-invalidated"`, sender broadcasts notification, assert handler invoked with correct payload
- [X] T047 [P] [US4] Write integration test: late-joining subscriber does NOT receive past notification in `reactive-commons-go/tests/integration/notifications_test.go` — emit notification, THEN start second app with handler, wait 200ms, assert handler was NOT called
- [X] T048 [P] [US4] Write integration test: no error when broadcasting with zero subscribers in `reactive-commons-go/tests/integration/notifications_test.go` — emit notification with no handlers registered anywhere, assert no error returned

### Implementation for User Story 4

- [X] T049 [US4] Implement `EmitNotification` in `reactive-commons-go/internal/rabbit/eventbus.go`: publish non-persistent (delivery-mode 1) to `domainEvents` exchange, routing key = notification name, no publisher confirm required
- [X] T050 [US4] Implement notification listener in `reactive-commons-go/internal/rabbit/notificationlistener.go`: declare auto-delete exclusive temp queue `{appName}.notification.{uuid}`, bind each registered notification name to `domainEvents` exchange, consume, dispatch to handler, panic recovery, error logged only (no redeliver)
- [X] T051 [US4] Extend `reactive-commons-go/rabbit/builder.go` to wire notification listener goroutine in `Start()` when any notification handlers are registered

**Checkpoint**: `go test -tags integration ./tests/integration/ -run TestNotifications` MUST pass GREEN.

---

## Phase 7: User Story 5 — Java Interoperability (Priority: P5)

**Goal**: Go services and Java services exchange all four message types transparently on the same broker using identical wire format.

**Independent Test**: Run `tests/integration/java_interop_test.go` using Java fixture bytes from `contracts/wire-format.md`. Assert all fixture messages deserialize correctly.

### Tests for User Story 5 (TDD — RED first) ⚠️

> **Verify T052–T055 FAIL before writing T056–T058 fixes.**

- [X] T052 [P] [US5] Write unit test: deserialize Java DomainEvent fixture in `reactive-commons-go/tests/unit/java_interop_test.go` — unmarshal exact bytes `{"name":"order.created","eventId":"d290f1ee-...","data":{"orderId":"42","amount":100}}`, assert all fields correct
- [X] T053 [P] [US5] Write unit test: deserialize Java Command fixture in `reactive-commons-go/tests/unit/java_interop_test.go` — unmarshal exact bytes `{"name":"send-invoice","commandId":"a3bb189e-...","data":{"invoiceId":"INV-001"}}`, assert all fields correct
- [X] T054 [P] [US5] Write unit test: deserialize Java Query fixture in `reactive-commons-go/tests/unit/java_interop_test.go` — unmarshal exact bytes `{"resource":"get-product","queryData":{"productId":"SKU-999"}}`, assert all fields correct
- [X] T055 [P] [US5] Write unit test: Go-serialized DomainEvent byte-matches Java expected format in `reactive-commons-go/tests/unit/java_interop_test.go` — marshal `DomainEvent{Name:"order.created",EventID:"...",Data:...}`, assert JSON contains expected field names and structure

### Implementation for User Story 5

- [X] T056 [US5] Verify and fix DomainEvent + Notification JSON field mapping in `reactive-commons-go/internal/rabbit/converter.go` — ensure Go `json` tags match Java Jackson output (`eventId` not `event_id`, `data` not `payload`)
- [X] T057 [US5] Verify and fix Command JSON field mapping in `reactive-commons-go/internal/rabbit/converter.go` — ensure `commandId` (not `command_id`), `data` (not `body`)
- [X] T058 [US5] Verify and fix Query JSON field mapping in `reactive-commons-go/internal/rabbit/converter.go` — ensure `resource` (not `name`), `queryData` (not `query_data`)

**Checkpoint**: `go test ./tests/unit/ -run TestJavaInterop` MUST pass GREEN.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Resilience, observability, and developer experience improvements spanning all stories.

- [X] T059 [P] Implement auto-reconnect with exponential backoff in `reactive-commons-go/internal/rabbit/connection.go`: on `amqp.ErrClosed`, retry with 1s→2s→4s→…→30s cap; re-declare topology and restart all consumer goroutines after reconnect
- [X] T060 [P] Implement graceful shutdown in `reactive-commons-go/rabbit/builder.go`: on `ctx.Done()`, stop accepting new messages, wait for in-flight handlers to complete (with 30s hard timeout), then close AMQP connection
- [X] T061 [P] Add panic recovery wrapper in `reactive-commons-go/internal/rabbit/eventlistener.go`, `commandlistener.go`, `querylistener.go`, `notificationlistener.go` — recover from handler panics, log with stack trace, nack message, continue consuming
- [X] T062 [P] Add optional DLQ declaration in `reactive-commons-go/internal/rabbit/topology.go`: when `WithDLQRetry=true`, declare `{exchange}.DLQ` direct exchange and `{queue}.DLQ` queue with `x-dead-letter-exchange` and `x-message-ttl` args; bind DLQ
- [X] T063 [P] Write integration test for panic recovery in `reactive-commons-go/tests/integration/events_test.go`: register panicking handler, send event, assert consumer goroutine continues processing subsequent events
- [X] T064 [P] Write README.md in `reactive-commons-go/README.md`: installation, all four pattern examples, Java interop note, configuration reference, link to quickstart
- [X] T065 Run `go vet ./...` and `golangci-lint run ./...` across `reactive-commons-go/` — resolve all linting errors
- [X] T066 [P] Run full test suite `go test ./tests/unit/... && go test -tags integration ./tests/integration/...` — all tests GREEN, coverage ≥ 70% on `internal/rabbit/`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — **BLOCKS all user stories**
- **US1 Domain Events (Phase 3)**: Depends on Phase 2 — no story dependencies
- **US2 Commands (Phase 4)**: Depends on Phase 2 — independent of US1
- **US3 Queries (Phase 5)**: Depends on Phase 2 — independent of US1/US2
- **US4 Notifications (Phase 6)**: Depends on Phase 2 and US1 (shares event bus/topology)
- **US5 Java Interop (Phase 7)**: Depends on Phases 3–6 complete (validates all patterns)
- **Polish (Phase 8)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Start after Phase 2 — no story dependencies
- **US2 (P2)**: Start after Phase 2 — independent; can run in parallel with US1
- **US3 (P3)**: Start after Phase 2 — independent; can run in parallel with US1/US2
- **US4 (P4)**: Start after Phase 2 — shares `DomainEventBus`; best after US1 to reuse eventbus.go
- **US5 (P5)**: Start after US1–US4 complete — validates all patterns against Java fixtures

### Within Each User Story (TDD Order)

1. Write tests → confirm RED
2. Implement until GREEN
3. Refactor if needed
4. Proceed to next story

### Parallel Opportunities (within Foundational)

All of T006–T009 can run in parallel (different files, no inter-dependencies).
T014/T015 (converter + tests), T016/T017 (namegen + tests), T021/T022 (reply router + tests)
can each be started independently once T006–T009 are done.

---

## Parallel Example: User Story 1

```bash
# Write all US1 integration tests together (TDD RED phase):
Task T025: "Write events_test.go - single subscriber test"
Task T026: "Write events_test.go - two subscribers test"
Task T027: "Write events_test.go - unregistered event ignored test"

# After confirming all RED — implement together:
Task T028: "Implement internal/rabbit/eventbus.go"
Task T029: "Implement internal/rabbit/eventlistener.go"
# T030 depends on T028+T029 (builder wires them):
Task T030: "Implement rabbit/builder.go for event wiring"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Write US1 tests (T025–T027), confirm RED
4. Complete Phase 3: User Story 1 (T028–T030)
5. **STOP and VALIDATE**: `go test -tags integration ./tests/integration/ -run TestEvents` GREEN
6. Library is usable for domain events — demonstrates real value

### Incremental Delivery

1. Setup + Foundational → Core compiled, unit tests GREEN
2. Add US1 → Domain events working → validate independently → **first publishable MVP**
3. Add US2 → Commands working → validate independently
4. Add US3 → Queries working → validate independently
5. Add US4 → Notifications working → validate independently
6. Add US5 → Java interop verified → **production-ready for mixed deployments**
7. Polish → Production hardening

### Parallel Team Strategy

With multiple developers after Phase 2 completes:

- **Developer A**: US1 (Domain Events)
- **Developer B**: US2 (Commands)
- **Developer C**: US3 (Queries)

US4 (Notifications) can follow US1 since they share `eventbus.go`.
US5 (Java Interop) integrates after all stories are green.

---

## Notes

- `[P]` tasks = different files, no inter-dependencies; safe to run in parallel
- `[Story]` label maps each task to its user story for traceability
- **TDD is mandatory** (Constitution III): tests written before implementation, must fail first
- Within each story, commit after RED phase and again after GREEN phase
- Stop at each story checkpoint to validate independently before next story
- Run `go test ./tests/unit/...` continuously during foundational phase (fast, no broker)
- Run `go test -tags integration ./tests/integration/ -run Test<Story>` to validate each story
- Java fixture bytes in `contracts/wire-format.md` are the ground truth for wire compatibility
