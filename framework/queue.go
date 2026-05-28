package framework

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Queue is the small ORDIN abstraction for background jobs/messages.
type Queue interface {
	Publish(ctx context.Context, queue string, payload []byte, options ...PublishOption) error
	PublishJSON(ctx context.Context, queue string, payload any, options ...PublishOption) error
	Consume(ctx context.Context, queue string, handler JobHandler, options ...ConsumeOption) error
	Close() error
}

// Job is a single queue delivery.
type Job struct {
	Queue       string
	Body        []byte
	ContentType string
	Headers     map[string]any
	Timestamp   time.Time
}

// DecodeJSON decodes a JSON job body into dst.
func (j Job) DecodeJSON(dst any) error {
	return json.Unmarshal(j.Body, dst)
}

// JobHandler handles one queue delivery. Return an error to reject/nack it.
type JobHandler func(context.Context, Job) error

// PublishOptions configures a queue publish.
type PublishOptions struct {
	ContentType  string
	Persistent   bool
	Exchange     string
	RoutingKey   string
	Headers      map[string]any
	Delay        time.Duration
	Mandatory    bool
	Immediate    bool
	DeliveryMode uint8
}

// PublishOption updates PublishOptions.
type PublishOption func(*PublishOptions)

// WithQueueContentType sets the message content type.
func WithQueueContentType(contentType string) PublishOption {
	return func(options *PublishOptions) {
		options.ContentType = strings.TrimSpace(contentType)
	}
}

// WithQueueHeaders adds AMQP headers.
func WithQueueHeaders(headers map[string]any) PublishOption {
	return func(options *PublishOptions) {
		if options.Headers == nil {
			options.Headers = map[string]any{}
		}
		for key, value := range headers {
			options.Headers[key] = value
		}
	}
}

// WithExchange publishes to a custom exchange and routing key.
func WithExchange(exchange, routingKey string) PublishOption {
	return func(options *PublishOptions) {
		options.Exchange = strings.TrimSpace(exchange)
		options.RoutingKey = strings.TrimSpace(routingKey)
	}
}

// WithTransientMessage disables persistent delivery for this message.
func WithTransientMessage() PublishOption {
	return func(options *PublishOptions) {
		options.Persistent = false
		options.DeliveryMode = amqp.Transient
	}
}

// WithQueueDelay sets the x-delay header for RabbitMQ delayed-message exchange setups.
// It requires the RabbitMQ delayed message plugin and a compatible exchange.
func WithQueueDelay(delay time.Duration) PublishOption {
	return func(options *PublishOptions) {
		options.Delay = delay
	}
}

// ConsumeOptions configures queue consumption.
type ConsumeOptions struct {
	ConsumerName   string
	Prefetch       int
	AutoAck        bool
	RequeueOnErr   bool
	DeclareDurable bool
}

// ConsumeOption updates ConsumeOptions.
type ConsumeOption func(*ConsumeOptions)

// WithConsumerName sets an AMQP consumer name.
func WithConsumerName(name string) ConsumeOption {
	return func(options *ConsumeOptions) {
		options.ConsumerName = strings.TrimSpace(name)
	}
}

// WithPrefetch sets channel QoS prefetch count.
func WithPrefetch(count int) ConsumeOption {
	return func(options *ConsumeOptions) {
		options.Prefetch = count
	}
}

// WithAutoAck enables automatic acknowledgements.
func WithAutoAck() ConsumeOption {
	return func(options *ConsumeOptions) {
		options.AutoAck = true
	}
}

// WithRequeueOnError requeues messages when the handler returns an error.
func WithRequeueOnError() ConsumeOption {
	return func(options *ConsumeOptions) {
		options.RequeueOnErr = true
	}
}

// RabbitMQConfig configures RabbitMQ/AMQP 0.9.1.
type RabbitMQConfig struct {
	URL string
}

// RabbitMQConfigFromEnv reads RABBITMQ_URL by default.
func RabbitMQConfigFromEnv(prefix string) RabbitMQConfig {
	prefix = strings.Trim(strings.ToUpper(prefix), "_")
	if prefix == "" {
		prefix = "RABBITMQ"
	}
	return RabbitMQConfig{URL: getenv(prefix+"_URL", "amqp://guest:guest@localhost:5672/")}
}

// RabbitQueue is the RabbitMQ implementation of Queue.
type RabbitQueue struct {
	conn *amqp.Connection
}

// NewRabbitQueue creates a RabbitMQ queue backend.
func NewRabbitQueue(config RabbitMQConfig) (*RabbitQueue, error) {
	if strings.TrimSpace(config.URL) == "" {
		return nil, errors.New("rabbitmq url is empty")
	}
	conn, err := amqp.Dial(config.URL)
	if err != nil {
		return nil, err
	}
	return &RabbitQueue{conn: conn}, nil
}

// MustRabbitQueue creates a RabbitMQ queue backend or panics.
func MustRabbitQueue(config RabbitMQConfig) *RabbitQueue {
	queue, err := NewRabbitQueue(config)
	if err != nil {
		panic(err)
	}
	return queue
}

// Connection exposes the underlying AMQP connection for advanced workflows.
func (q *RabbitQueue) Connection() *amqp.Connection {
	return q.conn
}

// Close closes the AMQP connection.
func (q *RabbitQueue) Close() error {
	if q == nil || q.conn == nil {
		return nil
	}
	return q.conn.Close()
}

// Publish sends a message to a queue. By default the message is persistent and
// the target queue is declared durable.
func (q *RabbitQueue) Publish(ctx context.Context, queue string, payload []byte, options ...PublishOption) error {
	if q == nil || q.conn == nil {
		return errors.New("rabbitmq queue is not configured")
	}
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return errors.New("queue name is empty")
	}

	publish := PublishOptions{
		ContentType:  "application/octet-stream",
		Persistent:   true,
		RoutingKey:   queue,
		DeliveryMode: amqp.Persistent,
	}
	for _, option := range options {
		if option != nil {
			option(&publish)
		}
	}
	if publish.Persistent {
		publish.DeliveryMode = amqp.Persistent
	}
	if publish.RoutingKey == "" {
		publish.RoutingKey = queue
	}

	channel, err := q.conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	if publish.Exchange == "" {
		if _, err := declareDurableQueue(channel, queue); err != nil {
			return err
		}
	}

	if publish.Delay > 0 {
		if publish.Headers == nil {
			publish.Headers = map[string]any{}
		}
		publish.Headers["x-delay"] = int64(publish.Delay / time.Millisecond)
	}

	return channel.PublishWithContext(ctx, publish.Exchange, publish.RoutingKey, publish.Mandatory, publish.Immediate, amqp.Publishing{
		ContentType:  publish.ContentType,
		DeliveryMode: publish.DeliveryMode,
		Body:         payload,
		Headers:      amqp.Table(publish.Headers),
		Timestamp:    time.Now(),
	})
}

// PublishJSON marshals payload as JSON and publishes it.
func (q *RabbitQueue) PublishJSON(ctx context.Context, queue string, payload any, options ...PublishOption) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	options = append([]PublishOption{WithQueueContentType("application/json")}, options...)
	return q.Publish(ctx, queue, body, options...)
}

// Consume handles messages until ctx is cancelled or the broker returns an error.
func (q *RabbitQueue) Consume(ctx context.Context, queue string, handler JobHandler, options ...ConsumeOption) error {
	if q == nil || q.conn == nil {
		return errors.New("rabbitmq queue is not configured")
	}
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return errors.New("queue name is empty")
	}
	if handler == nil {
		return errors.New("job handler is nil")
	}

	consume := ConsumeOptions{Prefetch: 1, DeclareDurable: true}
	for _, option := range options {
		if option != nil {
			option(&consume)
		}
	}

	channel, err := q.conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	if consume.DeclareDurable {
		if _, err := declareDurableQueue(channel, queue); err != nil {
			return err
		}
	}
	if consume.Prefetch > 0 {
		if err := channel.Qos(consume.Prefetch, 0, false); err != nil {
			return err
		}
	}

	deliveries, err := channel.Consume(queue, consume.ConsumerName, consume.AutoAck, false, false, false, nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return nil
			}

			job := Job{
				Queue:       queue,
				Body:        delivery.Body,
				ContentType: delivery.ContentType,
				Headers:     map[string]any(delivery.Headers),
				Timestamp:   delivery.Timestamp,
			}

			err := handler(ctx, job)
			if consume.AutoAck {
				continue
			}
			if err != nil {
				_ = delivery.Nack(false, consume.RequeueOnErr)
				continue
			}
			_ = delivery.Ack(false)
		}
	}
}

func declareDurableQueue(channel *amqp.Channel, name string) (amqp.Queue, error) {
	return channel.QueueDeclare(name, true, false, false, false, nil)
}
