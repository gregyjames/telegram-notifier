package main

import (
	"context"
	"mime/multipart"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramNotifier struct {
	bot    *tgbotapi.BotAPI
	chatID int64
	queue  MessageQueue
}

func NewTelegramNotifier(config Configuration, queue MessageQueue) (*TelegramNotifier, error) {
	chatID, err := strconv.ParseInt(config.Chatid, 10, 64)
	if err != nil {
		return nil, err
	}

	bot, err := tgbotapi.NewBotAPI(config.Key)
	if err != nil {
		return nil, err
	}

	notifier := &TelegramNotifier{
		bot:    bot,
		chatID: chatID,
		queue:  queue,
	}

	// Start consuming messages
	queue.StartConsumer(notifier.SendToTelegram)
	queue.StartFileConsumer(notifier.SendFileToTelegram)

	return notifier, nil
}

func (t *TelegramNotifier) SendToTelegram(message RequestBody) error {
	escapedText := tgbotapi.EscapeText(tgbotapi.ModeMarkdown, message.Message)
	msg := tgbotapi.NewMessage(t.chatID, escapedText)
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, err := t.bot.Send(msg)
	return err
}

func (t *TelegramNotifier) SendFileToTelegram(message FileMessage) error {
	b := tgbotapi.FileBytes{
		Name: message.FileName,
		Bytes: message.Data,
	}

	msg := tgbotapi.NewDocument(t.chatID, b)
	msg.Caption = "Test"
	_, err := t.bot.Send(msg)
	return err
}

func (t *TelegramNotifier) PublishMessage(ctx context.Context, message RequestBody) error {
	return t.queue.Publish(ctx, message)
}

func (t *TelegramNotifier) PublishFile(ctx context.Context, file multipart.File, contentType string, fileName string) error {
	return t.queue.PublishFile(ctx, file, contentType, fileName)
}

func (t *TelegramNotifier) Close() error {
	return t.queue.Close()
}