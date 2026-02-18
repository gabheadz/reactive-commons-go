package rcommons

import (
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitClientInterface defines the methods needed from RabbitClient for testing
type RabbitClientInterface interface {
	CreateChannel(name string) (*amqp.Channel, error)
	GetChannel(name string) (*amqp.Channel, error)
	PublishJson(exchange, key string, msg []byte, channelKey string, headers map[string]string) error
	PublishJsonQuery(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error
	ConsumeOne(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error)
	Close() error
}

// EventPublisher handles publishing domain events
type EventPublisher interface {
	EmitEvent(event DomainEvent[any], opts EventOptions) error
}

// EventListener handles consuming domain events
type EventListener interface {
	ListenEvent(eventName string, handler EventHandler)
	ListenEvents(handlers map[string]EventHandler)
}

// CommandSender handles sending commands
type CommandSender interface {
	SendCommand(command Command[any], opts CommandOptions) error
}

// CommandHandler handles receiving and processing commands
type CommandReceiver interface {
	HandleCommand(commandName string, handler CommandHandler)
	HandleCommands(handlers map[string]CommandHandler)
}

// QueryClient handles sending queries and receiving responses
type QueryClient interface {
	SendQueryRequest(request AsyncQuery[any], opts RequestReplyOptions) ([]byte, error)
}

// QueryServer handles receiving queries and sending responses
type QueryServer interface {
	ServeQuery(resource string, handler QueryHandler) error
	ServeQueries(handlers map[string]QueryHandler)
}

// topologyManager handles RabbitMQ topology setup
type topologyManager interface {
	setupDomainEvents() error
	setupDirectCommands() error
	setupAsyncQueries() error
}
