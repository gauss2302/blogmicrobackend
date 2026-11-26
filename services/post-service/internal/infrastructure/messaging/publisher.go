package messaging

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"post-service/pkg/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Timings for the connection owned by run. Fixed values: a dead broker costs
// one dial attempt and one warning every reconnectDelay.
const (
	reconnectDelay = 5 * time.Second
	dialTimeout    = 5 * time.Second
	closeTimeout   = 5 * time.Second
)

// EventPublisher publishes post events to a RabbitMQ topic exchange. A single
// goroutine (run) owns the connection: it reconnects whenever the connection or
// the channel closes, so publishing never waits on a broken connection; while
// disconnected, publish fails immediately.
type EventPublisher struct {
	url          string
	exchangeName string
	retryDelay   time.Duration
	logger       *logger.Logger

	mu      sync.Mutex
	channel *amqp.Channel // nil while disconnected

	stop      chan struct{}
	stopped   chan struct{}
	closeOnce sync.Once
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

// NewEventPublisher returns an error only for a malformed URL. An unreachable
// broker is not an error: the publisher keeps reconnecting in the background
// and publishing fails until it is connected.
func NewEventPublisher(rabbitMQURL, exchangeName string, logger *logger.Logger) (*EventPublisher, error) {
	return newEventPublisher(rabbitMQURL, exchangeName, reconnectDelay, logger)
}

func newEventPublisher(rabbitMQURL, exchangeName string, retryDelay time.Duration, logger *logger.Logger) (*EventPublisher, error) {
	if _, err := amqp.ParseURI(rabbitMQURL); err != nil {
		return nil, fmt.Errorf("invalid RabbitMQ URL: %w", err)
	}
	p := &EventPublisher{
		url:          rabbitMQURL,
		exchangeName: exchangeName,
		retryDelay:   retryDelay,
		logger:       logger,
		stop:         make(chan struct{}),
		stopped:      make(chan struct{}),
	}

	// First attempt is synchronous so a reachable broker is ready on return.
	conn, ch, err := p.connect()
	if err != nil {
		logger.Warn(fmt.Sprintf("RabbitMQ unavailable, will retry every %s: %v", retryDelay, err))
	} else {
		logger.Info("Event publisher connected to RabbitMQ")
	}
	go p.run(conn, ch)
	return p, nil
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
	p.mu.Lock()
	ch := p.channel
	p.mu.Unlock()
	if ch == nil {
		return fmt.Errorf("publisher is not connected to RabbitMQ")
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// A channel closed since we read it returns amqp.ErrClosed; the close
	// notifications are buffered, so the library never holds its lock on them.
	err = ch.Publish(
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

// connect dials the broker, opens a channel and declares the exchange.
func (p *EventPublisher) connect() (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.DialConfig(p.url, amqp.Config{
		Locale: "en_US", // amqp.Dial defaults, with a shorter dial timeout
		Dial:   amqp.DefaultDial(dialTimeout),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.CloseDeadline(time.Now().Add(closeTimeout))
		return nil, nil, fmt.Errorf("failed to open channel: %w", err)
	}

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
		_ = conn.CloseDeadline(time.Now().Add(closeTimeout))
		return nil, nil, fmt.Errorf("failed to declare exchange: %w", err)
	}
	return conn, ch, nil
}

// run owns the connection until Close: it serves the current one until the
// connection or channel closes, then waits retryDelay and reconnects.
func (p *EventPublisher) run(conn *amqp.Connection, ch *amqp.Channel) {
	defer close(p.stopped)
	for {
		if conn != nil && !p.serve(conn, ch) {
			return
		}
		select {
		case <-p.stop:
			return
		case <-time.After(p.retryDelay):
		}
		var err error
		if conn, ch, err = p.connect(); err != nil {
			p.logger.Warn(fmt.Sprintf("RabbitMQ reconnect failed, retrying in %s: %v", p.retryDelay, err))
		} else {
			p.logger.Info("Event publisher reconnected to RabbitMQ")
		}
	}
}

// serve exposes ch to publishers until the connection or channel closes
// (returns true) or Close is called (returns false). Either way the connection
// is closed and publishers see no channel when it returns.
func (p *EventPublisher) serve(conn *amqp.Connection, ch *amqp.Channel) bool {
	// Buffered: the library sends at most one error per listener, so its
	// shutdown never blocks on us. A listener registered after the close is
	// closed at once.
	connClosed := conn.NotifyClose(make(chan *amqp.Error, 1))
	chClosed := ch.NotifyClose(make(chan *amqp.Error, 1))

	p.mu.Lock()
	p.channel = ch
	p.mu.Unlock()

	var reason *amqp.Error
	lost := true
	select {
	case reason = <-connClosed:
	case reason = <-chClosed:
	case <-p.stop:
		lost = false
	}

	p.mu.Lock()
	p.channel = nil
	p.mu.Unlock()
	_ = conn.CloseDeadline(time.Now().Add(closeTimeout)) // ErrClosed if already gone

	if lost {
		p.logger.Error(fmt.Sprintf("RabbitMQ connection or channel closed, reconnecting in %s: %v", p.retryDelay, reason))
	}
	return lost
}

func (p *EventPublisher) IsConnected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.channel != nil && !p.channel.IsClosed()
}

// Close stops reconnecting, closes the connection and waits for the owner
// goroutine to exit. Publishing after Close returns an error.
func (p *EventPublisher) Close() error {
	p.closeOnce.Do(func() {
		p.logger.Info("Closing event publisher...")
		close(p.stop)
		<-p.stopped
		p.logger.Info("Event publisher closed")
	})
	return nil
}

func (p *EventPublisher) HealthCheck() error {
	if !p.IsConnected() {
		return fmt.Errorf("event publisher is not connected")
	}
	return nil
}
