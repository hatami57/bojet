package bot

import (
	"log"

	"gopkg.in/telebot.v4"
)

func (b *SonateBot) RegisterMiddlewares() {
	b.TgBot.Use(func(next telebot.HandlerFunc) telebot.HandlerFunc {
		log.Println("In middleware...")
		return func(c telebot.Context) error {
			senderID := c.Sender().ID
			msg := c.Message()

			if b.cfg.IsAdmin(senderID) {
				log.Printf("  middleware: it's admin -> next")
				return next(c)
			}

			if msg != nil {
				if msg.Text == "/start" || msg.Contact != nil {
					log.Printf("  middleware: it's /start or contact -> next")
					return next(c)
				}
			}

			var isConfirmed bool
			err := b.db.QueryRow("SELECT is_confirmed FROM users WHERE tg_id=?", c.Sender().ID).Scan(&isConfirmed)
			if err != nil || !isConfirmed {
				log.Printf("  middleware: user is not confirmed -> stop")
				return c.Send("⛔ You are not authorized to use this bot. Please wait for admin approval.")
			}

			log.Printf("  middleware: user is confirmed -> next")
			return next(c)
		}
	})
}
