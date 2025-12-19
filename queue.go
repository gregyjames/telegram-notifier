package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/vmihailenco/msgpack/v5"
)

// MessageQueue interface defines queue operations
type MessageQueue interface {
	Publish(ctx context.Context, message RequestBody) error
	PublishFile(ctx context.Context, file multipart.File, contentType string, fileName string) error
	StartConsumer(handler func(RequestBody) error)
	StartFileConsumer(handler func(FileMessage) error)
	Close() error
}

// RabbitMQQueue implements MessageQueue using RabbitMQ
type RabbitMQQueue struct {
	conn  		 *amqp.Connection
	ch    		 *amqp.Channel
	queueMessage *amqp.Queue
	queueFile	 *amqp.Queue
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
		"telegram-notifier-messages",
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

	queue2, err := ch.QueueDeclare(
		"telegram-notifier-files",
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
		queueMessage: &queue,
		queueFile: &queue2,
	}, nil
}

func (r *RabbitMQQueue) Publish(ctx context.Context, message RequestBody) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = r.ch.PublishWithContext(ctx,
		"",           // exchange
		r.queueMessage.Name, // routing key
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

func (r *RabbitMQQueue) PublishFile(ctx context.Context, file multipart.File, contentType string, fileName string) error {
	data, err := io.ReadAll(file)
	failOnError(err, "failed to read file")

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	payload, err := msgpack.Marshal(&FileMessage{
		ContentType: contentType,
		Data:        data,
		FileName:    fileName,
	})

	failOnError(err, "Error serializing data for RabbitMQ.")

	err = r.ch.PublishWithContext(ctx,
		"",           // exchange
		r.queueFile.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/octet-stream",
			Body:         payload,
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
		r.queueMessage.Name, // queue
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

func (r *RabbitMQQueue) StartFileConsumer(handler func(FileMessage) error) {
	msgs, err := r.ch.Consume(
		r.queueFile.Name, // queue
		"",           // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	failOnError(err, "Failed to register consumer")

	go func() {
		for d := range msgs {
			var evt FileMessage
			
			if err := msgpack.Unmarshal(d.Body, &evt); err != nil {
				log.Printf("failed to unmarshal FileMessage: %v", err)
				continue
			}

			log.Printf("Received a file message: %s", evt.FileName)

			if err := handler(evt); err != nil {
				log.Printf("Error in file handler: %v", err)
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
	fileChan    chan FileMessage
}

func NewChannelQueue(bufferSize int) *ChannelQueue {
	return &ChannelQueue{
		messageChan: make(chan RequestBody, bufferSize),
		fileChan:    make(chan FileMessage, bufferSize),
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

func (c *ChannelQueue) PublishFile(
	ctx context.Context,
	file multipart.File,
	contentType string,
	fileName string,
) error {
	data, err := io.ReadAll(file)
	failOnError(err, "failed to read file")

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	msg := FileMessage{
		ContentType: contentType,
		Data:        data, // []byte
		FileName:    fileName,
	}

	select {
	case c.fileChan <- msg:
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

func (c *ChannelQueue) StartFileConsumer(handler func(FileMessage) error) {
	go func() {
		for evt := range c.fileChan {
			log.Printf("Received a file: %s", evt.FileName)
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