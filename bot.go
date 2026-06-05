package bojet

import (
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/hatami57/microjet/core"
	"github.com/robfig/cron/v3"
	"gopkg.in/telebot.v4"
)

// Bot is the central bot instance. Create one with New().
type Bot struct {
	tb *telebot.Bot

	store        UserStore
	homePage     *Page
	adminIDs     map[int64]struct{}
	proxyURL     string
	pollTimeout  time.Duration
	cacheExpiry  time.Duration
	messages     Messages
	registration RegistrationFlow
	errorHandler func(error, telebot.Context)
	contactAdmin bool
	logger       *slog.Logger
	clock        core.TimeProvider

	hooks hooks

	mu    sync.Mutex
	users map[int64]*User

	cron           *cron.Cron
	publicKeyboard *telebot.ReplyMarkup
}

// New creates a Bot with the given Telegram token and options.
//
//	bot, err := bojet.New(os.Getenv("BOT_TOKEN"),
//	    bojet.WithStore(store),
//	    bojet.WithAdmins(123456789),
//	    bojet.WithHomePage(homePage),
//	)
func New(token string, opts ...Option) (*Bot, error) {
	b := &Bot{
		adminIDs:     map[int64]struct{}{},
		pollTimeout:  10 * time.Second,
		cacheExpiry:  30 * time.Minute,
		messages:     DefaultMessages,
		registration: &PhoneVerificationFlow{},
		users:        map[int64]*User{},
		contactAdmin: true,
		errorHandler: func(err error, _ telebot.Context) {},
		cron:         cron.New(),
		logger:       core.NewLogger(nil),
		clock:        &core.SystemClock{},
	}

	for _, opt := range opts {
		opt(b)
	}

	settings := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: b.pollTimeout},
	}

	if b.proxyURL != "" {
		pu, err := url.Parse(b.proxyURL)
		if err != nil {
			return nil, core.NewBadRequestError("Proxy", "invalid proxy URL").
				WithParams("url", b.proxyURL).WithInner(err)
		}
		settings.Client = &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(pu)},
		}
	}

	tb, err := telebot.NewBot(settings)
	if err != nil {
		return nil, core.NewInternalError("Telegram", "failed to initialize Telegram bot").WithInner(err)
	}
	b.tb = tb
	b.buildPublicKeyboard()

	return b, nil
}

// Start registers all handlers, middleware and schedulers, then begins
// polling Telegram for updates. Blocks until Stop() is called.
func (b *Bot) Start() error {
	if b.store == nil {
		return ErrStoreRequired
	}

	b.setupMiddleware()
	b.setupHandlers()
	b.registration.SetupHandlers(b)
	b.cron.Start()

	b.tb.Start()
	return nil
}

// Stop gracefully shuts down the bot and its scheduler.
func (b *Bot) Stop() {
	b.cron.Stop()
	b.tb.Stop()
}

// Handle registers a handler for the given endpoint. The handler receives an
// enriched Context with BotUser() already resolved. Mirrors telebot.Handle().
func (b *Bot) Handle(endpoint interface{}, h HandlerFunc) {
	b.tb.Handle(endpoint, func(c telebot.Context) error {
		user, err := b.resolveUser(c.Sender().ID)
		if err != nil {
			b.errorHandler(err, c)
			return c.Send(b.messages.GenericError)
		}
		return h(&botCtx{c, user})
	})
}

// IsAdmin reports whether the given Telegram user ID has admin privileges.
func (b *Bot) IsAdmin(userID int64) bool {
	_, ok := b.adminIDs[userID]
	return ok
}

// Broadcast sends a plain-text message to all confirmed users concurrently.
// Delivery errors are routed to the error handler.
func (b *Bot) Broadcast(msg string) error {
	ids, err := b.store.ListConfirmedIDs()
	if err != nil {
		return err
	}
	for _, id := range ids {
		go func(userID int64) {
			if _, err := b.tb.Send(&telebot.User{ID: userID}, msg); err != nil {
				b.errorHandler(err, nil)
			}
		}(id)
	}
	return nil
}

// Schedule registers a recurring job using a standard cron expression.
//
//	bot.Schedule("0 9 * * *", func() { bot.Broadcast("Good morning!") })
func (b *Bot) Schedule(expr string, fn func()) error {
	_, err := b.cron.AddFunc(expr, fn)
	return err
}

// ScheduleBroadcast is a convenience wrapper for scheduling a broadcast message.
//
//	bot.ScheduleBroadcast("0 9 * * *", "🌅 Good morning!")
func (b *Bot) ScheduleBroadcast(expr string, msg string) error {
	return b.Schedule(expr, func() {
		if err := b.Broadcast(msg); err != nil {
			b.errorHandler(err, nil)
		}
	})
}

// resolveUser returns the user from the in-memory cache, or loads from the
// store and seeds the cache. A cached entry older than cacheExpiry is treated
// as stale and reloaded from the store. Returns nil (no error) for unknown users.
func (b *Bot) resolveUser(id int64) (*User, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock.Now()
	if u, ok := b.users[id]; ok && !u.isExpired(now) {
		u.resetExpiration(now, b.cacheExpiry)
		return u, nil
	}

	u, err := b.store.GetUser(id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		delete(b.users, id)
		return nil, nil
	}

	u.CurrentPage = b.homePage
	u.resetExpiration(now, b.cacheExpiry)
	b.users[id] = u
	return u, nil
}

func (b *Bot) cacheUser(u *User) {
	b.mu.Lock()
	defer b.mu.Unlock()
	u.resetExpiration(b.clock.Now(), b.cacheExpiry)
	b.users[u.ID] = u
}

func (b *Bot) buildPublicKeyboard() {
	pk := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}
	pk.Reply(pk.Row(pk.Contact(b.messages.SharePhoneButton)))
	b.publicKeyboard = pk
}
