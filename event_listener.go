package rcommons

import (
	"encoding/json"
	"log"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
	"github.com/rabbitmq/amqp091-go"
)

type RabbitEventListener struct {
	client   *rabbit.RabbitClient
	domain   DomainDefinition
	registry *Registry
}

// NewEventListener creates a new RabbitEventListener instance with the provided RabbitMQ client,
// domain definition, and event handler registry. The returned listener can be used to
// subscribe to and process domain events from RabbitMQ queues.
func NewEventListener(client *rabbit.RabbitClient, domain DomainDefinition, registry *Registry) *RabbitEventListener {
	return &RabbitEventListener{
		client:   client,
		domain:   domain,
		registry: registry,
	}
}

func (l *RabbitEventListener) ListenEvents(handlers map[string]EventHandler) {
	for eventName, handler := range handlers {
		l.ListenEvent(eventName, handler)
	}
}

func (l *RabbitEventListener) ListenEvent(eventName string, handler EventHandler) {
	rmqChannel, err := l.client.GetChannel(ChannelForEvents)
	if err != nil {
		log.Panicf("Failed to get channel: %v", err)
	}

	queueName := calculateQueueName(l.domain.Name, l.domain.DomainEventsSuffix, false)

	// Wrap channel for topology operations
	wrappedChannel := &rabbit.RabbitChannel{Channel: rmqChannel}
	err = rabbit.Bind(wrappedChannel, queueName, eventName, l.domain.DomainEventsExchange, true)
	if err != nil {
		log.Panicf("Failed to bind queue: %v", err)
	}

	msgs, err := rmqChannel.Consume(
		queueName,
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
			l.processEventMessage(eventName, msg)
		}
	}()
}

func (l *RabbitEventListener) processEventMessage(eventName string, msg amqp091.Delivery) {
	var event DomainEvent[any]
	err := json.Unmarshal(msg.Body, &event)
	if err != nil {
		log.Printf("Failed to unmarshal event: %v", err)
		msg.Nack(false, false)
		return
	}

	evtHandler, exists := l.registry.EventHandlers[eventName]
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
