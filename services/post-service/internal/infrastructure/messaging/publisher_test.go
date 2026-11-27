package messaging

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"post-service/pkg/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Integration tests run against a real broker, reached through tcpProxy so a
// test can drop the connection. Start one with
//
//	docker run -d --rm -p 127.0.0.1::5672 rabbitmq:3-alpine
//
// and set TEST_RABBITMQ_URL=amqp://guest:guest@127.0.0.1:<port>/.

func TestPublishDoesNotHangAfterConnectionDrop(t *testing.T) {
	broker := brokerURL(t)
	proxy := startProxy(t, broker.Host, true)
	p, err := NewEventPublisher(proxy.url(broker), testExchange(t), logger.New("error"))
	if err != nil {
		t.Fatalf("NewEventPublisher: %v", err)
	}
	closeWithin(t, p, 5*time.Second)
	waitFor(t, 5*time.Second, "publisher to connect", p.IsConnected)
	if err := publishWithin(t, p, 3*time.Second); err != nil {
		t.Fatalf("publish before the drop: %v", err)
	}

	proxy.cut()
	for i := 0; i < 10; i++ {
		_ = publishWithin(t, p, 3*time.Second) // an error is fine, blocking is not
		time.Sleep(50 * time.Millisecond)
	}
}

// Runs without a broker: an unreachable broker is not a startup error.
func TestPublisherWithoutBroker(t *testing.T) {
	if _, err := NewEventPublisher("http://not-amqp", "x", logger.New("error")); err == nil {
		t.Error("NewEventPublisher accepted a non-AMQP URL")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing listens there now

	p, err := NewEventPublisher("amqp://guest:guest@"+addr+"/", "x", logger.New("error"))
	if err != nil {
		t.Fatalf("NewEventPublisher with the broker down: %v", err)
	}
	closeWithin(t, p, time.Second)
	if err := publishWithin(t, p, 100*time.Millisecond); err == nil {
		t.Error("publish succeeded with the broker down")
	}
}

func TestPublisherConnectsWhenBrokerAppears(t *testing.T) {
	broker := brokerURL(t)
	proxy := startProxy(t, broker.Host, false)
	p, err := newEventPublisher(proxy.url(broker), testExchange(t), 50*time.Millisecond, logger.New("error"))
	if err != nil {
		t.Fatalf("NewEventPublisher with the broker down: %v", err)
	}
	closeWithin(t, p, 5*time.Second)
	if err := publishWithin(t, p, 100*time.Millisecond); err == nil {
		t.Fatal("publish succeeded with the broker down")
	}

	proxy.setUp(true)
	waitFor(t, 5*time.Second, "publish to succeed once the broker is up", func() bool {
		return publishWithin(t, p, time.Second) == nil
	})

	proxy.cut()
	_ = publishWithin(t, p, time.Second)
	waitFor(t, 5*time.Second, "publish to succeed after reconnect", func() bool {
		return publishWithin(t, p, time.Second) == nil
	})
}

func TestPublisherReopensChannelAfterBrokerClosesIt(t *testing.T) {
	broker := brokerURL(t)
	exchange := testExchange(t)
	p, err := newEventPublisher(broker.String(), exchange, 50*time.Millisecond, logger.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	closeWithin(t, p, 5*time.Second)
	waitFor(t, 5*time.Second, "publisher to connect", p.IsConnected)

	admin, err := amqp.Dial(broker.String())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	adminCh, err := admin.Channel()
	if err != nil {
		t.Fatal(err)
	}
	if err := adminCh.ExchangeDelete(exchange, false, false); err != nil {
		t.Fatal(err)
	}

	// Publishing to the missing exchange makes the broker close the channel
	// (404) while the connection stays up.
	_ = publishWithin(t, p, time.Second)
	waitFor(t, 5*time.Second, "publisher to reopen and redeclare the exchange", func() bool {
		return exchangeExists(admin, exchange)
	})
	waitFor(t, 5*time.Second, "publish to succeed", func() bool {
		return publishWithin(t, p, time.Second) == nil
	})
}

func TestCloseDuringPublish(t *testing.T) {
	broker := brokerURL(t)
	p, err := newEventPublisher(broker.String(), testExchange(t), 50*time.Millisecond, logger.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, "publisher to connect", p.IsConnected)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = p.PublishPostCreated(PostCreatedEvent{PostID: "p"})
			}
		}()
	}
	time.Sleep(5 * time.Millisecond)

	done := make(chan struct{})
	go func() { p.Close(); wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close or publishers still blocked after 5s")
	}
	if err := publishWithin(t, p, 100*time.Millisecond); err == nil {
		t.Error("publish after Close succeeded")
	}
}

func exchangeExists(conn *amqp.Connection, name string) bool {
	ch, err := conn.Channel() // a failed passive declare closes the channel, so use a fresh one
	if err != nil {
		return false
	}
	defer ch.Close()
	return ch.ExchangeDeclarePassive(name, "topic", true, false, false, false, nil) == nil
}

// publishWithin fails the test if a publish does not return within d.
func publishWithin(t *testing.T, p *EventPublisher, d time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- p.PublishPostCreated(PostCreatedEvent{PostID: "p", CreatedAt: time.Now()}) }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		t.Fatalf("PublishPostCreated still blocked after %s", d)
		return nil
	}
}

// closeWithin closes p at test end and fails if Close does not return within d.
func closeWithin(t *testing.T, p *EventPublisher, d time.Duration) {
	t.Cleanup(func() {
		done := make(chan struct{})
		go func() { p.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(d):
			t.Errorf("Close still blocked after %s", d)
		}
	})
}

func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s", d, what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func brokerURL(t *testing.T) *url.URL {
	t.Helper()
	raw := os.Getenv("TEST_RABBITMQ_URL")
	if raw == "" {
		t.Skip("TEST_RABBITMQ_URL not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("TEST_RABBITMQ_URL: %v", err)
	}
	return u
}

func testExchange(t *testing.T) string {
	return fmt.Sprintf("test.%s.%d", t.Name(), time.Now().UnixNano())
}

// tcpProxy forwards connections to the broker so a test can drop them (cut) or
// refuse new ones (setUp(false)) without touching the broker itself.
type tcpProxy struct {
	ln     net.Listener
	target string

	mu    sync.Mutex
	up    bool
	conns []net.Conn
}

func startProxy(t *testing.T, target string, up bool) *tcpProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &tcpProxy{ln: ln, target: target, up: up}
	go p.serve()
	t.Cleanup(func() { ln.Close(); p.cut() })
	return p
}

func (p *tcpProxy) url(broker *url.URL) string {
	u := *broker
	u.Host = p.ln.Addr().String()
	return u.String()
}

func (p *tcpProxy) serve() {
	for {
		client, err := p.ln.Accept()
		if err != nil {
			return
		}
		p.mu.Lock()
		up := p.up
		p.mu.Unlock()
		if !up {
			client.Close()
			continue
		}
		broker, err := net.Dial("tcp", p.target)
		if err != nil {
			client.Close()
			continue
		}
		p.mu.Lock()
		p.conns = append(p.conns, client, broker)
		p.mu.Unlock()
		go func() { _, _ = io.Copy(broker, client); broker.Close() }()
		go func() { _, _ = io.Copy(client, broker); client.Close() }()
	}
}

// cut drops every proxied connection, as a broker crash or network loss would.
func (p *tcpProxy) cut() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		c.Close()
	}
	p.conns = nil
}

func (p *tcpProxy) setUp(up bool) {
	p.mu.Lock()
	p.up = up
	p.mu.Unlock()
}
