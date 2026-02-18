package rcommons

import (
	"log"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
)

type rabbitTopologyManager struct {
	client *rabbit.RabbitClient
	domain *DomainDefinition
}

func newTopologyManager(client *rabbit.RabbitClient, domain *DomainDefinition) *rabbitTopologyManager {
	return &rabbitTopologyManager{
		client: client,
		domain: domain,
	}
}

func (m *rabbitTopologyManager) setupDomainEvents() error {

	ch, err := m.client.CreateChannel(ChannelForEvents)
	if err != nil {
		log.Panicf("Failed to create channel: %v", err)
	}

	// Wrap channel for topology operations
	wrappedChannel := &rabbit.RabbitChannel{Channel: ch}

	err = rabbit.DeclareExchange(wrappedChannel, m.domain.DomainEventsExchange, "topic", true, false, false, false)
	if err != nil {
		log.Printf("Failed to declare domain events exchange: %v", err)
		return err
	}

	queueName := calculateQueueName(m.domain.Name, m.domain.DomainEventsSuffix, false)
	err = rabbit.DeclareQueue(wrappedChannel, "", queueName, true, false, false, false)
	if err != nil {
		log.Printf("Failed to declare domain events queue: %v", err)
		return err
	}

	return nil
}

func (m *rabbitTopologyManager) setupDirectCommands() error {

	ch, err := m.client.CreateChannel(ChannelForCommands)
	if err != nil {
		log.Panicf("Failed to create channel: %v", err)
	}

	// Wrap channel for topology operations
	wrappedChannel := &rabbit.RabbitChannel{Channel: ch}

	err = rabbit.DeclareExchange(wrappedChannel, m.domain.DirectExchange, "direct", true, false, false, false)
	if err != nil {
		log.Printf("Failed to declare direct exchange: %v", err)
		return err
	}

	queueName := calculateQueueName(m.domain.Name, m.domain.DirectCommandsSuffix, false)
	err = rabbit.DeclareQueue(wrappedChannel, "", queueName, true, false, false, false)
	if err != nil {
		log.Printf("Failed to declare direct commands queue: %v", err)
		return err
	}

	if m.domain.UseDirectQueries {
		ch2, err2 := m.client.CreateChannel(ChannelForQueries)
		if err2 != nil {
			log.Panicf("Failed to create channel: %v", err)
		}

		wrappedChannel2 := &rabbit.RabbitChannel{Channel: ch2}
		queriesQueueName := calculateQueueName(m.domain.Name, m.domain.DirectQuerySuffix, false)

		err = rabbit.DeclareQueue(wrappedChannel2, "", queriesQueueName, true, false, false, false)
		if err != nil {
			log.Printf("Failed to declare direct queries queue: %v", err)
			return err
		}

		err = rabbit.Bind(wrappedChannel2, queriesQueueName, queriesQueueName, m.domain.DirectExchange, true)
		if err != nil {
			log.Printf("Failed to bind direct queries queue: %v", err)
			return err
		}
		log.Printf("Bind queue %s to exchange %s with routing key %s for Queries processing", queriesQueueName, m.domain.DirectExchange, queriesQueueName)
	}

	return nil
}

func (m *rabbitTopologyManager) setupAsyncQueries() error {

	ch, err := m.client.CreateChannel(ChannelForReplies)
	if err != nil {
		log.Panicf("Failed to create channel: %v", err)
	}

	// Wrap channel for topology operations
	wrappedChannel := &rabbit.RabbitChannel{Channel: ch}

	err = rabbit.DeclareExchange(wrappedChannel, m.domain.GlobalExchange, "topic", true, false, false, false)
	if err != nil {
		log.Printf("Failed to declare global exchange: %v", err)
		return err
	}

	err = rabbit.DeclareQueue(wrappedChannel, "", m.domain.globalRepliesID, false, true, true, false)
	if err != nil {
		log.Printf("Failed to declare replies queue: %v", err)
		return err
	}

	err = rabbit.Bind(wrappedChannel, m.domain.globalRepliesID, m.domain.globalBindID, m.domain.GlobalExchange, true)
	if err != nil {
		log.Printf("Failed to bind replies queue: %v", err)
		return err
	}
	log.Printf("Bind queue %s to exchange %s with routing key %s for Replies processing",
		m.domain.globalRepliesID, m.domain.GlobalExchange, m.domain.globalBindID)

	return nil
}
