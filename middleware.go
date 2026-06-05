package bojet

import "gopkg.in/telebot.v4"

func (b *Bot) setupMiddleware() {
	b.tb.Use(func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			senderID := c.Sender().ID

			if b.IsAdmin(senderID) {
				return next(c)
			}

			if msg := c.Message(); msg != nil {
				if msg.Text == "/start" || msg.Contact != nil {
					return next(c)
				}
			}

			allowed, err := b.registration.IsAllowed(senderID, b.store)
			if err != nil {
				b.errorHandler(err, c)
				return c.Send(b.messages.NotAuthorized)
			}
			if !allowed {
				b.errorHandler(ErrUserNotApproved.WithParams("user_id", senderID), c)
				return c.Send(b.messages.NotAuthorized)
			}

			return next(c)
		}
	})
}
