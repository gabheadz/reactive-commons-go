# Implementation Plan: Go Core Messaging Library (RabbitMQ)

**Branch**: `001-go-core-messaging` | **Date**: 2026-03-11 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-go-core-messaging/spec.md`

## Summary

Build `reactive-commons-go` — a Go library providing the four reactive-commons messaging patterns
(Domain Events, Commands, Queries, Notifications) over RabbitMQ. The library exposes broker-agnostic
Go interfaces backed by a RabbitMQ implementation that is wire-compatible with reactive-commons-java,
enabling mixed Java/Go microservice deployments. Implementation follows TDD with integration tests
using real RabbitMQ containers via testcontainers-go.

**Scope of this plan**: RabbitMQ implementation only. Kafka is a future phase.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: `rabbitmq/amqp091-go` (RabbitMQ AMQP client), `testify/assert` (test assertions), `testcontainers-go` (integration test containers)
**Storage**: N/A — message broker only; no persistent storage
**Testing**: `go test` (standard library) + `testify/assert`; integration tests via `testcontainers-go` with real RabbitMQ container
**Target Platform**: Linux server (library; embeds in any Go service)
**Project Type**: Go library (importable module, no binary)
**Performance Goals**: ≥1,000 messages/second per pattern type; query round-trip overhead ≤5ms
**Constraints**: Wire-compatible with reactive-commons-java message envelopes; no global state; no DI framework; graceful shutdown with zero message loss for ≤100 in-flight messages
**Scale/Scope**: Library supporting microservices at production scale; initial implementation covers all 4 patterns over RabbitMQ

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Notes |
|-----------|-------|-------|
| I. Go-Idiomatic Design | ✅ PASS | goroutines + channels + context.Context throughout; no Reactor ports |
| II. Behavioral Parity | ✅ PASS | All 4 patterns implemented; handler registration mirrors Java semantics |
| III. Test-First | ✅ PASS | TDD enforced; integration tests with testcontainers-go required |
| IV. Broker Abstraction | ✅ PASS | Core interfaces broker-agnostic; RabbitMQ is first implementation behind interface |
| V. Wire Protocol Compat | ✅ PASS | JSON envelopes and AMQP topology verified against Java source (see research.md) |
| VI. Simplicity & Explicit | ✅ PASS | Constructor functions; no DI; no global state; configuration passed explicitly |

*Post-design re-check: All principles remain satisfied. No complexity violations.*

## Project Structure

### Documentation (this feature)

```text
specs/001-go-core-messaging/
├── plan.md          # This file
├── research.md      # Phase 0 — wire protocol & topology research
├── data-model.md    # Phase 1 — Go types & message envelope model
├── quickstart.md    # Phase 1 — developer integration guide
├── contracts/       # Phase 1 — exported Go interface contracts
│   ├── interfaces.md
│   └── wire-format.md
└── tasks.md         # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
reactive-commons-go/
├── go.mod
├── go.sum
├── README.md
│
├── pkg/                          # Exported public API
│   ├── async/                    # Core broker-agnostic interfaces
│   │   ├── types.go              # DomainEvent, Command, AsyncQuery, Notification
│   │   ├── handlers.go           # Handler function type definitions
│   │   ├── registry.go           # HandlerRegistry interface
│   │   ├── eventbus.go           # DomainEventBus interface
│   │   ├── gateway.go            # DirectAsyncGateway interface
│   │   └── errors.go             # Library error types
│   └── headers/
│       └── headers.go            # Wire header constants (matches Java Headers.java)
│
├── internal/
│   ├── rabbit/                   # RabbitMQ implementation (not exported)
│   │   ├── connection.go         # Connection + channel pool management
│   │   ├── topology.go           # Exchange/queue declaration
│   │   ├── sender.go             # Message sender (publish with/without confirm)
│   │   ├── eventbus.go           # RabbitMQ DomainEventBus impl
│   │   ├── gateway.go            # RabbitMQ DirectAsyncGateway impl
│   │   ├── eventlistener.go      # Domain event consumer
│   │   ├── commandlistener.go    # Command consumer
│   │   ├── querylistener.go      # Query consumer + reply sender
│   │   ├── notificationlistener.go # Notification consumer
│   │   ├── replyrouter.go        # Correlation-ID → channel reply routing
│   │   └── converter.go          # JSON message serialization/deserialization
│   └── utils/
│       └── namegen.go            # Queue/exchange name generation (mirrors Java NameGenerator)
│
├── rabbit/                       # Public RabbitMQ factory (exported)
│   ├── config.go                 # RabbitConfig struct (host, port, credentials, options)
│   └── builder.go                # NewRabbitApplication() — wires everything together
│
└── tests/
    ├── unit/                     # Unit tests (no broker)
    │   └── ...
    └── integration/              # Integration tests (testcontainers-go RabbitMQ)
        ├── events_test.go
        ├── commands_test.go
        ├── queries_test.go
        └── notifications_test.go
```

**Structure Decision**: Single Go module under `reactive-commons-go/`. Public API lives in
`pkg/async/` (interfaces + types) and `rabbit/` (factory). All broker-specific implementation
is in `internal/rabbit/` to prevent accidental direct usage by consumers.
