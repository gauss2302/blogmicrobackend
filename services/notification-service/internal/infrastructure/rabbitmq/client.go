package rabbitmq

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"notification-service/internal/config"
	"notification-service/internal/domain/entities"
	"notification-service/pkg/logger"
	"time"
)

type Client struct {
	config     config.RabbitMQConfig
	connection *amqp.Connection
	channel    *amqp.Channel
	logger     *logger.Logger
	done       chan error
}

type MessageHandler func([]byte) error

func NewClient(cfg config.RabbitMQConfig, logger *logger.Logger) *Client {
	return &Client{
		config: cfg,
		logger: logger,
		done:   make(chan error),
	}
}

func (c *Client) Connect() error {
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

	queue, err := c.channel.QueueDeclare(
		c.config.QueueName,
		true,
		false,
		false,
		false,
		nil,
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

	c.logger.Info("Connected to rabbit successfully")
	return nil
}

func (c *Client) StartConsuming(handler MessageHandler) error {
	msgs, err := c.channel.Consume(
		c.config.QueueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return fmt.Errorf("failed to reg consumer: %w", err)
	}

	go func() {
		for d := range msgs {
			c.processMessages(d, handler)
		}
	}()

	c.logger.Info("Start consuming messages from rabbit")
	return nil
}

func (c *Client) processMessages(delivery amqp.Delivery, handler MessageHandler) {
	var err error
	retries := 0

	for retries <= c.config.MaxRetries {
		err = handler(delivery.Body)
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
	if rejectErr := delivery.Reject(false); rejectErr != nil {
		c.logger.Error(fmt.Sprintf("failed to reject message: %v", rejectErr))
	}

}

func (c *Client) PublishEvent(routingKey string, event interface{}) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = c.channel.Publish(
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

func (c *Client) Close() error {
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
	return c.connection != nil && !c.connection.IsClosed()
}

func (c *Client) Reconnect() error {
	c.logger.Info("attempt to reconnect to rabbit")

	if c.IsConnected() {
		c.Close()
	}

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
