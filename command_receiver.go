package rcommons

import (
	"encoding/json"
	"log"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
	"github.com/rabbitmq/amqp091-go"
)

type RabbitCommandReceiver struct {
	client   *rabbit.RabbitClient
	domain   DomainDefinition
	registry *Registry
}

// NewCommandReceiver creates a new RabbitCommandReceiver instance with the provided client, domain configuration, and handler registry.
// It returns a pointer to the initialized RabbitCommandReceiver that can be used to handle incoming commands.
func NewCommandReceiver(client *rabbit.RabbitClient, domain DomainDefinition, registry *Registry) *RabbitCommandReceiver {
	return &RabbitCommandReceiver{
		client:   client,
		domain:   domain,
		registry: registry,
	}
}

func (r *RabbitCommandReceiver) HandleCommands(handlers map[string]CommandHandler) {
	for commandName, handler := range handlers {
		r.HandleCommand(commandName, handler)
	}
}

func (r *RabbitCommandReceiver) HandleCommand(commandName string, handler CommandHandler) {
	rmqChannel, _ := r.client.GetChannel(ChannelForCommands)

	queueName := calculateQueueName(r.domain.Name, r.domain.DirectCommandsSuffix, false)

	// Wrap channel for topology operations
	wrappedChannel := &rabbit.RabbitChannel{Channel: rmqChannel}
	err := rabbit.Bind(wrappedChannel, queueName, r.domain.Name, r.domain.DirectExchange, true)
	if err != nil {
		log.Panicf("Failed to bind queue: %v", err)
	}

	msgs, err := rmqChannel.Consume(
		queueName,
		r.domain.Name,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Panicf("Failed to consume: %v", err)
	}

	go func() {
		for msg := range msgs {
			r.processCommandMessage(msg)
		}
	}()
}

func (r *RabbitCommandReceiver) processCommandMessage(msg amqp091.Delivery) {
	var command Command[any]
	err := json.Unmarshal(msg.Body, &command)
	if err != nil {
		log.Printf("Failed to unmarshal command: %v", err)
		msg.Nack(false, false)
		return
	}

	h, exists := r.registry.CommandHandlers[command.Name]
	if exists {
		err = h(command)
		if err != nil {
			msg.Nack(false, true)
			return
		}
		msg.Ack(false)
	} else {
		log.Printf("No handler found for command: %s", command.Name)
		msg.Nack(false, false)
	}
}
