# Reactive Commons Go

A Go library for building reactive, event-driven microservices using RabbitMQ as the message broker. This library provides a clean abstraction for implementing Domain Events, Commands, and Queries patterns in distributed systems.

## Features

- **Domain Events**: Publish and subscribe to domain events across services
- **Direct Commands**: Send commands to specific service instances
- **Async Queries**: Request-reply pattern for querying data from other services
- **Type Safety**: Generic support for strongly-typed event and command handlers
- **Automatic Topology Management**: RabbitMQ exchanges and queues are created automatically
- **Clean Architecture**: Separation of concerns with focused components

## Installation

```bash
go get github.com/bancolombia/reactive-commons-go
```

## Quick Start

### 1. Configure Your Domain

```go
package main

import (
    "github.com/bancolombia/reactive-commons-go"
)

func main() {
    // Configure RabbitMQ connection
    config := rcommons.DomainDefinition{
        Name: "my-service",
        ConnectionConfig: rcommons.RabbitMQConfig{
            Host:        "localhost",
            Port:        5672,
            User:        "guest",
            Password:    "guest",
            VirtualHost: "/",
            Secure:      false,
        },
    }

    // Create ReactiveCommons instance
    rc := rcommons.NewReactiveCommons(config)

    // Create and configure registry
    registry := rcommons.NewRegistry()
    
    // Register handlers (see examples below)
    
    // Start the service
    rc.Start(registry)
}
```

### 2. Working with Domain Events

#### Publishing Events

```go
// Define your event data structure
type UserCreated struct {
    UserID   string
    Username string
    Email    string
}

// Emit an event
event := rcommons.DomainEvent[any]{
    Name:    "user.created",
    EventId: uuid.New().String(),
    Data: UserCreated{
        UserID:   "123",
        Username: "john_doe",
        Email:    "john@example.com",
    },
}

opts := rcommons.EventOptions{
    Domain: "my-service",
}

err := rc.EmitEvent(event, opts)
if err != nil {
    log.Printf("Failed to emit event: %v", err)
}
```

#### Subscribing to Events

```go
// Option 1: Using generic EventHandler
registry.ListenEvent("user.created", func(event any) error {
    domainEvent := event.(rcommons.DomainEvent[any])
    log.Printf("Received event: %s with data: %v", domainEvent.Name, domainEvent.Data)
    return nil
})

// Option 2: Using type-safe handler
rcommons.ListenEventTyped(registry, "user.created", func(event rcommons.DomainEvent[UserCreated]) error {
    log.Printf("User created: %s (%s)", event.Data.Username, event.Data.Email)
    return nil
})
```

### 3. Working with Commands

#### Sending Commands

```go
// Define your command data
type CreateOrder struct {
    OrderID    string
    CustomerID string
    Amount     float64
}

// Send a command
command := rcommons.Command[any]{
    Name:      "create.order",
    CommandId: uuid.New().String(),
    Data: CreateOrder{
        OrderID:    "ORD-001",
        CustomerID: "CUST-123",
        Amount:     99.99,
    },
}

opts := rcommons.CommandOptions{
    TargetName: "order-service",
    Domain:     "my-service",
}

err := rc.SendCommand(command, opts)
if err != nil {
    log.Printf("Failed to send command: %v", err)
}
```

#### Handling Commands

```go
registry.HandleCommand("create.order", func(command any) error {
    cmd := command.(rcommons.Command[any])
    orderData := cmd.Data.(CreateOrder)
    
    log.Printf("Processing order: %s for customer: %s", 
        orderData.OrderID, orderData.CustomerID)
    
    // Process the command
    // ...
    
    return nil
})
```

### 4. Working with Queries (Request-Reply)

#### Sending Queries

```go
// Define your query data
type GetUserQuery struct {
    UserID string
}

// Send a query
query := rcommons.AsyncQuery[any]{
    Resource: "user.get",
    QueryData: GetUserQuery{
        UserID: "123",
    },
}

opts := rcommons.RequestReplyOptions{
    TargetName: "user-service",
    Domain:     "my-service",
}

response, err := rc.SendQueryRequest(query, opts)
if err != nil {
    log.Printf("Failed to send query: %v", err)
    return
}

// Parse response
var user User
json.Unmarshal(response, &user)
log.Printf("Received user: %v", user)
```

#### Serving Queries

```go
registry.ServeQuery("user.get", func(query any) (any, error) {
    q := query.(rcommons.AsyncQuery[any])
    queryData := q.QueryData.(GetUserQuery)
    
    // Fetch user from database
    user := User{
        ID:       queryData.UserID,
        Username: "john_doe",
        Email:    "john@example.com",
    }
    
    return user, nil
})
```

## API Reference

### Core Types

#### DomainDefinition

Configuration for your service domain.

```go
type DomainDefinition struct {
    Name                     string          // Service name (required)
    ConnectionConfig         RabbitMQConfig  // RabbitMQ connection config (required)
    DomainEventsExchange     string          // Custom exchange name (optional)
    DomainEventsSuffix       string          // Custom queue suffix (optional)
    DirectExchange           string          // Custom direct exchange (optional)
    DirectCommandsSuffix     string          // Custom command queue suffix (optional)
    DirectQuerySuffix        string          // Custom query queue suffix (optional)
    GlobalExchange           string          // Custom global exchange (optional)
    GlobalRepliesSuffix      string          // Custom replies queue suffix (optional)
}
```

#### RabbitMQConfig

RabbitMQ connection configuration.

```go
type RabbitMQConfig struct {
    Host        string  // RabbitMQ host
    Port        int     // RabbitMQ port
    User        string  // Username
    Password    string  // Password
    Secure      bool    // Use TLS (amqps)
    VirtualHost string  // Virtual host
}
```

#### DomainEvent[T]

Generic domain event structure.

```go
type DomainEvent[T any] struct {
    Name    string  // Event name/type
    EventId string  // Unique event identifier
    Data    T       // Event payload
}
```

#### Command[T]

Generic command structure.

```go
type Command[T any] struct {
    Name      string  // Command name/type
    CommandId string  // Unique command identifier
    Data      T       // Command payload
}
```

#### AsyncQuery[T]

Generic query structure.

```go
type AsyncQuery[T any] struct {
    Resource  string  // Resource/query identifier
    QueryData T       // Query parameters
}
```

### ReactiveCommons Methods

#### Event Methods

```go
// Emit a domain event
EmitEvent(event DomainEvent[any], opts EventOptions) error

// Listen to a single event
ListenEvent(eventName string, handler EventHandler)

// Listen to multiple events
ListenEvents(handlers map[string]EventHandler)
```

#### Command Methods

```go
// Send a command to a target service
SendCommand(command Command[any], opts CommandOptions) error

// Handle a single command
HandleCommand(commandName string, handler CommandHandler)

// Handle multiple commands
HandleCommands(handlers map[string]CommandHandler)
```

#### Query Methods

```go
// Send a query and wait for response
SendQueryRequest(request AsyncQuery[any], opts RequestReplyOptions) ([]byte, error)
```

### Registry Methods

```go
// Create a new registry
NewRegistry() *Registry

// Register event handlers
ListenEvent(eventName string, handler EventHandler)
ListenEvents(handlers map[string]EventHandler)
ListenEventTyped[T any](registry *Registry, eventName string, handler func(event T) error)

// Register command handlers
HandleCommand(commandName string, handler CommandHandler)
HandleCommands(handlers map[string]CommandHandler)

// Register query handlers
ServeQuery(resource string, handler QueryHandler)
ServeQueries(handlers map[string]QueryHandler)
```

## Complete Example

```go
package main

import (
    "encoding/json"
    "log"
    
    "github.com/bancolombia/reactive-commons-go"
    "github.com/google/uuid"
)

type UserCreated struct {
    UserID   string
    Username string
    Email    string
}

type CreateOrder struct {
    OrderID    string
    CustomerID string
    Amount     float64
}

type GetUserQuery struct {
    UserID string
}

type User struct {
    ID       string
    Username string
    Email    string
}

func main() {
    // Configure domain
    config := rcommons.DomainDefinition{
        Name: "order-service",
        ConnectionConfig: rcommons.RabbitMQConfig{
            Host:        "localhost",
            Port:        5672,
            User:        "guest",
            Password:    "guest",
            VirtualHost: "/",
            Secure:      false,
        },
    }

    // Create ReactiveCommons instance
    rc := rcommons.NewReactiveCommons(config)

    // Create registry
    registry := rcommons.NewRegistry()

    // Register event handlers
    rcommons.ListenEventTyped(registry, "user.created", func(event rcommons.DomainEvent[UserCreated]) error {
        log.Printf("User created: %s (%s)", event.Data.Username, event.Data.Email)
        return nil
    })

    // Register command handlers
    registry.HandleCommand("create.order", func(command any) error {
        cmd := command.(rcommons.Command[any])
        log.Printf("Processing command: %s", cmd.Name)
        return nil
    })

    // Register query handlers
    registry.ServeQuery("user.get", func(query any) (any, error) {
        q := query.(rcommons.AsyncQuery[any])
        queryData := q.QueryData.(map[string]interface{})
        
        user := User{
            ID:       queryData["UserID"].(string),
            Username: "john_doe",
            Email:    "john@example.com",
        }
        
        return user, nil
    })

    // Start the service
    rc.Start(registry)

    // Emit an event
    event := rcommons.DomainEvent[any]{
        Name:    "order.created",
        EventId: uuid.New().String(),
        Data: map[string]interface{}{
            "OrderID": "ORD-001",
            "Amount":  99.99,
        },
    }

    err := rc.EmitEvent(event, rcommons.EventOptions{Domain: "order-service"})
    if err != nil {
        log.Printf("Failed to emit event: %v", err)
    }

    // Keep the service running
    select {}
}
```

## Architecture

The library is organized into focused components:

- **EventPublisher/EventListener**: Handle domain event publishing and consumption
- **CommandSender/CommandReceiver**: Handle command sending and processing
- **QueryClient/QueryServer**: Handle request-reply query patterns
- **TopologyManager**: Manages RabbitMQ topology (exchanges, queues, bindings)
- **Registry**: Central registry for all handlers

## Default Naming Conventions

If not specified, the library uses these defaults:

- **Domain Events Exchange**: `domainEvents` (topic)
- **Domain Events Queue**: `{serviceName}.subsEvents`
- **Direct Exchange**: `directMessages` (direct)
- **Direct Commands Queue**: `{serviceName}.direct`
- **Direct Queries Queue**: `{serviceName}.query`
- **Global Replies Exchange**: `globalReply` (topic)
- **Global Replies Queue**: `{serviceName}.replies.{uuid}`

## Error Handling

All handler functions should return errors. The library will:
- **Nack and requeue** messages when handlers return errors (for retry)
- **Nack without requeue** messages that fail to unmarshal
- **Ack** messages that are processed successfully

## Testing

The library includes comprehensive unit tests. Run them with:

```bash
go test ./...
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Related Projects

- [Reactive Commons Java](https://github.com/bancolombia/reactive-commons-java)
- [Reactive Commons NodeJS](https://github.com/bancolombia/reactive-commons-nodejs)
