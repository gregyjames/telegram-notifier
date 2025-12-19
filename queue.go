package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MessageQueue interface defines queue operations
type MessageQueue interface {
	Publish(ctx context.Context, message RequestBody) error
	StartConsumer(handler func(RequestBody) error)
	Close() error
}

// RabbitMQQueue implements MessageQueue using RabbitMQ
type RabbitMQQueue struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	queue *amqp.Queue
}

func NewRabbitMQQueue(config Configuration) (*RabbitMQQueue, error) {
	address := fmt.Sprintf("amqp://%s:%s@%s:%d/",
		config.RabbitMQ.Username,
		config.RabbitMQ.Password,
		config.RabbitMQ.Host,
		config.RabbitMQ.Port)

	conn, err := amqp.Dial(address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	queue, err := ch.QueueDeclare(
		"telegram-notifier",
		true,  // durable
		true,  // delete when unused
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	return &RabbitMQQueue{
		conn:  conn,
		ch:    ch,
		queue: &queue,
	}, nil
}

func (r *RabbitMQQueue) Publish(ctx context.Context, message RequestBody) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = r.ch.PublishWithContext(ctx,
		"",           // exchange
		r.queue.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	return nil
}

func (r *RabbitMQQueue) StartConsumer(handler func(RequestBody) error) {
	msgs, err := r.ch.Consume(
		r.queue.Name, // queue
		"",           // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		log.Printf("Failed to register consumer: %v", err)
		return
	}

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)

			var evt RequestBody
			if err := json.Unmarshal(d.Body, &evt); err != nil {
				log.Printf("Error parsing json from RabbitMQ: %v", err)
				continue
			}

			if err := handler(evt); err != nil {
				log.Printf("Error in handler: %v", err)
			}
		}
	}()
}

func (r *RabbitMQQueue) Close() error {
	if r.ch != nil {
		r.ch.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// ChannelQueue implements MessageQueue using Go channels
type ChannelQueue struct {
	messageChan chan RequestBody
}

func NewChannelQueue(bufferSize int) *ChannelQueue {
	return &ChannelQueue{
		messageChan: make(chan RequestBody, bufferSize),
	}
}

func (c *ChannelQueue) Publish(ctx context.Context, message RequestBody) error {
	select {
	case c.messageChan <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("channel send timeout")
	}
}

func (c *ChannelQueue) StartConsumer(handler func(RequestBody) error) {
	go func() {
		for evt := range c.messageChan {
			log.Printf("Received a message: %s", evt.Message)
			if err := handler(evt); err != nil {
				log.Printf("Error in handler: %v", err)
			}
		}
	}()
}

func (c *ChannelQueue) Close() error {
	close(c.messageChan)
	return nil
}