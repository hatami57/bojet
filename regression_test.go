package bojet

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hatami57/microjet/gormx"
	"github.com/hatami57/microjet/gormx/sqlite"
	"github.com/hatami57/microjet/host"
	"gopkg.in/telebot.v4"
)

const (
	adminID = int64(1)
	userID  = int64(100)
)

// Close must cope with a bot whose Init failed (no Telegram client) and with
// one that was initialized but never started polling.
func TestCloseWithoutStart(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := New().Close(); err != nil {
			t.Errorf("Close after failed init: %v", err)
		}
		b := newTestBot(t, newFakeAPI(t), newMemStore())
		if err := b.Close(); err != nil {
			t.Errorf("Close without polling: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked on a poller that never started")
	}
}

// The store must work whichever order bojet and gormx are registered in.
func TestDBStoreModuleOrderAndRejection(t *testing.T) {
	for _, storeFirst := range []bool{true, false} {
		app, err := host.New(host.WithConfigValue("database.name", ":memory:"))
		if err != nil {
			t.Fatal(err)
		}
		store := NewDBStore()
		if storeFirst {
			host.ProvideService(app, store)
			app.WithModule(gormx.Module(sqlite.Driver()))
		} else {
			app.WithModule(gormx.Module(sqlite.Driver()))
			host.ProvideService(app, store)
		}
		if err := app.Start(context.Background()); err != nil {
			t.Fatalf("storeFirst=%v: start: %v", storeFirst, err)
		}

		if err := store.SaveUser(&User{ID: 7, FirstName: "A"}); err != nil {
			t.Fatal(err)
		}
		if err := store.SetConfirmed(7, false); err != nil {
			t.Fatal(err)
		}
		u, err := store.GetUser(7)
		if err != nil || u == nil || u.IsConfirmed || !u.IsRejected {
			t.Fatalf("after reject: %+v, %v", u, err)
		}
		if err := store.SetConfirmed(7, true); err != nil {
			t.Fatal(err)
		}
		u, _ = store.GetUser(7)
		if !u.IsConfirmed || u.IsRejected {
			t.Fatalf("after approve: %+v", u)
		}
		_ = app.Shutdown(context.Background())
	}
}

// Concurrent updates from one user must not race on their session.
func TestUpdatesSerializedPerUser(t *testing.T) {
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	store := newMemStore(User{ID: userID, IsConfirmed: true})
	b := newWiredBot(t, newFakeAPI(t), store, false,
		WithHandler("/inc", func(c Context) error {
			defer wg.Done()
			v, _ := c.SessionGet("n")
			cnt, _ := v.(int)
			time.Sleep(time.Millisecond) // widen the race window
			c.SessionSet("n", cnt+1)
			return nil
		}))

	for i := range n {
		b.tb.ProcessUpdate(telebot.Update{ID: i + 1, Message: &telebot.Message{
			ID:     i + 1,
			Sender: &telebot.User{ID: userID},
			Chat:   &telebot.Chat{ID: userID, Type: telebot.ChatPrivate},
			Text:   "/inc",
		}})
	}
	wg.Wait()

	u, _ := b.resolveUser(userID)
	if got := u.Session.Data["n"]; got != n {
		t.Fatalf("counter = %v; want %d (lost updates)", got, n)
	}
}

// A panicking handler is reported, not fatal; sender-less updates and forms
// started for unknown users do not crash.
func TestPanicsAndNilUsers(t *testing.T) {
	var errs []error
	api := newFakeAPI(t)
	channelUser := &User{}
	b := newTestBot(t, api, newMemStore(User{ID: userID, IsConfirmed: true}),
		WithErrorHandler(func(err error, _ telebot.Context) { errs = append(errs, err) }),
		WithHandler("/boom", func(Context) error { panic("boom") }),
		WithHandler(telebot.OnChannelPost, func(c Context) error {
			channelUser = c.BotUser()
			return nil
		}))

	text(b, userID, "/boom")
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "panic") {
		t.Fatalf("panic not reported: %v", errs)
	}

	b.tb.ProcessUpdate(telebot.Update{ID: 999, ChannelPost: &telebot.Message{
		ID: 1, Chat: &telebot.Chat{ID: -100, Type: telebot.ChatChannel}, Text: "post",
	}})
	if channelUser != nil {
		t.Fatalf("channel post handler got user %+v; want nil", channelUser)
	}

	f := &Form{Source: StaticSource(&Question{Key: "q", Prompt: "Q?"})}
	if err := b.startForm(nil, nil, f); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("startForm(nil user) = %v; want ErrUserNotFound", err)
	}
}

// Sharing someone else's contact must not register their ID.
func TestContactMustBeSendersOwn(t *testing.T) {
	api := newFakeAPI(t)
	store := newMemStore()
	b := newTestBot(t, api, store, WithAdmins(adminID))

	message(b, userID, &telebot.Message{Contact: &telebot.Contact{UserID: 555, PhoneNumber: "1"}})
	if _, ok := store.get(555); ok {
		t.Fatal("another user's contact was registered")
	}
	if got := api.lastText(userID); got != b.messages.ShareOwnContact {
		t.Fatalf("reply = %q; want ShareOwnContact", got)
	}

	message(b, userID, &telebot.Message{Contact: &telebot.Contact{UserID: userID, PhoneNumber: "2"}})
	if u, ok := store.get(userID); !ok || u.PhoneNumber != "2" {
		t.Fatalf("own contact not registered: %+v", u)
	}
}

// Deep-link and addressed /start reach the welcome prompt for new users.
func TestStartWithPayload(t *testing.T) {
	for _, cmd := range []string{"/start", "/start ref42", "/start@testbot"} {
		if !isStartCommand(cmd) {
			t.Errorf("isStartCommand(%q) = false", cmd)
		}
	}
	for _, cmd := range []string{"", "/started", "hello /start"} {
		if isStartCommand(cmd) {
			t.Errorf("isStartCommand(%q) = true", cmd)
		}
	}

	api := newFakeAPI(t)
	b := newTestBot(t, api, newMemStore())
	text(b, userID, "/start ref42")
	if got := api.lastText(userID); got != b.messages.Welcome {
		t.Fatalf("reply = %q; want Welcome", got)
	}
}

// The reject hook fires even when the user is not cached, and the rejected
// user is told so on /start.
func TestRejectAfterRestart(t *testing.T) {
	api := newFakeAPI(t)
	store := newMemStore(User{ID: userID, FirstName: "Pending"})
	var rejected *User
	b := newTestBot(t, api, store, WithAdmins(adminID),
		WithOnUserRejected(func(u *User) error { rejected = u; return nil }))

	callback(b, adminID, "reject", "100")
	if rejected == nil || rejected.ID != userID {
		t.Fatalf("reject hook got %+v", rejected)
	}

	text(b, userID, "/start")
	if got := api.lastText(userID); got != b.messages.Rejected {
		t.Fatalf("rejected user /start reply = %q; want Rejected", got)
	}
	text(b, userID, "hello")
	if got := api.lastText(userID); got != b.messages.Rejected {
		t.Fatalf("rejected user message reply = %q; want Rejected", got)
	}
}

// /start answers by registration state, and approved users land on the menu.
func TestStartByRegistrationState(t *testing.T) {
	api := newFakeAPI(t)
	home := NewPage("Home", ActionItem("Go", func(c Context) error { return nil }))
	store := newMemStore(User{ID: 10, IsConfirmed: true}, User{ID: 11})
	b := newTestBot(t, api, store, WithAdmins(adminID), WithHomePage(home))

	text(b, 10, "/start")
	if got := api.lastText(10); got != "Home" {
		t.Fatalf("confirmed /start = %q; want Home", got)
	}
	text(b, 11, "/start")
	if got := api.lastText(11); got != b.messages.RegistrationPending {
		t.Fatalf("pending /start = %q; want RegistrationPending", got)
	}
	message(b, 10, &telebot.Message{Contact: &telebot.Contact{UserID: 10}})
	if got := api.lastText(10); got != "Home" {
		t.Fatalf("confirmed contact = %q; want Home", got)
	}
}

// Admins can use the menus, and their replies reach users both through
// forwards Telegram attributes and through the bot's relay log.
func TestAdminMenuAndReplies(t *testing.T) {
	api := newFakeAPI(t)
	ran := false
	home := NewPage("Home", ActionItem("Go", func(c Context) error { ran = true; return nil }))
	store := newMemStore(User{ID: userID, IsConfirmed: true})
	b := newTestBot(t, api, store, WithAdmins(adminID), WithHomePage(home))

	text(b, adminID, "Go")
	if !ran {
		t.Fatalf("admin menu item did not run; reply = %q", api.lastText(adminID))
	}

	// User contacts the admin; the forward is logged for replies.
	text(b, userID, b.messages.ContactAdminButton)
	text(b, userID, "help me")
	fwds := api.sent("forwardMessage")
	if len(fwds) != 1 || fwds[0].chatID() != adminID {
		t.Fatalf("contact-admin forwards = %+v", fwds)
	}
	var fwdID int
	b.relays.mu.Lock()
	for k := range b.relays.users {
		fwdID = k.msgID
	}
	b.relays.mu.Unlock()

	// Reply to the relayed message (Telegram hides the original sender).
	message(b, adminID, &telebot.Message{Text: "answer", ReplyTo: &telebot.Message{
		ID: fwdID, Chat: &telebot.Chat{ID: adminID},
	}})
	// Reply to a forward attributed through forward_origin.
	message(b, adminID, &telebot.Message{Text: "answer2", ReplyTo: &telebot.Message{
		ID: 1, Chat: &telebot.Chat{ID: adminID},
		Origin: &telebot.MessageOrigin{Type: "user", Sender: &telebot.User{ID: 200}},
	}})
	fwds = api.sent("forwardMessage")
	if len(fwds) != 3 || fwds[1].chatID() != userID || fwds[2].chatID() != 200 {
		t.Fatalf("admin replies forwarded to %+v; want users %d and 200", fwds, userID)
	}
}

// Broadcasts are paced and retried after 429.
func TestBroadcastPacedWithRetry(t *testing.T) {
	api := newFakeAPI(t)
	var once sync.Once
	api.respond = func(c apiCall) string {
		if c.method == "sendMessage" && c.chatID() == 2 {
			body := ""
			once.Do(func() {
				body = `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 0","parameters":{"retry_after":0}}`
			})
			return body
		}
		return ""
	}
	store := newMemStore(User{ID: 1, IsConfirmed: true}, User{ID: 2, IsConfirmed: true}, User{ID: 3, IsConfirmed: true})
	b := newTestBot(t, api, store)

	start := time.Now()
	if err := b.Broadcast("hi"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "broadcast", func() bool { return len(api.sent("sendMessage")) == 4 })
	if elapsed := time.Since(start); elapsed < 2*broadcastInterval {
		t.Fatalf("3 messages sent in %v; want paced at %v", elapsed, broadcastInterval)
	}
	got := map[int64]int{}
	for _, c := range api.sent("sendMessage") {
		got[c.chatID()]++
	}
	if got[1] != 1 || got[2] != 2 || got[3] != 1 {
		t.Fatalf("deliveries per user = %v; want user 2 retried once", got)
	}
}

// Without a custom handler, errors are logged rather than dropped.
func TestDefaultErrorHandlerLogs(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	New().errorHandler(errors.New("disk on fire"), nil)
	if !strings.Contains(buf.String(), "disk on fire") {
		t.Fatalf("error not logged: %q", buf.String())
	}
}

// Expired users are swept from the cache.
func TestCacheSweepsExpiredUsers(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0).UTC()}
	b := &Bot{users: map[int64]*User{}, clock: clock, config: Config{CacheExpiry: time.Minute}}
	b.cacheUser(&User{ID: 1})
	clock.now = clock.now.Add(2 * time.Minute)
	b.cacheUser(&User{ID: 2})
	if _, ok := b.users[1]; ok {
		t.Fatal("expired user 1 was not swept")
	}
	if _, ok := b.users[2]; !ok {
		t.Fatal("fresh user 2 was evicted")
	}
}

// lockProbeStore reports whether the bot's cache lock was held during GetUser.
type lockProbeStore struct {
	memStore
	b                *Bot
	lockedDuringLoad bool
}

func (s *lockProbeStore) GetUser(id int64) (*User, error) {
	if s.b.mu.TryLock() {
		s.b.mu.Unlock()
	} else {
		s.lockedDuringLoad = true
	}
	return s.memStore.GetUser(id)
}

// The cache lock is not held across store queries, and authorization uses the
// cache instead of querying the store for every update.
func TestUserLookupUsesCacheWithoutGlobalLock(t *testing.T) {
	store := &lockProbeStore{memStore: *newMemStore(User{ID: userID, IsConfirmed: true})}
	b := newTestBot(t, newFakeAPI(t), store)
	store.b = b

	for range 3 {
		text(b, userID, "hello")
	}
	if store.lockedDuringLoad {
		t.Fatal("cache lock held during store query")
	}
	if store.loads != 1 {
		t.Fatalf("store loaded %d times for 3 messages; want 1", store.loads)
	}
}

// A message that reached some admins is not resent; with no admins the
// contact-admin button is hidden.
func TestContactAdminPartialFailureAndNoAdmins(t *testing.T) {
	api := newFakeAPI(t)
	api.respond = func(c apiCall) string {
		if c.method == "forwardMessage" && c.chatID() == 2 {
			return `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`
		}
		return ""
	}
	b := newTestBot(t, api, newMemStore(User{ID: userID, IsConfirmed: true}),
		WithAdmins(1, 2), WithErrorHandler(func(error, telebot.Context) {}))

	text(b, userID, b.messages.ContactAdminButton)
	text(b, userID, "hello")
	if got := api.lastText(userID); got != b.messages.MessageSent {
		t.Fatalf("partial delivery reply = %q; want MessageSent", got)
	}
	u, _ := b.resolveUser(userID)
	if u.Session.input != nil {
		t.Fatal("contact-admin prompt still active after delivery")
	}

	noAdmins := newTestBot(t, api, newMemStore())
	if noAdmins.contactAdminEnabled(&User{ID: userID}) {
		t.Fatal("contact-admin enabled with no admins")
	}
	if !b.contactAdminEnabled(&User{ID: userID}) || b.contactAdminEnabled(&User{ID: 1}) {
		t.Fatal("contact-admin should be on for users and off for admins")
	}
}

// Back on the first question re-asks it instead of storing "🔙 Back".
func TestFormBackWithoutHistory(t *testing.T) {
	api := newFakeAPI(t)
	var got Answers
	form := &Form{
		Source:     StaticSource(&Question{Key: "name", Prompt: "Name?"}),
		OnComplete: func(_ Context, a Answers) error { got = a; return nil },
	}
	var errs []error
	b := newTestBot(t, api, newMemStore(User{ID: userID, IsConfirmed: true}),
		WithHomePage(NewPage("Home", FormItem("Form", form))),
		WithErrorHandler(func(err error, _ telebot.Context) { errs = append(errs, err) }))

	text(b, userID, "Form")
	text(b, userID, PageBackText)
	if got != nil {
		t.Fatalf("Back was taken as the answer: %v", got)
	}
	if len(errs) != 0 {
		t.Fatalf("Back on the first question failed: %v", errs)
	}
	asked := 0
	for _, m := range api.sent("sendMessage") {
		if m.text() == "Name?" {
			asked++
		}
	}
	if asked != 2 {
		t.Fatalf("question asked %d times; want it re-asked after Back", asked)
	}
	text(b, userID, "Ada")
	if got["name"] != "Ada" {
		t.Fatalf("answers = %v", got)
	}
}
