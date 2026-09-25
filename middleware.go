package bojet

import (
	"strings"

	"gopkg.in/telebot.v4"
)

func (b *Bot) setupMiddleware() {
	b.tb.Use(b.recoverMiddleware, b.serializeMiddleware, b.authMiddleware)
}

// recoverMiddleware keeps a panicking handler from crashing the process.
func (b *Bot) recoverMiddleware(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		defer b.recoverPanic(c)
		return next(c)
	}
}

// serializeMiddleware processes one update at a time per sender, since
// telebot runs each update in its own goroutine and a user's Session is not
// safe for concurrent use.
func (b *Bot) serializeMiddleware(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		if sender := c.Sender(); sender != nil {
			defer b.userLocks.lock(sender.ID)()
		}
		return next(c)
	}
}

// authMiddleware admits admins, the registration entry points (/start and a
// shared contact), and senders the registration flow allows.
func (b *Bot) authMiddleware(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		sender := c.Sender()
		if sender == nil {
			// Updates without a sender (e.g. channel posts) carry no user to
			// authorize; only custom handlers receive them.
			return next(c)
		}
		senderID := sender.ID

		if b.IsAdmin(senderID) {
			return next(c)
		}

		if c.Callback() == nil {
			if msg := c.Message(); msg != nil {
				if isStartCommand(msg.Text) || msg.Contact != nil {
					return next(c)
				}
			}
		}

		allowed, err := b.registration.IsAllowed(senderID, cachedUserStore{UserStore: b.userStore, bot: b})
		if err != nil {
			b.errorHandler(err, c)
			return c.Send(b.messages.NotAuthorized)
		}
		if !allowed {
			b.errorHandler(ErrUserNotApproved.WithParams("user_id", senderID), c)
			if u, err := b.resolveUser(senderID); err == nil && u != nil && u.IsRejected {
				return c.Send(b.messages.Rejected)
			}
			return c.Send(b.messages.NotAuthorized)
		}

		return next(c)
	}
}

// isStartCommand reports whether text is the /start command, including its
// deep-link ("/start <payload>") and group ("/start@botname") forms.
func isStartCommand(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	cmd, _, _ := strings.Cut(fields[0], "@")
	return cmd == "/start"
}

// cachedUserStore serves GetUser from the bot's user cache, so authorizing an
// update does not query the database on every message.
type cachedUserStore struct {
	UserStore
	bot *Bot
}

func (s cachedUserStore) GetUser(id int64) (*User, error) { return s.bot.resolveUser(id) }
