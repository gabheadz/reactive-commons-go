package rcommons

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type rabbitCommandSender struct {
	client RabbitClientInterface
	domain *DomainDefinition
}

func newCommandSender(client RabbitClientInterface, domain *DomainDefinition) *rabbitCommandSender {
	_, err := client.CreateChannel(ChannelForCommands)
	if err != nil {
		log.Panicf("Failed to create channel: %v", err)
	}
	return &rabbitCommandSender{
		client: client,
		domain: domain,
	}
}

func (s *rabbitCommandSender) sendCommand(command Command[any], opts CommandOptions) error {
	if command.Name == "" || opts.Domain == "" || opts.TargetName == "" {
		return fmt.Errorf("command name, domain, and target are required")
	}

	cmdBytes, err := json.Marshal(command)
	if err != nil {
		log.Printf("Failed to marshal command: %v", err)
		return err
	}

	headers := map[string]string{
		"sourceApplication": s.domain.Name,
		"delivery_mode":     "persistent",
		"message_id":        command.CommandId,
		"timestamp":         fmt.Sprintf("%d", time.Now().Unix()),
		"app_id":            s.domain.Name,
	}

	err = s.client.PublishJson(s.domain.DirectExchange, opts.TargetName, cmdBytes, ChannelForCommands, headers)
	if err != nil {
		log.Printf("Failed send command: %v", err)
		return err
	}

	return nil
}
