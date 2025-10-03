package main

import (
	"log"
	"sonatebot/internal/bot"
	"sonatebot/internal/config"
	"sonatebot/internal/db"

	"gopkg.in/telebot.v4"
)

func main() {
	cfg := config.Load()

	database := db.Init(cfg.DBPath)

	tb, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.Token,
		Poller: &telebot.LongPoller{Timeout: cfg.PollTimeout},
	})
	if err != nil {
		log.Fatal(err)
	}

	bot.RegisterHandlers(tb, database, cfg)

	bot.StartSchedulers(tb, database, cfg)

	log.Println("🚀 Bot started")
	tb.Start()
}
