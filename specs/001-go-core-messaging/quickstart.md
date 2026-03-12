# Quickstart: reactive-commons-go (RabbitMQ)

**Feature**: 001-go-core-messaging
**Date**: 2026-03-11

---

## Prerequisites

- Go 1.22+
- RabbitMQ 3.x running locally (or via Docker)
- A Go module initialized: `go mod init your-service`

---

## Installation

```bash
go get github.com/bancolombia/reactive-commons-go
```

---

## 1. Minimal Setup

```go
package main

import (
    "context"
    "log"
    "os/signal"
    "syscall"

    "github.com/bancolombia/reactive-commons-go/rabbit"
)

func main() {
    app, err := rabbit.NewApplication(rabbit.RabbitConfig{
        AppName:  "my-service",
        Host:     "localhost",
        Password: "guest",
    })
    if err != nil {
        log.Fatal("failed to create app:", err)
    }

    // Register handlers here (see sections below)

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    if err := app.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

---

## 2. Publishing a Domain Event

```go
type OrderCreated struct {
    OrderID string  `json:"orderId"`
    Amount  float64 `json:"amount"`
}

// After creating app, before Start():
bus := app.EventBus()

ctx := context.Background()
err := bus.Emit(ctx, async.DomainEvent[OrderCreated]{
    Name:    "order.created",
    EventID: uuid.New().String(),
    Data:    OrderCreated{OrderID: "42", Amount: 99.99},
})
if err != nil {
    log.Println("emit failed:", err)
}
```

---

## 3. Subscribing to a Domain Event

```go
type OrderCreated struct {
    OrderID string  `json:"orderId"`
    Amount  float64 `json:"amount"`
}

err := app.Registry().ListenEvent("order.created",
    func(ctx context.Context, e async.DomainEvent[OrderCreated]) error {
        log.Printf("received order.created: orderId=%s amount=%.2f",
            e.Data.OrderID, e.Data.Amount)
        return nil // return non-nil to nack and redeliver
    },
)
```

---

## 4. Sending a Command

```go
type SendInvoice struct {
    InvoiceID string `json:"invoiceId"`
}

gw := app.Gateway()

err := gw.SendCommand(ctx, async.Command[SendInvoice]{
    Name:      "send-invoice",
    CommandID: uuid.New().String(),
    Data:      SendInvoice{InvoiceID: "INV-001"},
}, "invoice-service") // target service name
```

---

## 5. Handling a Command

```go
err := app.Registry().ListenCommand("send-invoice",
    func(ctx context.Context, cmd async.Command[SendInvoice]) error {
        log.Printf("sending invoice: %s", cmd.Data.InvoiceID)
        // do work...
        return nil
    },
)
```

---

## 6. Sending a Query (Request/Reply)

```go
type GetProductRequest struct {
    ProductID string `json:"productId"`
}

type Product struct {
    SKU  string  `json:"sku"`
    Name string  `json:"name"`
    Price float64 `json:"price"`
}

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

raw, err := app.Gateway().RequestReply(ctx,
    async.AsyncQuery[GetProductRequest]{
        Resource:  "get-product",
        QueryData: GetProductRequest{ProductID: "SKU-999"},
    },
    "catalog-service",
)
if err != nil {
    log.Println("query failed:", err)
    return
}

var product Product
if err := json.Unmarshal(raw, &product); err != nil {
    log.Println("deserialize failed:", err)
    return
}
log.Printf("product: %+v", product)
```

---

## 7. Handling a Query

```go
type GetProductRequest struct{ ProductID string `json:"productId"` }
type Product struct {
    SKU   string  `json:"sku"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
}

err := app.Registry().ServeQuery("get-product",
    func(ctx context.Context, q async.AsyncQuery[GetProductRequest], from async.From) (Product, error) {
        // look up product...
        return Product{SKU: q.QueryData.ProductID, Name: "Widget", Price: 9.99}, nil
    },
)
```

---

## 8. Broadcasting a Notification

```go
type CacheInvalidated struct {
    Key string `json:"key"`
}

err := bus.EmitNotification(ctx, async.Notification[CacheInvalidated]{
    Name:    "cache-invalidated",
    EventID: uuid.New().String(),
    Data:    CacheInvalidated{Key: "product:SKU-999"},
})
```

---

## 9. Receiving a Notification

```go
err := app.Registry().ListenNotification("cache-invalidated",
    func(ctx context.Context, n async.Notification[CacheInvalidated]) error {
        log.Printf("cache key invalidated: %s", n.Data.Key)
        return nil
    },
)
```

---

## 10. Configuration Reference

```go
rabbit.RabbitConfig{
    // Required
    AppName:  "my-service",

    // Connection (defaults shown)
    Host:        "localhost",
    Port:        5672,
    Username:    "guest",
    Password:    "guest",
    VirtualHost: "/",

    // Exchange names — defaults match reactive-commons-java
    DomainEventsExchange:   "domainEvents",
    DirectMessagesExchange: "directMessages",
    GlobalReplyExchange:    "globalReply",

    // Behaviour defaults
    PrefetchCount:      250,
    ReplyTimeout:       15 * time.Second,
    PersistentEvents:   true,
    PersistentCommands: true,
    PersistentQueries:  false,
    WithDLQRetry:       false,
    RetryDelay:         1 * time.Second,
}
```

---

## Interoperability with reactive-commons-java

No special configuration is required. As long as both the Java and Go services use the same
RabbitMQ broker and the default exchange names (`domainEvents`, `directMessages`, `globalReply`),
they communicate transparently:

- A Java service calling `domainEventBus.emit(event)` → Go `ListenEvent` handler fires
- A Go service calling `Emit(event)` → Java `@HandleEvent` listener fires
- Commands and queries work symmetrically in both directions

---

## Running Integration Tests

```bash
# Requires Docker/Nerdctl/Podman (testcontainers-go starts RabbitMQ automatically)
go test -tags integration ./tests/integration/...
```
