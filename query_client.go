package rcommons

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type rabbitQueryClient struct {
	clientSending   RabbitClientInterface
	clientReceiving RabbitClientInterface
	domain          *DomainDefinition
	queueName       string
}

func newQueryClient(clientSending RabbitClientInterface, clientReceiving RabbitClientInterface, domain *DomainDefinition) *rabbitQueryClient {
	_, err := clientSending.CreateChannel(ChannelForQueries)
	if err != nil {
		log.Panicf("Failed to create channel: %v", err)
	}
	return &rabbitQueryClient{
		clientSending:   clientSending,
		clientReceiving: clientReceiving,
		domain:          domain,
		queueName:       calculateQueueName(domain.Name, domain.DirectQuerySuffix, false),
	}
}

func (c *rabbitQueryClient) sendQueryRequest(request AsyncQuery[any], opts RequestReplyOptions) ([]byte, error) {
	if c.clientSending == nil || c.clientReceiving == nil {
		return nil, fmt.Errorf("clients not initialized")
	}

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

	err = c.clientSending.PublishJson(c.domain.DirectExchange, c.queueName, cmdBytes, ChannelForQueries, headers)
	if err != nil {
		log.Printf("Failed send request: %v", err)
		return nil, err
	}

	body, err := c.clientReceiving.ConsumeOne(ChannelForReplies, c.domain.globalRepliesID,
		c.domain.globalBindID,
		maxWaitDuration(opts))
	if err != nil {
		log.Printf("Failed to consume reply: %v", err)
		return nil, err
	}

	return body, nil
}

func maxWaitDuration(opts RequestReplyOptions) time.Duration {
	if opts.MaxWait <= 0 {
		return time.Duration(3) * time.Second
	}
	return time.Duration(opts.MaxWait) * time.Second
}
