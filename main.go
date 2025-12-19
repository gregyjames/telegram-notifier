package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

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

	conn, err := amqp.Dial(address)
	defer conn.Close()

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
		// Define a struct to receive JSON body
		var body struct {
			Message string `json:"message"`
		}

		// Parse JSON body
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON body")
		}

		// Validate message
		if body.Message == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Missing message in request body")
		}

		// Send message via Telegram bot
		escapedText := tgbotapi.EscapeText(tgbotapi.ModeMarkdown, body.Message)
		msg := tgbotapi.NewMessage(chatID, escapedText)
		msg.ParseMode = tgbotapi.ModeMarkdown
		
		_, err := bot.Send(msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to send message")
		}

		return c.SendString("Message sent successfully!")
	})

	// Start the Fiber server
	log.Println("Server is running on http://localhost:8080")
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
