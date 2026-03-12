<!--
SYNC IMPACT REPORT
==================
Version change: N/A → 1.0.0 (initial constitution)
Modified principles: None (new document)
Added sections: Core Principles (6), Technology Stack & Constraints, Migration Strategy, Governance
Removed sections: None
Templates requiring updates:
  ✅ /Users/gabrimar/desarrollo/reactive-commons/.specify/templates/plan-template.md — Constitution Check gate references this constitution
  ✅ /Users/gabrimar/desarrollo/reactive-commons/.specify/templates/spec-template.md — no structural changes needed; constitution alignment verified
  ✅ /Users/gabrimar/desarrollo/reactive-commons/.specify/templates/tasks-template.md — task categories aligned with principles (test-first, broker abstraction, wire compat)
Follow-up TODOs: None — all placeholders resolved
-->

# reactive-commons-go Constitution

## Core Principles

### I. Go-Idiomatic Design (NON-NEGOTIABLE)
All abstractions MUST use Go-native concurrency primitives: goroutines, channels, and `context.Context` for cancellation/timeout propagation.
Reactor-style (Mono/Flux) patterns MUST NOT be ported literally; use Go's `chan`, `context`, and structured concurrency instead.
APIs MUST follow Go conventions: error returns, interface-based design, no annotation magic.

### II. Behavioral Parity
The library MUST implement all four messaging patterns from the Java version: Domain Events (pub/sub), Commands (point-to-point), Queries (async request/reply), and Notifications.
Handler registration APIs MUST mirror Java's semantic model (register handler by event/command name, typed payload) even if syntax differs.
Wire protocol (RabbitMQ exchanges/queues topology, Kafka topic conventions, JSON envelope format) MUST be compatible with the Java implementation to allow mixed Java/Go deployments.

### III. Test-First (NON-NEGOTIABLE)
TDD is mandatory: tests MUST be written before implementation code; tests MUST fail before any implementation is added.
Red-Green-Refactor cycle is strictly enforced.
Every exported symbol MUST have at least one unit test.
Integration tests using real broker containers (via testcontainers-go or docker-compose) are REQUIRED for each broker implementation.

### IV. Broker Abstraction & Multi-Broker Support
Core domain logic (event/command/query patterns) MUST be broker-agnostic via a defined Go interface.
RabbitMQ and Kafka MUST be the first two supported broker implementations, matching Java parity.
New brokers MUST be addable without modifying core abstractions (open/closed principle via interfaces).

### V. Wire Protocol Compatibility
JSON message envelopes (headers, payload structure) MUST be identical to the Java implementation to allow seamless interoperability between Java and Go microservices on the same broker.
Any deviation from Java wire format MUST be explicitly documented and version-gated.
Serialization/deserialization MUST be tested with fixture messages produced by the Java version.

### VI. Simplicity & Explicit Over Magic
No dependency injection frameworks; use constructor functions and explicit wiring.
No global state; all configuration MUST be passed explicitly.
YAGNI: do not port Java-specific Spring Boot integration code; provide clean Go library interfaces only.
Prefer flat, readable code over clever abstractions. Complexity MUST be justified in comments and architecture docs.

## Technology Stack & Constraints

- Language: Go 1.22+
- Module system: Go modules (`go.mod`)
- Broker clients: `rabbitmq/amqp091-go` (RabbitMQ), `segmentio/kafka-go` or `confluentinc/confluent-kafka-go` (Kafka)
- Serialization: `encoding/json` (standard library); no annotation-based mappers
- Testing: `testing` package + `testify/assert`; integration tests via `testcontainers-go`
- No Spring, no Lombok, no dependency injection containers
- All Go modules MUST pass `go vet`, `staticcheck`, and `golangci-lint` before merge
- Minimum Go version pinned in `go.mod` and CI; no vendor directory (use module proxy)

## Migration Strategy

- Phase 1 — Core Abstractions: Define Go interfaces for `EventBus`, `CommandGateway`, `QueryGateway`, `HandlerRegistry`; define message envelope structs compatible with Java wire format.
- Phase 2 — RabbitMQ Implementation: Implement all four messaging patterns over RabbitMQ with full test coverage; validate wire compatibility with Java fixtures.
- Phase 3 — Kafka Implementation: Port Kafka broker support following the same interface contracts; validate parity with Java Kafka module.
- Phase 4 — Developer Experience: Fluent builder APIs, configuration validation, structured logging, and example applications demonstrating Java↔Go interoperability.
- Java source code in `reactive-commons-java/` is the authoritative reference for behavior; any ambiguity MUST be resolved by reading Java source or tests.

## Governance

This constitution supersedes all other development practices and guides in this repository.
Amendments require: (1) a written proposal describing the change and rationale, (2) review by at least one maintainer, (3) a migration plan if existing implementations are affected, and (4) a version bump following semantic versioning rules.
All PRs MUST include a "Constitution Check" section confirming compliance with relevant principles.
Complexity violations (deviations from Principle VI) MUST be documented in a `complexity.md` file at the repository root with justification.
Versioning policy: MAJOR for backward-incompatible principle redefinitions or removals; MINOR for new principles or material guidance additions; PATCH for clarifications and wording fixes.
Compliance review: conduct at each major feature milestone; document findings in `.specify/memory/`.

**Version**: 1.0.0 | **Ratified**: 2026-03-11 | **Last Amended**: 2026-03-11
