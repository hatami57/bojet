package bojet

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/hatami57/microjet/core"
	"github.com/hatami57/microjet/host"
	"github.com/robfig/cron/v3"
	"gopkg.in/telebot.v4"
)

// Bot is the central bot instance. Create one with New().
type Bot struct {
	tb *telebot.Bot

	app          *host.App
	config       Config
	adminIDs     map[int64]struct{}
	userStore    UserStore
	sessions     SessionStore
	homePage     *Page
	messages     Messages
	registration RegistrationFlow
	errorHandler func(error, telebot.Context)

	hooks hooks
	opts  []Option
	clock core.TimeProvider

	// pendingHandlers and pendingSchedules hold custom handlers and cron jobs
	// registered via options before the bot starts. They are applied in Start,
	// once the Telegram client and scheduler exist.
	pendingHandlers  []pendingHandler
	pendingSchedules []pendingSchedule

	mu    sync.Mutex
	users map[int64]*User

	cron           *cron.Cron
	publicKeyboard *telebot.ReplyMarkup
}

// New creates a Bot from the given options. It is normally not called directly;
// use Module so the host wires the bot, its user store, and the database
// together. The Telegram token and other settings come from the [bot] config
// section (see Config), with options taking precedence.
//
//	bojet.Module(
//	    bojet.WithHomePage(homePage),
//	    bojet.WithAdmins(123456789),
//	)
func New(opts ...Option) *Bot {
	return &Bot{
		messages:     DefaultMessages,
		registration: &PhoneVerificationFlow{},
		sessions:     NewMemorySessionStore(),
		users:        map[int64]*User{},
		errorHandler: func(err error, _ telebot.Context) {},
		cron:         cron.New(),
		clock:        core.UTC,
		adminIDs:     map[int64]struct{}{},
		opts:         opts,
	}
}

func (b *Bot) ReadConfig(reader core.ConfigReader) error {
	reader.SetDefault("bot.pollTimeout", "10s")
	reader.SetDefault("bot.cacheExpiry", "30m")
	reader.SetDefault("bot.contactAdmin", true)
	return reader.Read("bot", &b.config)
}

func (b *Bot) Init(app *host.App) error {
	b.applyOptions()

	for _, id := range b.config.AdminIDs {
		b.adminIDs[id] = struct{}{}
	}

	settings, err := b.createSettings()
	if err != nil {
		return err
	}

	tb, err := telebot.NewBot(*settings)
	if err != nil {
		return core.NewInternalError("Telegram", "failed to initialize Telegram bot").WithInner(err)
	}

	b.app = app
	b.clock = app.Clock
	b.userStore = host.MustResolveService[UserStore](app)
	b.tb = tb
	b.buildPublicKeyboard()

	return nil
}

func (b *Bot) applyOptions() {
	for _, opt := range b.opts {
		opt(b)
	}
}

func (b *Bot) createSettings() (*telebot.Settings, error) {
	var client *http.Client

	if b.config.ProxyURL != "" {
		url, err := url.Parse(b.config.ProxyURL)
		if err != nil {
			return nil, core.NewBadRequestError("Proxy", "invalid proxy URL").
				WithParams("url", b.config.ProxyURL).
				WithInner(err)
		}
		client = &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(url)},
		}
	}

	return &telebot.Settings{
		Token:  b.config.Token,
		Poller: &telebot.LongPoller{Timeout: b.config.PollTimeout},
		Client: client,
	}, nil
}

// Start registers all handlers, middleware and schedulers, then begins
// polling Telegram for updates. Run in a go routine.
func (b *Bot) Start(app *host.App) error {
	if b.userStore == nil {
		return ErrStoreRequired
	}

	b.app.Logger.Info("starting telegram bot")

	b.setupMiddleware()
	b.setupHandlers()
	b.registration.SetupHandlers(b)

	for _, ph := range b.pendingHandlers {
		b.Handle(ph.endpoint, ph.handler)
	}
	for _, ps := range b.pendingSchedules {
		if _, err := b.cron.AddFunc(ps.expr, ps.fn); err != nil {
			return err
		}
	}
	b.cron.Start()

	go func() {
		b.tb.Start()
	}()

	return nil
}

// pendingHandler is a custom Telegram handler queued via WithHandler.
type pendingHandler struct {
	endpoint any
	handler  HandlerFunc
}

// pendingSchedule is a cron job queued via WithSchedule/WithScheduledBroadcast.
type pendingSchedule struct {
	expr string
	fn   func()
}

// Close implements core.Closer, gracefully stopping the bot.
func (b *Bot) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	b.stop(ctx)

	return nil
}

// Stop gracefully shuts down the bot and its scheduler, honoring ctx. If ctx is
// cancelled or its deadline elapses before shutdown completes, stop returns
// early and leaves any in-flight work to be reaped by the process exit.
func (b *Bot) stop(ctx context.Context) {
	// cron.Stop() returns a context that is done once all running jobs finish.
	cronDone := b.cron.Stop().Done()

	// telebot.Stop() blocks until polling halts, so run it off the calling
	// goroutine to keep it cancellable via ctx.
	tbDone := make(chan struct{})
	go func() {
		b.tb.Stop()
		close(tbDone)
	}()

	for cronDone != nil || tbDone != nil {
		select {
		case <-ctx.Done():
			b.app.Logger.Warn("bot shutdown timed out", "err", ctx.Err())
			return
		case <-cronDone:
			cronDone = nil
		case <-tbDone:
			tbDone = nil
		}
	}
}

// Handle registers a handler for the given endpoint. The handler receives an
// enriched Context with BotUser() already resolved. Mirrors telebot.Handle().
func (b *Bot) Handle(endpoint any, h HandlerFunc) {
	b.tb.Handle(endpoint, func(c telebot.Context) error {
		user, err := b.resolveUser(c.Sender().ID)
		if err != nil {
			b.errorHandler(err, c)
			return c.Send(b.messages.GenericError)
		}
		return h(&botCtx{Context: c, bot: b, user: user})
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
	ids, err := b.userStore.ListConfirmedIDs()
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
		u.resetExpiration(now, b.config.CacheExpiry)
		return u, nil
	}

	u, err := b.userStore.GetUser(id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		delete(b.users, id)
		return nil, nil
	}

	u.Session = b.loadOrNewSession(id)
	u.resetExpiration(now, b.config.CacheExpiry)
	b.users[id] = u
	return u, nil
}

// loadOrNewSession returns the persisted session for the user (e.g. an
// in-progress form whose user-cache entry expired), or a fresh session on the
// home page when none is stored.
func (b *Bot) loadOrNewSession(id int64) *Session {
	if b.sessions != nil {
		if s, err := b.sessions.LoadSession(id); err != nil {
			b.errorHandler(err, nil)
		} else if s != nil {
			return s
		}
	}
	return newSession(b.homePage)
}

// saveSession persists the user's current session, if a store is configured.
func (b *Bot) saveSession(u *User) {
	if b.sessions == nil || u == nil || u.Session == nil {
		return
	}
	if err := b.sessions.SaveSession(u.ID, u.Session); err != nil {
		b.errorHandler(err, nil)
	}
}

// deleteSession removes any persisted session for the user.
func (b *Bot) deleteSession(id int64) {
	if b.sessions == nil {
		return
	}
	if err := b.sessions.DeleteSession(id); err != nil {
		b.errorHandler(err, nil)
	}
}

func (b *Bot) cacheUser(u *User) {
	b.mu.Lock()
	defer b.mu.Unlock()
	u.resetExpiration(b.clock.Now(), b.config.CacheExpiry)
	b.users[u.ID] = u
}

func (b *Bot) buildPublicKeyboard() {
	pk := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}
	pk.Reply(pk.Row(pk.Contact(b.messages.SharePhoneButton)))
	b.publicKeyboard = pk
}
