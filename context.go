package bojet

import "gopkg.in/telebot.v4"

// HandlerFunc is the signature for bot handlers. The Context is an enriched
// version of telebot.Context that also carries the resolved User.
type HandlerFunc func(c Context) error

// Context extends telebot.Context with a BotUser() accessor so handlers
// don't have to call the store themselves.
type Context interface {
	telebot.Context

	// BotUser returns the resolved User for the message sender.
	// Returns nil if the sender is not registered.
	BotUser() *User
}

type botCtx struct {
	telebot.Context
	user *User
}

func (c *botCtx) BotUser() *User { return c.user }
