package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v2"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Configuration struct {
	Key      string `json:"key"`
	Chatid   string `json:"chatid"`
	RabbitMQ struct {
		Host     string    `json:"Host"`
		Port        int    `json:"Port"`
		Username    string `json:"Username"`
		Password    string `json:"Password"`
		UseRabbitMQ bool   `json:"UseRabbitMQ"`
	} `json:"RabbitMQ"`
}

type RequestBody struct {
	Message string `json:"message"`
}

func failOnError(err error, msg string) {
  if err != nil {
    log.Panicf("%s: %s", msg, err)
  }
}

func publishToChannel(queue *amqp.Queue, ch *amqp.Channel, ctx context.Context, message RequestBody) error {
	body, err := json.Marshal(message)
	failOnError(err, "Failed to parse message to channel.")

	err = ch.PublishWithContext(ctx,
  		"",     // exchange
  		queue.Name, // routing key
  		false,  // mandatory
  		false,  // immediate
  		amqp.Publishing {
    		ContentType: "application/json",
    		Body:        body,
			DeliveryMode: amqp.Persistent,
			Timestamp: time.Now(),
  		},
	)

	failOnError(err, "Failed to publish a message")
	return err
}

func sendToTelegram(bot *tgbotapi.BotAPI, chatID int64, body RequestBody) error{
	// Send message via Telegram bot
	escapedText := tgbotapi.EscapeText(tgbotapi.ModeMarkdown, body.Message)
	msg := tgbotapi.NewMessage(chatID, escapedText)
	msg.ParseMode = tgbotapi.ModeMarkdown
		
	_, err := bot.Send(msg)

	return err
}

func main() {
	// Open the JSON configuration file
	file, err := os.Open("/usr/src/app/data/config.json")
	if err != nil {
		log.Fatalf("Error opening config file: %v", err)
	}
	defer file.Close()

	// Parse the JSON configuration
	var config Configuration
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
    	log.Fatalf("Error decoding config file: %v", err)
	}

	address := fmt.Sprintf("amqp://%s:%s@%s:%d/", config.RabbitMQ.Username, config.RabbitMQ.Password, config.RabbitMQ.Host, config.RabbitMQ.Port);

	conn, err := amqp.Dial(address);
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	queue, err := ch.QueueDeclare(
  		"telegram-notifier", // name
 		 true,   // durable
 		 true,   // delete when unused
 		 false,   // exclusive
 		 false,   // no-wait
 		 nil,     // arguments
	)
	failOnError(err, "Failed to declare a queue")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Extract and assert bot token
	botToken := config.Key
	chatIDString := config.Chatid

	// Convert chat ID to int64
	chatID, err := strconv.ParseInt(chatIDString, 10, 64)
	if err != nil {
		log.Fatalf("Error converting 'chatid' to int64: %v", err)
	}

	// Initialize Telegram Bot
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Error initializing Telegram bot: %v", err)
	}

	// Initialize Fiber app
	app := fiber.New()

	// Define POST endpoint
	app.Post("/send", func(c *fiber.Ctx) error {
		var body RequestBody

		// Parse JSON body
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON body")
		}

		// Validate message
		if body.Message == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Missing message in request body")
		}


		err = publishToChannel(&queue, ch, ctx, body)

		if err != nil {
			log.Printf("Error sending message: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Error sending message to RabbitMQ.")
		}

		return c.SendString("Message sent successfully!")
	})

	msgs, err := ch.Consume(
  		queue.Name, // queue
  		"",     // consumer
  		true,   // auto-ack
  		false,  // exclusive
  		false,  // no-local
  		false,  // no-wait
  		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
  		for d := range msgs {
    		log.Printf("Received a message: %s", d.Body)
			
			var evt RequestBody
			err := json.Unmarshal(d.Body, &evt)
			failOnError(err, "Error parsing json from RabbitMQ.")

			err = sendToTelegram(bot, chatID, evt)
  		}
	}()

	// Start the Fiber server
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}

	log.Println("Server is running on http://localhost:8080")
}
