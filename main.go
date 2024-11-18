package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v2"
)

func main() {
	// Open the JSON configuration file
	file, err := os.Open("/usr/src/app/data/config.json")
	if err != nil {
		log.Fatalf("Error opening config file: %v", err)
	}
	defer file.Close()

	// Parse the JSON configuration
	var config map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		log.Fatalf("Error decoding JSON: %v", err)
	}

	// Extract and assert bot token
	botToken, ok := config["key"].(string)
	if !ok {
		log.Fatalf("Invalid or missing 'key' in config")
	}

	// Extract and assert chat ID
	chatIDString, ok := config["chatid"].(string)
	if !ok {
		log.Fatalf("Invalid or missing 'chatid' in config")
	}

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

	// Define GET endpoint
	app.Get("/get", func(c *fiber.Ctx) error {
		// Retrieve the message from query parameters
		message := c.Query("message", "No message provided")

		// Send message via Telegram bot
		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = tgbotapi.ModeMarkdown
		_, err := bot.Send(msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to send message")
		}

		// Respond to HTTP request
		return c.SendString("Message sent successfully!")
	})

	// Start the Fiber server
	log.Println("Server is running on http://localhost:8080")
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
