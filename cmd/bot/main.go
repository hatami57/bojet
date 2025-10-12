package main

import (
	"sonatebot/internal/bot"
	"sonatebot/internal/config"
	"sonatebot/internal/db"
)

func main() {
	cfg := config.Load()

	database := db.Init(cfg.DBPath)

	sonateBot := bot.NewSonateBot(cfg, database)

	sonateBot.Start()
}
