package rabbit

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitClient struct {
	AppName  string
	Conn     *amqp.Connection
	channels map[string]*amqp.Channel
	connAddr string
}

// NewRabbitClient NewRabbitMQ creates a new RabbitMQ connection using the provided configuration and context.
// It attempts to connect to the RabbitMQ server with exponential backoff and a maximum number of retries.
// The connection is closed automatically when the provided context is cancelled.
//
// Parameters:
//
//	cfg: Pointer to RabbitMQConfig containing connection settings.
//	ctx: Context for managing the connection lifecycle (cancels and closes connection when done).
//
// Returns:
//
//	*RabbitMQ: Pointer to the created RabbitMQ struct containing the connection and config.
//	error:    Error if the connection could not be established.
func NewRabbitClient(appName, connAddr string) (*RabbitClient, error) {
	rmq := &RabbitClient{
		AppName:  appName,
		Conn:     nil,
		connAddr: connAddr,
		channels: make(map[string]*amqp.Channel),
	}
	return rmq, nil
}

func (r *RabbitClient) Connect() error {
	log.Printf("Attempting to connect to RabbitMQ at %s", r.connAddr)
	result := make(chan error, 1)
	var conn *amqp.Connection

	go func() {
		var err error
		conn, err = amqp.Dial(r.connAddr)
		if err == nil {
			r.Conn = conn
		}
		result <- err
	}()

	select {
	case err := <-result:
		if err != nil {
			log.Panicf("Failed to connect to RabbitMQ: %v. Connection information: %s", err, r.connAddr)
			return err
		}
		log.Printf("Connected to RabbitMQ")
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout connecting to RabbitMQ at %s", r.connAddr)
	}
}

func (r *RabbitClient) CreateChannel(name string) (*amqp.Channel, error) {
	ch, err := r.Conn.Channel()
	if err != nil {
		log.Printf("Failed to create channel: %v", err)
		return nil, err
	}
	r.channels[name] = ch
	log.Printf("Channel [%s] created, total channels: %d", name, len(r.channels))
	return ch, nil
}

func (r *RabbitClient) GetChannel(name string) (*amqp.Channel, error) {
	channel := r.channels[name]
	if channel == nil {
		return nil, fmt.Errorf("RabbitMQ channel not found: %s", name)
	}

	if channel.IsClosed() {
		return nil, fmt.Errorf("RabbitMQ channel is closed: %s", name)
	}

	return channel, nil
}

func (r *RabbitClient) PublishJson(exchange, key string, msg []byte, channelKey string, headers map[string]string) error {

	channel, err := r.GetChannel(channelKey)
	if err != nil {
		return err
	}

	messageId := headers["message_id"]
	if messageId == "" {
		log.Printf("RabbitMQ publishing message id not found for key: %s", channelKey)
		messageId = uuid.NewString()
	}

	err = channel.Publish(
		exchange,
		key,
		true,
		false,
		amqp.Publishing{
			Headers: amqp.Table{
				"sourceApplication": headers["sourceApplication"],
				"delivery_mode":     headers["delivery_mode"],
				"message_id":        headers["message_id"],
				"timestamp":         headers["timestamp"],
				"app_id":            headers["app_id"],
			},
			ContentType:     "application/json",
			ContentEncoding: "UTF-8",
			DeliveryMode:    amqp.Persistent,
			Priority:        0,
			AppId:           r.AppName,
			Body:            msg,
			CorrelationId:   messageId,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *RabbitClient) PublishJsonQuery(exchange, key string, msg []byte, replyQ string, correlationId string, channelKey string, headers map[string]string) error {

	channel, err := r.GetChannel(channelKey)
	if err != nil {
		return err
	}

	err = channel.Publish(
		exchange,
		key,
		true,
		false,
		amqp.Publishing{
			// Headers: amqp.Table{
			// 	"x-reply_id":             headers["routing_key"],
			// 	"x-serveQuery-id":        headers["delivery_mode"],
			// 	"x-correlation-id":       headers["message_id"],
			// 	"x-reply-timeout-millis": headers["timestamp"],
			// },
			ContentType:     "application/json",
			ContentEncoding: "UTF-8",
			DeliveryMode:    amqp.Persistent,
			Priority:        0,
			AppId:           r.AppName,
			Body:            msg,
			ReplyTo:         replyQ,
			CorrelationId:   correlationId,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

//func (r *RabbitClient) Consume(queueName string) (<-chan amqp.Delivery, error) {
//	channel := r.channels[queueName]
//	if channel == nil {
//		return nil, fmt.Errorf("RabbitMQ channel not found for queue: %s", queueName)
//	}
//
//	if channel.IsClosed() {
//		return nil, fmt.Errorf("RabbitMQ channel is closed for queue: %s", queueName)
//	}
//
//	msgs, err := channel.Consume(
//		queueName,
//		r.AppName,
//		false,
//		false,
//		false,
//		false,
//		nil,
//	)
//	if err != nil {
//		return nil, err
//	}
//
//	return msgs, nil
//}

func (r *RabbitClient) ConsumeOne(channelKey string, queueName string, correlationId string, timeout time.Duration) ([]byte, error) {
	channel, err := r.GetChannel(channelKey)
	if err != nil {
		log.Printf("Failed: %v", err)
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	counter := 1
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for reply with correlationId %s", correlationId)
		}
		log.Printf("RabbitMQ - ConsumeOne - consuming in queue %s ... try: %v", queueName, counter)
		msg, ok, err := channel.Get(queueName, false)
		if err != nil {
			log.Printf("RabbitMQ - ConsumeOne - channel.Get failed: %v", err)
			return nil, err
		}
		if !ok {
			time.Sleep(50 * time.Millisecond)
			counter += 1
			continue
		}
		log.Printf("RabbitMQ - ConsumeOne - msg.coId: %s, expecting: %s", msg.CorrelationId, correlationId)
		if msg.CorrelationId == correlationId {
			msg.Ack(true)
			return msg.Body, nil
		}

		msg.Nack(false, true)
	}
}

func (r *RabbitClient) Close() error {
	if r.Conn != nil && !r.Conn.IsClosed() {
		err := r.Conn.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
