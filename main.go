package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Load configuration
	config, err := LoadConfig("/usr/src/app/data/config.json")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Create queue based on configuration
	var queue MessageQueue
	if config.RabbitMQ.UseRabbitMQ {
		log.Println("Using RabbitMQ queue")
		queue, err = NewRabbitMQQueue(*config)
		if err != nil {
			log.Fatalf("Failed to create RabbitMQ queue: %v", err)
		}
		defer queue.Close()
	} else {
		log.Println("Using channel queue")
		queue = NewChannelQueue(100)
		defer queue.Close()
	}

	// Create notifier
	notifier, err := NewTelegramNotifier(*config, queue)
	if err != nil {
		log.Fatalf("Failed to create notifier: %v", err)
	}

	// Initialize Fiber app
	app := fiber.New()

	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()

	// Define POST endpoint
	app.Post("/send", func(c *fiber.Ctx) error {
		var body RequestBody

		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON body")
		}

		if body.Message == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Missing message in request body")
		}

		if err := notifier.PublishMessage(c.Context(), body); err != nil {
			log.Printf("Error sending message: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Error sending message.")
		}

		return c.SendString("Message sent successfully!")
	})

	app.Post("/sendfile", func(c *fiber.Ctx) error {
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Error getting file")
		}

		content, err := file.Open()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Error opening file")
		}

		defer content.Close()

		contentType := file.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		if err := notifier.PublishFile(c.Context(), content, contentType, file.Filename); err != nil {
			log.Printf("Error sending file: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Error sending file.")
		}

		return c.SendString("File sent successfully!")
	})

	// Start the Fiber server
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}

	log.Println("Server is running on http://localhost:8080")
}