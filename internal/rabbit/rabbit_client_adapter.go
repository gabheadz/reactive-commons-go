package rabbit

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitChannel wraps amqp.Channel to implement ChannelInterface
type RabbitChannel struct {
	Channel *amqp.Channel
}

func (rc *RabbitChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	return rc.Channel.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

func (rc *RabbitChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	return rc.Channel.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
}

func (rc *RabbitChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	return rc.Channel.QueueBind(name, key, exchange, noWait, args)
}

func (rc *RabbitChannel) QueueUnbind(name, key, exchange string, args amqp.Table) error {
	return rc.Channel.QueueUnbind(name, key, exchange, args)
}

// RabbitClientAdapter adapts RabbitClient to return ChannelInterface
type RabbitClientAdapter struct {
	*RabbitClient
}

func NewRabbitClientAdapter(appName, connAddr string) (*RabbitClientAdapter, error) {
	client, err := NewRabbitClient(appName, connAddr)
	if err != nil {
		return nil, err
	}
	return &RabbitClientAdapter{RabbitClient: client}, nil
}

func (a *RabbitClientAdapter) CreateChannelInterface(name string) (ChannelInterface, error) {
	ch, err := a.RabbitClient.CreateChannel(name)
	if err != nil {
		return nil, err
	}
	return &RabbitChannel{Channel: ch}, nil
}

func (a *RabbitClientAdapter) GetChannelInterface(name string) (ChannelInterface, error) {
	ch, err := a.RabbitClient.GetChannel(name)
	if err != nil {
		return nil, err
	}
	return &RabbitChannel{Channel: ch}, nil
}

// Wrapper methods to maintain compatibility
func (r *RabbitClient) CreateChannelInterface(name string) (ChannelInterface, error) {
	ch, err := r.CreateChannel(name)
	if err != nil {
		return nil, err
	}
	return &RabbitChannel{Channel: ch}, nil
}

func (r *RabbitClient) GetChannelInterface(name string) (ChannelInterface, error) {
	ch, err := r.GetChannel(name)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, fmt.Errorf("channel not found: %s", name)
	}
	return &RabbitChannel{Channel: ch}, nil
}

func (r *RabbitClient) ConsumeOneWithTimeout(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error) {
	return r.ConsumeOne(channelKey, queueName, correlationId, timeout)
}
