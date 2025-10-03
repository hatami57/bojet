package bot

import (
	"database/sql"
	"log"
	"slices"
	"sonatebot/internal/config"

	"gopkg.in/telebot.v4"
)

func RegisterMiddlewares(tb *telebot.Bot, db *sql.DB, cfg *config.Config) {
	// Middleware: check if user is confirmed
	tb.Use(func(next telebot.HandlerFunc) telebot.HandlerFunc {
		log.Println("In middleware...")
		return func(c telebot.Context) error {
			if slices.Contains(cfg.AdminIDs, c.Sender().ID) {
				log.Printf("  middleware: it's admin -> next")
				return next(c)
			}

			if c.Message() != nil {
				if c.Message().Text == "/start" || c.Message().Contact != nil {
					log.Printf("  middleware: it's /start or contact -> next")
					return next(c)
				}
			}

			var isConfirmed bool
			err := db.QueryRow("SELECT is_confirmed FROM users WHERE tg_id=?", c.Sender().ID).Scan(&isConfirmed)
			if err != nil || !isConfirmed {
				log.Printf("  middleware: user is not confirmed -> stop")
				return c.Send("⛔ You are not authorized to use this bot. Please wait for admin approval.")
			}

			log.Printf("  middleware: user is confirmed -> next")
			return next(c)
		}
	})
}
