package rcommons

import (
	"encoding/json"
	"log"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
	"github.com/rabbitmq/amqp091-go"
)

type rabbitEventListener struct {
	client    RabbitClientInterface
	domain    DomainDefinition
	registry  *Registry
	queueName string
}

// newEventListener creates a new rabbitEventListener instance with the provided RabbitMQ client,
// domain definition, and event handler registry. The returned listener can be used to
// subscribe to and process domain events from RabbitMQ queues.
func newEventListener(client RabbitClientInterface, domain DomainDefinition, registry *Registry) *rabbitEventListener {
	return &rabbitEventListener{
		client:    client,
		domain:    domain,
		registry:  registry,
		queueName: calculateQueueName(domain.Name, domain.DomainEventsSuffix, false),
	}
}

func (l *rabbitEventListener) setupBindings() {
	rmqChannel, err := l.client.GetChannel(ChannelForEvents)
	if err != nil {
		log.Panicf("Failed to get channel: %v", err)
	}

	for eventName := range l.registry.EventHandlers {
		// Wrap channel for topology operations
		wrappedChannel := &rabbit.RabbitChannel{Channel: rmqChannel}
		err = rabbit.Bind(wrappedChannel, l.queueName, eventName, l.domain.DomainEventsExchange, true)
		if err != nil {
			log.Panicf("Failed to bind queue: %v", err)
		}
	}
}

func (l *rabbitEventListener) startListeningEvents() {

	rmqChannel, err := l.client.GetChannel(ChannelForEvents)
	if err != nil {
		log.Panicf("Failed to get channel: %v", err)
	}

	msgs, err := rmqChannel.Consume(
		l.queueName,
		"",
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
			l.processEventMessage(msg)
		}
	}()
}

func (l *rabbitEventListener) processEventMessage(msg amqp091.Delivery) {
	var event DomainEvent[any]
	err := json.Unmarshal(msg.Body, &event)

	if err != nil {
		log.Printf("Failed to unmarshal event: %v", err)
		msg.Nack(false, false)
		return
	}

	evtHandler, exists := l.registry.EventHandlers[event.Name]
	if exists {
		errEvt := evtHandler(event)
		if errEvt != nil {
			log.Printf("Failed to handle event: %v", errEvt)
			msg.Nack(false, true)
			return
		}
		msg.Ack(false)
	} else {
		log.Printf("No handler found for event: %s", event.Name)
		msg.Nack(false, false)
	}
}
