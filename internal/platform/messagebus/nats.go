package messagebus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jokeoa/simple-order-microservices/internal/platform/events"
	"github.com/nats-io/nats.go"
)

const (
	PaymentStream          = "PAYMENTS"
	NotificationConsumer   = "notification-service"
	paymentDuplicateWindow = 24 * time.Hour
)

type NATSConfig struct {
	URL string
}

func Connect(config NATSConfig) (*nats.Conn, error) {
	if config.URL == "" {
		return nil, errors.New("nats url is required")
	}

	conn, err := nats.Connect(config.URL, nats.Name("simple-order-microservices"))
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	return conn, nil
}

func EnsurePaymentStream(ctx context.Context, conn *nats.Conn) (nats.JetStreamContext, error) {
	js, err := conn.JetStream(nats.Context(ctx))
	if err != nil {
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	config := &nats.StreamConfig{
		Name:       PaymentStream,
		Subjects:   []string{events.PaymentCompletedSubject, events.PaymentCompletedDLQ},
		Retention:  nats.LimitsPolicy,
		Storage:    nats.FileStorage,
		Replicas:   1,
		Duplicates: paymentDuplicateWindow,
	}

	if _, err := js.StreamInfo(PaymentStream, nats.Context(ctx)); err != nil {
		if !errors.Is(err, nats.ErrStreamNotFound) {
			return nil, fmt.Errorf("load payment stream: %w", err)
		}
		if _, err := js.AddStream(config, nats.Context(ctx)); err != nil {
			return nil, fmt.Errorf("add payment stream: %w", err)
		}
		return js, nil
	}

	if _, err := js.UpdateStream(config, nats.Context(ctx)); err != nil {
		return nil, fmt.Errorf("update payment stream: %w", err)
	}

	return js, nil
}

type PaymentCompletedPublisher struct {
	js nats.JetStreamContext
}

func NewPaymentCompletedPublisher(js nats.JetStreamContext) *PaymentCompletedPublisher {
	return &PaymentCompletedPublisher{js: js}
}

func (p *PaymentCompletedPublisher) PublishPaymentCompleted(ctx context.Context, event events.PaymentCompleted) error {
	return p.publish(ctx, events.PaymentCompletedSubject, event)
}

func (p *PaymentCompletedPublisher) PublishPaymentCompletedDLQ(ctx context.Context, event events.PaymentCompleted) error {
	return p.publish(ctx, events.PaymentCompletedDLQ, event)
}

func (p *PaymentCompletedPublisher) publish(ctx context.Context, subject string, event events.PaymentCompleted) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", subject, err)
	}

	message := &nats.Msg{
		Subject: subject,
		Header: nats.Header{
			nats.MsgIdHdr: []string{event.MessageID},
		},
		Data: payload,
	}
	if subject == events.PaymentCompletedDLQ {
		message.Header.Set(nats.MsgIdHdr, event.MessageID+":dlq")
	}

	if _, err := p.js.PublishMsg(message, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish %s event: %w", subject, err)
	}

	return nil
}

type PaymentCompletedHandler interface {
	HandlePaymentCompleted(ctx context.Context, event events.PaymentCompleted) error
}

type PaymentCompletedConsumerConfig struct {
	MaxDeliver int
	RetryDelay time.Duration
	FetchWait  time.Duration
}

type PaymentCompletedConsumer struct {
	js      nats.JetStreamContext
	handler PaymentCompletedHandler
	dlq     *PaymentCompletedPublisher
	config  PaymentCompletedConsumerConfig
	logger  *log.Logger
}

func NewPaymentCompletedConsumer(
	js nats.JetStreamContext,
	handler PaymentCompletedHandler,
	dlq *PaymentCompletedPublisher,
	config PaymentCompletedConsumerConfig,
	logger *log.Logger,
) *PaymentCompletedConsumer {
	if config.MaxDeliver <= 0 {
		config.MaxDeliver = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 2 * time.Second
	}
	if config.FetchWait <= 0 {
		config.FetchWait = 2 * time.Second
	}

	return &PaymentCompletedConsumer{
		js:      js,
		handler: handler,
		dlq:     dlq,
		config:  config,
		logger:  logger,
	}
}

func (c *PaymentCompletedConsumer) Run(ctx context.Context) error {
	if err := c.ensureConsumer(ctx); err != nil {
		return err
	}

	subscription, err := c.js.PullSubscribe(
		events.PaymentCompletedSubject,
		NotificationConsumer,
		nats.Bind(PaymentStream, NotificationConsumer),
		nats.ManualAck(),
	)
	if err != nil {
		return fmt.Errorf("subscribe payment completed: %w", err)
	}
	defer func() {
		if err := subscription.Drain(); err != nil {
			c.logger.Printf("drain payment completed subscription: %v", err)
		}
	}()

	for {
		if ctx.Err() != nil {
			return nil
		}

		fetchCtx, cancel := context.WithTimeout(ctx, c.config.FetchWait)
		messages, err := subscription.Fetch(1, nats.Context(fetchCtx))
		cancel()
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.Canceled) {
				continue
			}
			return fmt.Errorf("fetch payment completed: %w", err)
		}

		for _, message := range messages {
			c.handleMessage(ctx, message)
		}
	}
}

func (c *PaymentCompletedConsumer) ensureConsumer(ctx context.Context) error {
	config := &nats.ConsumerConfig{
		Durable:       NotificationConsumer,
		Name:          NotificationConsumer,
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    c.config.MaxDeliver,
		FilterSubject: events.PaymentCompletedSubject,
		MaxAckPending: 1,
	}

	if _, err := c.js.ConsumerInfo(PaymentStream, NotificationConsumer, nats.Context(ctx)); err != nil {
		if !errors.Is(err, nats.ErrConsumerNotFound) {
			return fmt.Errorf("load notification consumer: %w", err)
		}
		if _, err := c.js.AddConsumer(PaymentStream, config, nats.Context(ctx)); err != nil {
			return fmt.Errorf("add notification consumer: %w", err)
		}
		return nil
	}

	if _, err := c.js.UpdateConsumer(PaymentStream, config, nats.Context(ctx)); err != nil {
		return fmt.Errorf("update notification consumer: %w", err)
	}

	return nil
}

func (c *PaymentCompletedConsumer) handleMessage(ctx context.Context, message *nats.Msg) {
	var event events.PaymentCompleted
	if err := json.Unmarshal(message.Data, &event); err != nil {
		c.handleFailure(ctx, message, event, fmt.Errorf("decode payment.completed: %w", err))
		return
	}

	if err := c.handler.HandlePaymentCompleted(ctx, event); err != nil {
		c.handleFailure(ctx, message, event, err)
		return
	}

	if err := message.Ack(); err != nil {
		c.logger.Printf("ack payment.completed message_id=%s: %v", event.MessageID, err)
	}
}

func (c *PaymentCompletedConsumer) handleFailure(ctx context.Context, message *nats.Msg, event events.PaymentCompleted, handleErr error) {
	metadata, err := message.Metadata()
	if err != nil {
		c.logger.Printf("load payment.completed metadata: %v", err)
		_ = message.NakWithDelay(c.config.RetryDelay)
		return
	}

	if int(metadata.NumDelivered) < c.config.MaxDeliver {
		c.logger.Printf(
			"retry payment.completed stream_seq=%d delivery=%d max_deliver=%d err=%v",
			metadata.Sequence.Stream,
			metadata.NumDelivered,
			c.config.MaxDeliver,
			handleErr,
		)
		if err := message.NakWithDelay(c.config.RetryDelay); err != nil {
			c.logger.Printf("nak payment.completed stream_seq=%d: %v", metadata.Sequence.Stream, err)
		}
		return
	}

	if event.MessageID == "" {
		event.MessageID = fmt.Sprintf("invalid:%d", metadata.Sequence.Stream)
	}
	if event.OrderID == "" {
		event.OrderID = "unknown"
	}

	if err := c.dlq.PublishPaymentCompletedDLQ(ctx, event); err != nil {
		c.logger.Printf("publish payment.completed dlq stream_seq=%d: %v", metadata.Sequence.Stream, err)
		if err := message.NakWithDelay(c.config.RetryDelay); err != nil {
			c.logger.Printf("nak payment.completed stream_seq=%d: %v", metadata.Sequence.Stream, err)
		}
		return
	}

	c.logger.Printf(
		"moved payment.completed to dlq stream_seq=%d delivery=%d message_id=%s err=%v",
		metadata.Sequence.Stream,
		metadata.NumDelivered,
		event.MessageID,
		handleErr,
	)
	if err := message.Ack(); err != nil {
		c.logger.Printf("ack dlq-transferred payment.completed stream_seq=%d: %v", metadata.Sequence.Stream, err)
	}
}
