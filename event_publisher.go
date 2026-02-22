package rcommons

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
)

type fastEventPublisherClient interface {
	PublishEvent(exchange, key string, msg []byte, channelKey string, sourceApplication, messageID string, timestamp int64) error
}

type rabbitEventPublisher struct {
	client RabbitClientInterface
	domain *DomainDefinition
}

func newEventPublisher(client RabbitClientInterface, domain *DomainDefinition) *rabbitEventPublisher {
	_, err := client.CreateChannel(ChannelForEvents)
	if err != nil {
		log.Panicf("Failed to create channel: %v", err)
	}
	return &rabbitEventPublisher{
		client: client,
		domain: domain,
	}
}

func (p *rabbitEventPublisher) emitEvent(event DomainEvent[any], opts EventOptions) error {
	if event.Name == "" || opts.Domain == "" {
		return fmt.Errorf("event name and domain are required")
	}

	evtBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return err
	}

	timestamp := time.Now().Unix()
	if fastClient, ok := p.client.(fastEventPublisherClient); ok {
		err = fastClient.PublishEvent(p.domain.DomainEventsExchange, event.Name, evtBytes, ChannelForEvents, p.domain.Name, event.EventId, timestamp)
	} else {
		headers := map[string]string{
			"sourceApplication": p.domain.Name,
			"delivery_mode":     "persistent",
			"message_id":        event.EventId,
			"timestamp":         strconv.FormatInt(timestamp, 10),
			"app_id":            p.domain.Name,
		}
		err = p.client.PublishJson(p.domain.DomainEventsExchange, event.Name, evtBytes, ChannelForEvents, headers)
	}
	if err != nil {
		log.Printf("Failed send event: %v", err)
		return err
	}

	return nil
}
