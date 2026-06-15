package bojet

import (
	"github.com/hatami57/microjet/host"
	"gopkg.in/telebot.v4"
)

// HandlerFunc is the signature for bot handlers. The Context is an enriched
// version of telebot.Context that also carries the resolved User.
type HandlerFunc func(c Context) error

// Context extends telebot.Context with a BotUser() accessor so handlers
// don't have to call the store themselves, plus session helpers and the
// ability to start a questionnaire form.
type Context interface {
	telebot.Context

	// BotUser returns the resolved User for the message sender.
	// Returns nil if the sender is not registered.
	BotUser() *User

	// App returns the main App instance
	App() *host.App

	// SessionGet returns a value previously stored on the user's session via
	// SessionSet. The second return value reports whether the key was present.
	SessionGet(key string) (any, bool)

	// SessionSet stores a value on the user's session scratch space. Unlike
	// telebot's per-update Get/Set, this persists for the life of the session.
	SessionSet(key string, val any)

	// SessionData returns the session scratch map directly, or nil if there is
	// no session. Mutations to the returned map are reflected on the session.
	SessionData() map[string]any

	// StartForm begins the given questionnaire for the current user, asking
	// its first question. Any active form is replaced.
	StartForm(f *Form) error
}

type botCtx struct {
	telebot.Context
	bot  *Bot
	user *User
}

func (c *botCtx) BotUser() *User { return c.user }

func (c *botCtx) App() *host.App {
	return c.bot.app
}

func (c *botCtx) SessionGet(key string) (any, bool) {
	if c.user == nil || c.user.Session == nil || c.user.Session.Data == nil {
		return nil, false
	}
	v, ok := c.user.Session.Data[key]
	return v, ok
}

func (c *botCtx) SessionSet(key string, val any) {
	if c.user == nil || c.user.Session == nil {
		return
	}
	if c.user.Session.Data == nil {
		c.user.Session.Data = map[string]any{}
	}
	c.user.Session.Data[key] = val
}

func (c *botCtx) SessionData() map[string]any {
	if c.user == nil || c.user.Session == nil {
		return nil
	}
	return c.user.Session.Data
}

func (c *botCtx) StartForm(f *Form) error {
	return c.bot.startForm(c, c.user, f)
}
