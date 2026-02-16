package rcommons

import (
	"fmt"
)

type RabbitMQConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	Secure      bool
	VirtualHost string
}

type DomainDefinition struct {
	Name                     string
	UseDomainEvents          bool
	UseDomainNotifications   bool
	UseDirectCommands        bool
	UseDirectQueries         bool
	DomainEventsExchange     string
	DomainEventsSuffix       string
	DomainNotificationSuffix string
	DirectExchange           string
	DirectQuerySuffix        string
	DirectCommandsSuffix     string
	GlobalExchange           string
	GlobalRepliesSuffix      string
	globalRepliesID          string
	globalBindID             string
	ConnectionConfig         RabbitMQConfig
}

type DomainEvent[T any] struct {
	Name    string
	EventId string
	Data    T
}

type EventOptions struct {
	TargetName  string
	DelayMillis int
	Domain      string
}

type DomainEventEmitter[T any] interface {
	EmitEvent(event DomainEvent[T], opts EventOptions) error
	//EmitCloudEvent(event cloudevents.Event, opts EventOptions)
	//EmitRaw(event RawMessage, opts EventOptions)
}

type EventHandler func(event any) error

// EventHandlerFunc is a generic wrapper that provides type safety
type EventHandlerFunc[T any] func(event T) error

// ToEventHandler converts a typed handler to the generic EventHandler
func (h EventHandlerFunc[T]) ToEventHandler() EventHandler {
	return func(event any) error {
		typedEvent, ok := event.(T)
		if !ok {
			return fmt.Errorf("event type mismatch: expected %T, got %T", *new(T), event)
		}
		return h(typedEvent)
	}
}

type Command[T any] struct {
	Name      string
	CommandId string
	Data      T
}

type CommandOptions struct {
	TargetName  string
	DelayMillis int
	Domain      string
}

type CommandHandler func(command any) error

type RequestReplyOptions struct {
	TargetName string
	Domain     string
}

type AsyncQuery[T any] struct {
	Resource  string
	QueryData T
}

type QueryHandler func(query any) (any, error)

func connectionString(c RabbitMQConfig) string {
	secure := ""
	if c.Secure {
		secure = "s"
	}
	connAddr := fmt.Sprintf(
		"amqp%s://%s:%s@%s:%d/%s",
		secure,
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.VirtualHost,
	)
	return connAddr
}
