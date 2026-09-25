package bojet

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hatami57/microjet/core"
	"github.com/hatami57/microjet/core/configx"
	"github.com/hatami57/microjet/core/errorx"
	"github.com/hatami57/microjet/host"
	"github.com/robfig/cron/v3"
	"gopkg.in/telebot.v4"
)

// broadcastInterval spaces broadcast messages to stay under Telegram's global
// limit of about 30 messages per second.
const broadcastInterval = 40 * time.Millisecond

// broadcastMaxRetries bounds how often one broadcast message is retried after
// Telegram answers 429 Too Many Requests.
const broadcastMaxRetries = 3

// minCacheSweepInterval is the shortest interval between sweeps of expired
// entries from the user cache.
const minCacheSweepInterval = time.Minute

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
	// registered via options (or Handle) before the bot starts. They are
	// applied in Start, once the middleware is installed, so every handler runs
	// behind it.
	pendingHandlers  []pendingHandler
	pendingSchedules []pendingSchedule

	// wired is set once the middleware and built-in handlers are registered;
	// Handle calls made before that are queued in pendingHandlers.
	wired bool
	// polling reports whether the Telegram poller was launched, so shutdown
	// only waits on a poller that is actually running.
	polling atomic.Bool
	// done is closed on shutdown to abort in-flight broadcasts.
	done     chan struct{}
	doneOnce sync.Once

	mu        sync.Mutex
	users     map[int64]*User
	lastSweep time.Time

	// userLocks serializes update handling per sender.
	userLocks keyedMutex
	// relays maps contact-admin messages forwarded to admins back to the user
	// who sent them, so admin replies can be routed.
	relays relayLog
	// broadcastMu serializes broadcasts so concurrent ones share one rate budget.
	broadcastMu sync.Mutex

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
	b := &Bot{
		messages:     DefaultMessages,
		registration: &PhoneVerificationFlow{},
		sessions:     NewMemorySessionStore(),
		users:        map[int64]*User{},
		clock:        core.UTC,
		adminIDs:     map[int64]struct{}{},
		opts:         opts,
		done:         make(chan struct{}),
	}
	b.errorHandler = b.logError
	b.cron = cron.New(cron.WithChain(b.recoverJob))
	return b
}

func (b *Bot) ReadConfig(reader configx.Reader) error {
	reader.SetDefault("bot.pollTimeout", "10s")
	reader.SetDefault("bot.cacheExpiry", "30m")
	reader.SetDefault("bot.contactAdmin", true)
	return reader.Read("bot", &b.config)
}

func (b *Bot) Init(app *host.App) error {
	b.app = app
	b.clock = app.Clock
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
		return errorx.NewInternalError("Telegram", "failed to initialize Telegram bot").WithInner(err)
	}

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
			return nil, errorx.NewBadRequestError("Proxy", "invalid proxy URL").
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
// polling Telegram for updates in the background.
func (b *Bot) Start(app *host.App) error {
	if b.userStore == nil {
		return ErrStoreRequired
	}

	b.logger().Info("starting telegram bot")

	if err := b.wire(); err != nil {
		return err
	}
	b.cron.Start()

	b.polling.Store(true)
	go b.tb.Start()

	return nil
}

// wire installs the middleware, the built-in and queued handlers, and the
// queued cron jobs on the Telegram client. The middleware must come first:
// telebot binds it to a handler when the handler is registered.
func (b *Bot) wire() error {
	b.setupMiddleware()
	b.setupHandlers()
	b.registration.SetupHandlers(b)
	b.wired = true

	for _, ph := range b.pendingHandlers {
		b.Handle(ph.endpoint, ph.handler)
	}
	b.pendingHandlers = nil
	for _, ps := range b.pendingSchedules {
		if _, err := b.cron.AddFunc(ps.expr, ps.fn); err != nil {
			return errorx.NewBadRequestError("Schedule", "invalid cron expression").
				WithParams("expr", ps.expr).
				WithInner(err)
		}
	}
	b.pendingSchedules = nil
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

// Close implements core.Closer, gracefully stopping the bot. The host calls it
// even when startup failed part-way, so it copes with a bot that was never
// initialized or never started polling.
func (b *Bot) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	b.stop(ctx)

	return nil
}

// stop gracefully shuts down the bot and its scheduler, honoring ctx. If ctx is
// cancelled or its deadline elapses before shutdown completes, stop returns
// early and leaves any in-flight work to be reaped by the process exit.
func (b *Bot) stop(ctx context.Context) {
	if b.done != nil {
		b.doneOnce.Do(func() { close(b.done) })
	}

	// cron.Stop() returns a context that is done once all running jobs finish.
	cronDone := b.cron.Stop().Done()

	// telebot.Stop() blocks until polling halts — and forever if polling never
	// started — so only call it for a running poller, and run it off the
	// calling goroutine to keep it cancellable via ctx.
	var tbDone chan struct{}
	if b.tb != nil && b.polling.Load() {
		tbDone = make(chan struct{})
		go func() {
			b.tb.Stop()
			close(tbDone)
		}()
	}

	for cronDone != nil || tbDone != nil {
		select {
		case <-ctx.Done():
			b.logger().Warn("bot shutdown timed out", "err", ctx.Err())
			return
		case <-cronDone:
			cronDone = nil
		case <-tbDone:
			tbDone = nil
		}
	}
}

// Handle registers a handler for the given endpoint. The handler receives an
// enriched Context with BotUser() already resolved — nil when the sender is
// not a registered user (on a public bot, senders are provisioned first).
// Mirrors telebot.Handle(). Handlers registered before the bot starts are
// queued and installed in Start, behind the bot's middleware.
func (b *Bot) Handle(endpoint any, h HandlerFunc) {
	if !b.wired {
		b.pendingHandlers = append(b.pendingHandlers, pendingHandler{endpoint: endpoint, handler: h})
		return
	}
	b.tb.Handle(endpoint, func(c telebot.Context) error {
		var user *User
		if sender := c.Sender(); sender != nil {
			var err error
			user, err = b.userFor(sender)
			if err != nil {
				b.errorHandler(err, c)
				return c.Send(b.messages.GenericError)
			}
		}
		return h(&botCtx{Context: c, bot: b, user: user})
	})
}

// IsAdmin reports whether the given Telegram user ID has admin privileges.
func (b *Bot) IsAdmin(userID int64) bool {
	_, ok := b.adminIDs[userID]
	return ok
}

// Broadcast sends a plain-text message to all confirmed users. Delivery runs in
// the background, paced to respect Telegram's rate limits and retried when
// Telegram asks the bot to slow down; it is abandoned on shutdown. Delivery
// errors are routed to the error handler.
func (b *Bot) Broadcast(msg string) error {
	ids, err := b.userStore.ListConfirmedIDs()
	if err != nil {
		return err
	}
	go b.deliver(ids, msg)
	return nil
}

// deliver sends msg to each user in turn, one every broadcastInterval.
func (b *Bot) deliver(ids []int64, msg string) {
	defer b.recoverPanic(nil)

	b.broadcastMu.Lock()
	defer b.broadcastMu.Unlock()

	tick := time.NewTicker(broadcastInterval)
	defer tick.Stop()

	for i, id := range ids {
		if i > 0 {
			select {
			case <-tick.C:
			case <-b.done:
				return
			}
		}
		if !b.sendWithRetry(id, msg) {
			return
		}
	}
}

// sendWithRetry sends msg to one user, waiting out 429 responses. It returns
// false if the bot is shutting down.
func (b *Bot) sendWithRetry(userID int64, msg string) bool {
	for attempt := 0; ; attempt++ {
		_, err := b.tb.Send(&telebot.User{ID: userID}, msg)
		var flood telebot.FloodError
		if errors.As(err, &flood) && attempt < broadcastMaxRetries {
			select {
			case <-time.After(time.Duration(flood.RetryAfter) * time.Second):
				continue
			case <-b.done:
				return false
			}
		}
		if err != nil {
			b.errorHandler(errorx.NewInternalError("Broadcast", "failed to deliver broadcast").
				WithParams("user_id", userID).
				WithInner(err), nil)
		}
		return true
	}
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
// The cache lock is not held while the store is queried.
func (b *Bot) resolveUser(id int64) (*User, error) {
	now := b.clock.Now()

	b.mu.Lock()
	if u, ok := b.users[id]; ok && !u.isExpired(now) {
		u.resetExpiration(now, b.config.CacheExpiry)
		b.mu.Unlock()
		return u, nil
	}
	b.mu.Unlock()

	u, err := b.userStore.GetUser(id)
	if err != nil {
		return nil, err
	}
	var session *Session
	if u != nil {
		session = b.loadOrNewSession(id)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Another goroutine may have cached the user while the store was queried.
	if cur, ok := b.users[id]; ok && !cur.isExpired(now) {
		return cur, nil
	}
	if u == nil {
		delete(b.users, id)
		return nil, nil
	}
	u.Session = session
	b.cacheLocked(u, now)
	return u, nil
}

// userFor resolves the User behind a Telegram sender. Unknown senders are
// provisioned when the registration flow supports it (public bots), and admins
// who are not registered users get an in-memory user so they can use the menus.
// Returns nil (no error) for an unknown sender the bot does not admit.
func (b *Bot) userFor(sender *telebot.User) (*User, error) {
	user, err := b.resolveUser(sender.ID)
	if err != nil || user != nil {
		return user, err
	}
	if prov, ok := b.registration.(UserProvisioner); ok {
		user, err = b.provision(prov, sender)
		if err != nil || user != nil {
			return user, err
		}
	}
	if b.IsAdmin(sender.ID) {
		user = &User{
			ID:          sender.ID,
			FirstName:   sender.FirstName,
			LastName:    sender.LastName,
			Username:    sender.Username,
			IsConfirmed: true,
			Session:     b.loadOrNewSession(sender.ID),
		}
		b.cacheUser(user)
		return user, nil
	}
	return nil, nil
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
	b.cacheLocked(u, b.clock.Now())
}

// cacheLocked stores u in the cache and periodically evicts expired entries so
// the cache does not grow with every user ever seen. b.mu must be held.
func (b *Bot) cacheLocked(u *User, now time.Time) {
	u.resetExpiration(now, b.config.CacheExpiry)
	b.users[u.ID] = u

	interval := max(b.config.CacheExpiry, minCacheSweepInterval)
	if now.Sub(b.lastSweep) < interval {
		return
	}
	b.lastSweep = now
	for id, cached := range b.users {
		if cached.isExpired(now) {
			delete(b.users, id)
		}
	}
}

// updateCachedUser applies fn to a copy of the cached user and swaps the copy
// in, so handlers already holding the old *User never observe a concurrent
// write. The Session is shared between the two.
func (b *Bot) updateCachedUser(id int64, fn func(*User)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if u, ok := b.users[id]; ok {
		cp := *u
		fn(&cp)
		b.users[id] = &cp
	}
}

func (b *Bot) buildPublicKeyboard() {
	pk := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}
	pk.Reply(pk.Row(pk.Contact(b.messages.SharePhoneButton)))
	b.publicKeyboard = pk
}

// logger returns the host's logger, or the default one before Init.
func (b *Bot) logger() *slog.Logger {
	if b.app != nil && b.app.Logger != nil {
		return b.app.Logger
	}
	return slog.Default()
}

// logError is the default error handler: client-caused errors (rejected
// users, bad input) are logged at Warn, everything else at Error.
func (b *Bot) logError(err error, c telebot.Context) {
	args := []any{"error", err}
	if c != nil && c.Sender() != nil {
		args = append(args, "user_id", c.Sender().ID)
	}
	if t, ok := errorx.GetErrorType(err); ok && t != errorx.InternalErrorType {
		b.logger().Warn("bot request rejected", args...)
		return
	}
	b.logger().Error("bot error", args...)
}

// recoverPanic turns a panic in a handler or background task into an error for
// the error handler instead of crashing the process: telebot runs handlers in
// their own goroutines without recovering. Use it deferred.
func (b *Bot) recoverPanic(c telebot.Context) {
	if r := recover(); r != nil {
		b.errorHandler(errorx.NewInternalError("Bot", "recovered from panic").
			WithParams("panic", fmt.Sprint(r), "stack", string(debug.Stack())), c)
	}
}

// recoverJob wraps scheduled jobs with recoverPanic.
func (b *Bot) recoverJob(j cron.Job) cron.Job {
	return cron.FuncJob(func() {
		defer b.recoverPanic(nil)
		j.Run()
	})
}
