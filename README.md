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
    // Configure RabbitMQ connection and domain
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
    
    // Start the service with the configured registry
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
// Option 1: Using generic EventHandler - register on the registry
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

// Must call rc.Start(registry) before handlers will start listening
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

// Send a command to a target service
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
// Register command handlers on the registry BEFORE calling rc.Start()
registry.HandleCommand("create.order", func(command any) error {
    cmd := command.(rcommons.Command[any])
    orderData := cmd.Data.(CreateOrder)
    
    log.Printf("Processing order: %s for customer: %s", 
        orderData.OrderID, orderData.CustomerID)
    
    // Process the command
    // ...
    
    return nil
})

// Must call rc.Start(registry) to start listening for commands
rc.Start(registry)
```

### 4. Working with Queries (Request-Reply)

#### Sending Queries

```go
// Define your query data
type GetUserQuery struct {
    UserID string
}

// Send a query and wait for response
query := rcommons.AsyncQuery[any]{
    Resource: "user.get",
    QueryData: GetUserQuery{
        UserID: "123",
    },
}

opts := rcommons.RequestReplyOptions{
    TargetName: "user-service",
    Domain:     "my-service",
    MaxWait:    3,
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
// Register query handlers on the registry BEFORE calling rc.Start()
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

// Must call rc.Start(registry) to start serving queries
rc.Start(registry)
```

## API Reference

### Core Types

#### DomainDefinition

Configuration for your service domain.

```go
type DomainDefinition struct {
    Name                     string          // Service name (required)
    ConnectionConfig         RabbitMQConfig  // RabbitMQ connection config (required)
    UseDomainEvents          bool            // Enable Events topology/listeners/publisher (optional)
    UseDirectCommands        bool            // Enable Commands topology/listeners/sender (optional)
    UseDirectQueries         bool            // Enable Queries topology/server/client (optional)
    DomainEventsExchange     string          // Custom exchange name (optional, defaults to "domainEvents")
    DomainEventsSuffix       string          // Custom queue suffix (optional, defaults to "subsEvents")
    DirectExchange           string          // Custom direct exchange (optional, defaults to "directMessages")
    DirectCommandsSuffix     string          // Custom command queue suffix (optional, defaults to "direct")
    DirectQuerySuffix        string          // Custom query queue suffix (optional, defaults to "query")
    GlobalExchange           string          // Custom global exchange (optional, defaults to "globalReply")
    GlobalRepliesSuffix      string          // Custom replies queue suffix (optional, defaults to "replies")
}
```

### Feature Switches: `UseDomainEvents`, `UseDirectCommands`, `UseDirectQueries`

These three flags control which messaging capabilities are initialized during `rc.Start(registry)`.

- If a flag is explicitly set to `true`, that capability is enabled.
- If a flag is `false`, Reactive Commons auto-enables it when matching handlers exist in the `Registry`:
    - `UseDomainEvents` => enabled when at least one event handler is registered.
    - `UseDirectCommands` => enabled when at least one command handler **or** query handler is registered.
    - `UseDirectQueries` => enabled when at least one query handler is registered.
- If all three remain disabled after this evaluation, `Start()` fails with configuration error.

Example (explicit enable):

```go
config := rcommons.DomainDefinition{
        Name:              "my-service",
        UseDomainEvents:   true,
        UseDirectCommands: true,
        UseDirectQueries:  false,
        ConnectionConfig:  rcommons.RabbitMQConfig{ /* ... */ },
}
```

Example (auto-enable from registered handlers):

```go
config := rcommons.DomainDefinition{
        Name:             "my-service",
        ConnectionConfig: rcommons.RabbitMQConfig{ /* ... */ },
}

registry := rcommons.NewRegistry()
registry.ListenEvent("user.created", handler)
registry.ServeQuery("user.get", queryHandler)

// Start() auto-enables:
// - UseDomainEvents (event handler exists)
// - UseDirectQueries (query handler exists)
// - UseDirectCommands (query implies direct command/direct exchange path)
rc.Start(registry)
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
    SeparateConnections bool // Use separate connections for sending and receiving
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

#### RequestReplyOptions

Options for sending queries and waiting for responses.

```go
type RequestReplyOptions struct {
    TargetName string // Target service
    Domain     string // Source domain
    MaxWait    int    // Timeout in seconds (default: 3)
}
```

### ReactiveCommons Methods

The `ReactiveCommons` struct provides these core methods for sending events, commands, and queries:

```go
// Initialize ReactiveCommons with domain configuration
NewReactiveCommons(domainCfg DomainDefinition) *ReactiveCommons

// Start listening for messages based on registered handlers
Start(registry Registry)

// Send a domain event
EmitEvent(event DomainEvent[any], opts EventOptions) error

// Send a command to a target service
SendCommand(command Command[any], opts CommandOptions) error

// Send a query and wait for response
SendQueryRequest(request AsyncQuery[any], opts RequestReplyOptions) ([]byte, error)
```

### Registry Methods

The `Registry` is used to register message handlers. Handlers must be registered BEFORE calling `rc.Start()`.

```go
// Create a new empty registry
NewRegistry() Registry

// Event handler registration
ListenEvent(eventName string, handler EventHandler)
ListenEvents(handlers map[string]EventHandler)
ListenEventTyped[T any](registry Registry, eventName string, handler func(event T) error)

// Command handler registration
HandleCommand(commandName string, handler CommandHandler)
HandleCommands(handlers map[string]CommandHandler)

// Query handler registration
ServeQuery(resource string, handler QueryHandler)
ServeQueries(handlers map[string]QueryHandler)

// Handler existence checks
HasEventHandler(eventName string) bool
HasCommandHandler(commandName string) bool
HasQueryHandler(resource string) bool

// Single handler getters
GetEventHandler(eventName string) (EventHandler, bool)
GetCommandHandler(commandName string) (CommandHandler, bool)
GetQueryHandler(resource string) (QueryHandler, bool)

// Handler map snapshots and counters
GetEventHandlers() map[string]EventHandler
GetCommandHandlers() map[string]CommandHandler
GetQueryHandlers() map[string]QueryHandler
EventHandlersCount() int
CommandHandlersCount() int
QueryHandlersCount() int
Clear()
```

### Handler Function Signatures

```go
// EventHandler - receives domain events
type EventHandler func(event any) error

// CommandHandler - receives and processes commands
type CommandHandler func(command any) error

// QueryHandler - receives queries and returns responses
type QueryHandler func(query any) (any, error)
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

    // Create registry and register all handlers BEFORE calling Start()
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
        queryData := q.QueryData.(GetUserQuery)
        
        user := User{
            ID:       queryData.UserID,
            Username: "john_doe",
            Email:    "john@example.com",
        }
        
        return user, nil
    })

    // Start the service - this initializes message listeners based on registered handlers
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

    err = rc.SendCommand(command, rcommons.CommandOptions{
        TargetName: "order-service",
        Domain:     "order-service",
    })
    if err != nil {
        log.Printf("Failed to send command: %v", err)
    }

    // Send a query
    query := rcommons.AsyncQuery[any]{
        Resource: "user.get",
        QueryData: GetUserQuery{UserID: "123"},
    }

    response, err := rc.SendQueryRequest(query, rcommons.RequestReplyOptions{
        TargetName: "user-service",
        Domain:     "order-service",
        MaxWait:    3,
    })
    if err != nil {
        log.Printf("Failed to send query: %v", err)
    } else {
        var user User
        json.Unmarshal(response, &user)
        log.Printf("Received user: %v", user)
    }

    // Keep the service running
    select {}
}
```

## Initialization Order

It's important to follow the correct initialization order when using Reactive Commons:

1. **Create ReactiveCommons instance** with domain configuration
   ```go
   rc := rcommons.NewReactiveCommons(config)
   ```

2. **Create a Registry** and register all handlers
   ```go
   registry := rcommons.NewRegistry()
   
   // Register event listeners
   registry.ListenEvent("event.name", handler)
   
   // Register command handlers
   registry.HandleCommand("command.name", handler)
   
   // Register query handlers
   registry.ServeQuery("query.resource", handler)
   ```

3. **Start the service** to activate message listeners
   ```go
   rc.Start(registry)
   ```

**Important**: Handlers must be registered before calling `rc.Start()`. The `Start()` method uses the registry to determine which message types to listen for and sets up the necessary RabbitMQ topology.

## Architecture

The library separates concerns into focused components:

- **EventPublisher**: Publishes domain events to RabbitMQ
- **EventListener**: Consumes domain events and invokes registered handlers
- **CommandSender**: Sends commands to target services
- **CommandReceiver**: Receives commands and invokes registered handlers
- **QueryClient**: Sends queries and receives responses (request-reply pattern)
- **QueryServer**: Receives queries and serves responses
- **TopologyManager**: Manages RabbitMQ exchanges, queues, and bindings
- **Registry**: Central registry for storing and managing all handlers

The `ReactiveCommons` struct serves as the main facade, coordinating these components and exposing a simple API for publishing events, sending commands, and sending queries.

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

All handler functions should return an error. The library uses the following acknowledgment strategy:

- **Handler returns nil**: Message is acknowledged (ack) - removed from queue
- **Handler returns error**: Message is nacked with requeue flag - message goes back to queue for retry
- **Message unmarshal fails**: Message is nacked without requeue - message is discarded

This ensures reliable message processing with automatic retries for handler failures, while discarding malformed messages.

### Example Error Handling

```go
registry.HandleCommand("process.order", func(command any) error {
    cmd := command.(rcommons.Command[any])
    
    // Process command
    err := processOrder(cmd.Data)
    if err != nil {
        log.Printf("Failed to process order: %v", err)
        return err  // Will cause message to be requeued
    }
    
    return nil  // Message will be acknowledged
})
```

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
