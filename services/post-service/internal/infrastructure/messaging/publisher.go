package messaging

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"post-service/pkg/logger"
	"time"
)

type EventPublisher struct {
	connection   *amqp.Connection
	channel      *amqp.Channel
	exchangeName string
	logger       *logger.Logger
	done         chan error
}

type PostCreatedEvent struct {
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
}

type PostUpdatedEvent struct {
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Published bool      `json:"published"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PostDeletedEvent struct {
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	DeletedAt time.Time `json:"deleted_at"`
}

func NewEventPublisher(rabbitMQURL, exchangeName string, logger *logger.Logger) (*EventPublisher, error) {
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange
	err = ch.ExchangeDeclare(
		exchangeName, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	publisher := &EventPublisher{
		connection:   conn,
		channel:      ch,
		exchangeName: exchangeName,
		logger:       logger,
		done:         make(chan error),
	}

	// Monitor connection
	go publisher.monitorConnection()

	logger.Info("Event publisher initialized successfully")
	return publisher, nil
}

func (p *EventPublisher) PublishPostCreated(event PostCreatedEvent) error {
	return p.publishEvent("post.created", event)
}

func (p *EventPublisher) PublishPostUpdated(event PostUpdatedEvent) error {
	return p.publishEvent("post.updated", event)
}

func (p *EventPublisher) PublishPostDeleted(event PostDeletedEvent) error {
	return p.publishEvent("post.deleted", event)
}

func (p *EventPublisher) publishEvent(routingKey string, event interface{}) error {
	if p.channel == nil {
		return fmt.Errorf("publisher channel is not available")
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.channel.Publish(
		p.exchangeName, // exchange
		routingKey,     // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // Make message persistent
			Timestamp:    time.Now(),
			MessageId:    fmt.Sprintf("%s-%d", routingKey, time.Now().UnixNano()),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Info(fmt.Sprintf("Published event: %s with %d bytes", routingKey, len(body)))
	return nil
}

func (p *EventPublisher) monitorConnection() {
	for {
		select {
		case err := <-p.connection.NotifyClose(make(chan *amqp.Error)):
			if err != nil {
				p.logger.Error(fmt.Sprintf("RabbitMQ connection closed: %v", err))
				p.done <- err
				return
			}
		case err := <-p.channel.NotifyClose(make(chan *amqp.Error)):
			if err != nil {
				p.logger.Error(fmt.Sprintf("RabbitMQ channel closed: %v", err))
				p.done <- err
				return
			}
		}
	}
}

func (p *EventPublisher) IsConnected() bool {
	return p.connection != nil && !p.connection.IsClosed() && p.channel != nil
}

func (p *EventPublisher) Reconnect(rabbitMQURL string) error {
	p.logger.Info("Attempting to reconnect to RabbitMQ...")

	// Close existing connections
	p.Close()

	// Create new connection
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return fmt.Errorf("failed to reconnect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel during reconnect: %w", err)
	}

	// Redeclare exchange
	err = ch.ExchangeDeclare(
		p.exchangeName, // name
		"topic",        // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("failed to redeclare exchange during reconnect: %w", err)
	}

	p.connection = conn
	p.channel = ch
	p.done = make(chan error)

	// Restart monitoring
	go p.monitorConnection()

	p.logger.Info("Successfully reconnected to RabbitMQ")
	return nil
}

func (p *EventPublisher) Close() error {
	p.logger.Info("Closing event publisher...")

	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			p.logger.Error(fmt.Sprintf("Failed to close channel: %v", err))
		}
		p.channel = nil
	}

	if p.connection != nil {
		if err := p.connection.Close(); err != nil {
			p.logger.Error(fmt.Sprintf("Failed to close connection: %v", err))
		}
		p.connection = nil
	}

	p.logger.Info("Event publisher closed")
	return nil
}

func (p *EventPublisher) HealthCheck() error {
	if !p.IsConnected() {
		return fmt.Errorf("event publisher is not connected")
	}
	return nil
}
