package rcommons

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type rabbitEventPublisher struct {
	client RabbitClientInterface
	domain DomainDefinition
}

func newEventPublisher(client RabbitClientInterface, domain DomainDefinition) *rabbitEventPublisher {
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

	headers := map[string]string{
		"sourceApplication": p.domain.Name,
		"delivery_mode":     "persistent",
		"message_id":        event.EventId,
		"timestamp":         fmt.Sprintf("%d", time.Now().Unix()),
		"app_id":            p.domain.Name,
	}

	err = p.client.PublishJson(p.domain.DomainEventsExchange, event.Name, evtBytes, ChannelForEvents, headers)
	if err != nil {
		log.Printf("Failed send event: %v", err)
		return err
	}

	return nil
}
