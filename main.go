package main

import (
	"context"
	"log"
	"os"

	tgClient "github.com/svladilsav00-qq/my-sleep-bot/clients/telegram"
	eventconsumer "github.com/svladilsav00-qq/my-sleep-bot/consumer/event_consumer.go"
	"github.com/svladilsav00-qq/my-sleep-bot/events/telegram"
	"github.com/svladilsav00-qq/my-sleep-bot/storage/postgres"
)

const (
	tgBotHost = "api.telegram.org"
	batchSize = 100
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("no db dsn")
	}

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN not set")
	}

	s, err := postgres.New(dbURL)
	if err != nil {
		log.Fatal("can't connect to storage: ", err)
	}

	if err := s.Init(context.TODO()); err != nil {
		log.Fatal("can't init storage: ", err)
	}

	eventsProcessor := telegram.New(
		tgClient.New(tgBotHost, token),
		s,
	)

	log.Printf("service started")

	consumer := eventconsumer.New(eventsProcessor, eventsProcessor, batchSize)

	if err := consumer.Start(); err != nil {
		log.Fatal("service is stopped", err)
	}

}
