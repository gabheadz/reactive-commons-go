package rabbit

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ChannelInterface defines the methods we need from amqp.Channel for testing
type ChannelInterface interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	QueueUnbind(name, key, exchange string, args amqp.Table) error
}

func DeclareExchange(channel ChannelInterface, exchangeName string, exchangeType string,
	durable bool, autoDelete bool, internal bool, noWait bool) error {
	err := channel.ExchangeDeclare(exchangeName, exchangeType, durable, autoDelete, internal, noWait, nil)
	if err != nil {
		log.Panicf("declare exchange failed: %s", err)
	}
	return nil
}

func DeclareQueue(channel ChannelInterface, queueType string, queueName string, durable bool, autoDelete bool, exclusive bool,
	noWait bool) error {

	args := amqp.Table{}
	if queueType != "" {
		args["x-queue-type"] = resolveQueueType(queueType, autoDelete, exclusive)
	}

	_, err := channel.QueueDeclare(queueName, durable, autoDelete, exclusive, noWait, args)
	if err != nil {
		log.Panicf("declare queue failed: %s", err)
		return err
	}
	return nil
}

func DeclareDLQ(channel ChannelInterface, queueType string, originQueue string, retryTarget string, retryTimer int, maxLengthBytesOpt int) error {

	args := amqp.Table{
		"x-dead-letter-exchange": retryTarget,
		"x-message-ttl":          retryTimer,
		"x-queue-type":           resolveQueueType(queueType, false, false),
	}
	if maxLengthBytesOpt > 0 {
		args["x-max-length-bytes"] = maxLengthBytesOpt
	}
	_, err := channel.QueueDeclare(originQueue+".DLQ", true, false, false, false, args)
	if err != nil {
		log.Panicf("declare dlq queue failed: %s", err)
		return err
	}
	return nil
}

func Bind(channel ChannelInterface, name string, key string, exchange string, exclusive bool) error {
	err := channel.QueueBind(name, key, exchange, exclusive, nil)
	if err != nil {
		log.Panicf("bind queue failed: %s", err)
	}
	return nil
}

func UnBind(channel ChannelInterface, name string, key string, exchange string) error {
	err := channel.QueueUnbind(name, key, exchange, nil)
	if err != nil {
		log.Panicf("unbind queue failed: %s", err)
	}
	return nil
}

func resolveQueueType(queueType string, autoDelete bool, exclusive bool) string {
	if queueType == "quorum" && (autoDelete || exclusive) {
		return "classic"
	}
	return queueType
}
