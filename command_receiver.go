package rcommons

import (
	"encoding/json"
	"log"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
	"github.com/rabbitmq/amqp091-go"
)

type rabbitCommandReceiver struct {
	client    RabbitClientInterface
	domain    *DomainDefinition
	registry  *Registry
	queueName string
}

// newCommandReceiver creates a new rabbitCommandReceiver instance with the provided client, domain configuration, and handler registry.
// It returns a pointer to the initialized rabbitCommandReceiver that can be used to handle incoming commands.
func newCommandReceiver(client RabbitClientInterface, domain *DomainDefinition, registry *Registry) *rabbitCommandReceiver {
	return &rabbitCommandReceiver{
		client:    client,
		domain:    domain,
		registry:  registry,
		queueName: calculateQueueName(domain.Name, domain.DirectCommandsSuffix, false),
	}
}

func (r *rabbitCommandReceiver) startListeningCommands() {

	rmqChannel, _ := r.client.GetChannel(ChannelForCommands)

	wrappedChannel := &rabbit.RabbitChannel{Channel: rmqChannel}
	err := rabbit.Bind(wrappedChannel, r.queueName, r.domain.Name, r.domain.DirectExchange, true)
	if err != nil {
		log.Panicf("Failed to bind queue: %v", err)
	}

	msgs, err := rmqChannel.Consume(
		r.queueName,
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

func (r *rabbitCommandReceiver) processCommandMessage(msg amqp091.Delivery) {
	var command Command[any]
	err := json.Unmarshal(msg.Body, &command)
	if err != nil {
		log.Printf("Failed to unmarshal command: %v", err)
		msg.Nack(false, false)
		return
	}

	h, exists := r.registry.GetCommandHandler(command.Name)
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
