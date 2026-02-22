package rcommons

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type rabbitQueryServer struct {
	client    RabbitClientInterface
	domain    *DomainDefinition
	registry  *Registry
	queueName string
}

func newQueryServer(client RabbitClientInterface, domain *DomainDefinition, registry *Registry) *rabbitQueryServer {
	return &rabbitQueryServer{
		client:    client,
		domain:    domain,
		registry:  registry,
		queueName: calculateQueueName(domain.Name, domain.DirectQuerySuffix, false),
	}
}

func (s *rabbitQueryServer) serveQueries() error {
	rmqChannel, err := s.client.GetChannel(ChannelForQueries)
	if err != nil {
		return err
	}

	log.Printf("Creating consumer for queue %s to process queries", s.queueName)

	msgs, err := rmqChannel.Consume(
		s.queueName,
		s.domain.globalBindID,
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
			s.processQueryMessage(msg)
		}
	}()

	return nil
}

func (s *rabbitQueryServer) processQueryMessage(msg amqp091.Delivery) {
	var query AsyncQuery[any]

	if err := json.Unmarshal(msg.Body, &query); err != nil {
		log.Printf("Failed to unmarshal query: %v", err)
		msg.Nack(false, true)
		return
	}

	h, exists := s.registry.GetQueryHandler(query.Resource)
	if !exists {
		log.Printf("No handler found for query: %s", query.Resource)
		msg.Nack(false, true)
		return
	}

	response, err := h(query)
	if err != nil {
		msg.Nack(false, true)
		return
	}

	err = s.sendReply(msg.ReplyTo, msg.CorrelationId, response)
	if err != nil {
		msg.Nack(false, true)
		return
	}

	msg.Ack(true)
}

func (s *rabbitQueryServer) sendReply(replyTo string, correlationId string, responseData any) error {
	dataBytes, err := json.Marshal(responseData)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return err
	}

	headers := map[string]string{
		"sourceApplication": s.domain.Name,
		"delivery_mode":     "persistent",
		"timestamp":         fmt.Sprintf("%d", time.Now().Unix()),
		"app_id":            s.domain.Name,
		"message_id":        correlationId,
	}

	log.Printf("Sending reply to exchange: %s, routing key: %s", s.domain.GlobalExchange, correlationId)
	err = s.client.PublishJson(s.domain.GlobalExchange, correlationId, dataBytes, ChannelForReplies, headers)
	if err != nil {
		log.Printf("Error trying to send a reply: %v", err)
		return err
	}
	log.Printf("Sent Reply: %v", 1)

	return nil
}
