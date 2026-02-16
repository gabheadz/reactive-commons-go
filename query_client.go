package rcommons

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bancolombia/reactive-commons-go/internal/rabbit"
)

type RabbitQueryClient struct {
	client *rabbit.RabbitClient
	domain DomainDefinition
}

func NewQueryClient(client *rabbit.RabbitClient, domain DomainDefinition) *RabbitQueryClient {
	return &RabbitQueryClient{
		client: client,
		domain: domain,
	}
}

func (c *RabbitQueryClient) SendQueryRequest(request AsyncQuery[any], opts RequestReplyOptions) ([]byte, error) {
	if request.Resource == "" {
		return nil, fmt.Errorf("request resource and domain are required")
	}

	if opts.TargetName == "" {
		return nil, fmt.Errorf("request target is required")
	}

	cmdBytes, err := json.Marshal(request)
	if err != nil {
		log.Printf("Failed to marshal request: %v", err)
		return nil, err
	}

	headers := map[string]string{
		"sourceApplication": c.domain.Name,
		"delivery_mode":     "persistent",
		"timestamp":         fmt.Sprintf("%d", time.Now().Unix()),
		"app_id":            c.domain.Name,
		"message_id":        c.domain.globalBindID,
	}

	queueName := calculateQueueName(c.domain.Name, c.domain.DirectQuerySuffix, false)
	err = c.client.PublishJson(c.domain.DirectExchange, queueName, cmdBytes, ChannelForQueries, headers)
	if err != nil {
		log.Printf("Failed send request: %v", err)
		return nil, err
	}
	log.Printf("Sent Query: %v", 1)

	body, err := c.client.ConsumeOne(ChannelForReplies, c.domain.globalRepliesID, c.domain.globalBindID,
		time.Duration(3)*time.Second)
	if err != nil {
		log.Printf("Failed to consume reply: %v", err)
		return nil, err
	}

	return body, nil
}
