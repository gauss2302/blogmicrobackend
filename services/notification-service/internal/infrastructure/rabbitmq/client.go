package rabbitmq

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"notification-service/internal/config"
	"notification-service/internal/domain/entities"
	"notification-service/pkg/logger"
)

type Client struct {
	config     config.RabbitMQConfig
	connection *amqp.Connection
	channel    *amqp.Channel
	logger     *logger.Logger
	done       chan error
	mu         sync.Mutex
	closed     bool // set by Close: supervise stops instead of reconnecting
}

type MessageHandler func(string, []byte) error

func NewClient(cfg config.RabbitMQConfig, logger *logger.Logger) *Client {
	return &Client{
		config: cfg,
		logger: logger,
		done:   make(chan error),
	}
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error

	c.connection, err = amqp.Dial(c.config.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to rabbit: %w", err)
	}

	c.channel, err = c.connection.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}

	err = c.channel.Qos(c.config.PrefetchCount, 0, false)
	if err != nil {
		return fmt.Errorf("failed to set Qos: %w", err)
	}

	err = c.channel.ExchangeDeclare(
		c.config.ExchangeName,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	err = c.channel.ExchangeDeclare(
		c.config.DLXName,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead-letter exchange: %w", err)
	}

	queue, err := c.channel.QueueDeclare(
		c.config.QueueName,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    c.config.DLXName,
			"x-dead-letter-routing-key": c.config.DLQRoutingKey,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	err = c.channel.QueueBind(
		queue.Name,
		c.config.RoutingKey,
		c.config.ExchangeName,
		false,
		nil)

	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	dlq, err := c.channel.QueueDeclare(
		c.config.DLQName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead-letter queue: %w", err)
	}

	err = c.channel.QueueBind(
		dlq.Name,
		c.config.DLQRoutingKey,
		c.config.DLXName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind dead-letter queue: %w", err)
	}

	c.logger.Info("Connected to rabbit successfully")
	return nil
}

func (c *Client) StartConsuming(handler MessageHandler) error {
	deliveriesDone, err := c.registerConsumer(handler)
	if err != nil {
		return err
	}
	go c.supervise(handler, deliveriesDone)
	c.logger.Info("Start consuming messages from rabbit")
	return nil
}

// registerConsumer starts one delivery loop on the current channel and returns
// a channel closed when that loop ends: on channel or connection close, or
// when the broker cancels the consumer (queue deleted, node failover) while
// the channel stays open. supervise is what brings it back.
func (c *Client) registerConsumer(handler MessageHandler) (<-chan struct{}, error) {
	c.mu.Lock()
	ch := c.channel
	c.mu.Unlock()
	if ch == nil {
		return nil, fmt.Errorf("rabbit channel is not open")
	}

	msgs, err := ch.Consume(
		c.config.QueueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reg consumer: %w", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for d := range msgs {
			c.processMessages(d, handler)
		}
		c.logger.Error("rabbit delivery channel closed")
	}()
	return done, nil
}

// supervise blocks until the connection or channel closes or the delivery loop
// ends, then reconnects and consumes again. Giving up exits the process so the pod restart policy (and
// the liveness probe) can replace a consumer that would otherwise stay "healthy"
// while dropping every post event.
func (c *Client) supervise(handler MessageHandler, deliveriesDone <-chan struct{}) {
	for {
		c.mu.Lock()
		conn := c.connection
		ch := c.channel
		c.mu.Unlock()
		if conn == nil || ch == nil {
			c.stopConsuming(fmt.Errorf("rabbit connection is not open"))
			return
		}

		connClosed := conn.NotifyClose(make(chan *amqp.Error, 1))
		chanClosed := ch.NotifyClose(make(chan *amqp.Error, 1))
		var reason interface{}
		select {
		case reason = <-connClosed:
		case reason = <-chanClosed:
		case <-deliveriesDone:
			reason = "delivery channel closed (consumer cancelled by the broker?)"
		}

		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return // Close was called: shutting down, not a failure
		}
		c.logger.Error(fmt.Sprintf("rabbit consumer disconnected: %v", reason))

		if err := c.Reconnect(); err != nil {
			c.stopConsuming(err)
			return
		}
		var err error
		if deliveriesDone, err = c.registerConsumer(handler); err != nil {
			c.stopConsuming(err)
			return
		}
		c.logger.Info("rabbit consumer resumed after reconnect")
	}
}

func (c *Client) stopConsuming(err error) {
	c.logger.Error(fmt.Sprintf("rabbit consumer stopped permanently: %v", err))
	os.Exit(1)
}

func (c *Client) processMessages(delivery amqp.Delivery, handler MessageHandler) {
	var err error
	retries := 0

	for retries <= c.config.MaxRetries {
		err = handler(delivery.RoutingKey, delivery.Body)
		if err == nil {
			if ackErr := delivery.Ack(false); ackErr != nil {
				c.logger.Error(fmt.Sprintf("failed to ack message: %v", ackErr))
			}
			return
		}

		retries++
		c.logger.Warn(fmt.Sprintf("message processing failed (attempt %d/%d): %v",
			retries, c.config.MaxRetries+1, err))

		if retries <= c.config.MaxRetries {
			time.Sleep(time.Duration(retries) * time.Second)
		}
	}

	// Reject message if more than retries
	c.logger.Error(fmt.Sprintf("message processing failed after %d atttmps, reject message", c.config.MaxRetries+1))
	if nackErr := delivery.Nack(false, false); nackErr != nil {
		c.logger.Error(fmt.Sprintf("failed to dead-letter message: %v", nackErr))
	}

}

func (c *Client) PublishEvent(routingKey string, event interface{}) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	c.mu.Lock()
	ch := c.channel
	c.mu.Unlock()
	if ch == nil {
		return fmt.Errorf("rabbit channel is not open")
	}
	err = ch.Publish(
		c.config.ExchangeName,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish msg: %w", err)
	}

	return nil
}

func (c *Client) HandlePostCreated(body []byte) error {
	var event entities.PostCreatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal post created event: %w", err)
	}
	c.logger.Info(fmt.Sprintf("processing post creatng event: %s, from user: %s", event.PostID, event.UserID))

	return nil
}

func (c *Client) HandlePostUpdated(body []byte) error {
	var event entities.PostUpdatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal post updated event: %w", err)
	}
	c.logger.Info(fmt.Sprintf("Processing post updated event: %s, from user: %v", event.PostID, event.UserID))

	return nil
}

func (c *Client) HandlePostDeleted(body []byte) error {
	var event entities.PostDeletedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal post deleted event: %w", err)
	}

	c.logger.Info(fmt.Sprintf("processing post deleted event: %s from user: %s", event.PostID, event.UserID))
	return nil
}

// Close stops consuming for good and closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return c.closeConnection()
}

func (c *Client) closeConnection() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			c.logger.Error(fmt.Sprintf("failed to close channel %v", err))
		}

		if c.connection != nil {
			if err := c.connection.Close(); err != nil {
				c.logger.Error(fmt.Sprintf("failed to close connection: %v", err))
			}
		}
	}
	c.logger.Info("rabbit connection closed")
	return nil
}

func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connection != nil && !c.connection.IsClosed()
}

func (c *Client) Reconnect() error {
	c.logger.Info("attempt to reconnect to rabbit")
	_ = c.closeConnection()

	for i := 0; i < c.config.MaxRetries; i++ {
		if err := c.Connect(); err != nil {
			c.logger.Warn(fmt.Sprintf("reconnection attempt %d failed: %v", i+1, err))
			time.Sleep(time.Duration(c.config.ReconnectDelay) * time.Second)
			continue
		}
		c.logger.Info("Ok, reconnected to rabbit")
		return nil
	}
	return fmt.Errorf("failed to reconnect after %d attempts", c.config.MaxRetries)
}
