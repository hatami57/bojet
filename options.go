package bojet

import (
	"time"

	"gopkg.in/telebot.v4"
)

// Option configures a Bot at construction time.
type Option func(*Bot)

// WithAdmins sets the Telegram user IDs that have admin privileges.
func WithAdmins(ids ...int64) Option {
	return func(b *Bot) {
		b.config.AdminIDs = ids
	}
}

// WithProxy sets an HTTP proxy URL for all Telegram API requests.
func WithProxy(proxyURL string) Option {
	return func(b *Bot) { b.config.ProxyURL = proxyURL }
}

// WithPollTimeout sets the long-polling timeout (default: 10s).
func WithPollTimeout(d time.Duration) Option {
	return func(b *Bot) { b.config.PollTimeout = d }
}

// WithCacheExpiry sets how long a user is cached in memory before being
// reloaded from the store (default: 30m).
func WithCacheExpiry(d time.Duration) Option {
	return func(b *Bot) { b.config.CacheExpiry = d }
}

// WithHomePage sets the root navigation page shown to confirmed users.
func WithHomePage(page *Page) Option {
	return func(b *Bot) { b.homePage = page }
}

// WithSessionStore sets the store used to persist in-progress form sessions
// (default: an in-memory store). Pass a custom implementation to survive
// restarts, or nil to disable session persistence entirely.
func WithSessionStore(store SessionStore) Option {
	return func(b *Bot) { b.sessions = store }
}

// WithRegistrationFlow replaces the default PhoneVerificationFlow.
// Use NoRegistrationFlow for open bots, or supply a custom implementation.
func WithRegistrationFlow(flow RegistrationFlow) Option {
	return func(b *Bot) { b.registration = flow }
}

// WithPublicAccess makes the bot open to everyone: senders are provisioned on
// first contact with no admin approval. It is shorthand for
// WithRegistrationFlow(&NoRegistrationFlow{}).
//
// The default (no option) keeps the admin-approval flow, where users share
// their phone number and an admin must approve them before use.
func WithPublicAccess() Option {
	return func(b *Bot) { b.registration = &NoRegistrationFlow{} }
}

// WithContactAdmin enables or disables the "Contact Admin" feature (default: true).
// When enabled, confirmed users see a button to send messages to all admins.
func WithContactAdmin(enabled bool) Option {
	return func(b *Bot) { b.config.ContactAdmin = enabled }
}

// WithMessages overrides specific bot messages. Only non-empty fields are
// applied; unset fields keep the DefaultMessages value.
func WithMessages(m Messages) Option {
	return func(b *Bot) {
		if m.Welcome != "" {
			b.messages.Welcome = m.Welcome
		}
		if m.SharePhoneButton != "" {
			b.messages.SharePhoneButton = m.SharePhoneButton
		}
		if m.ContactAdminButton != "" {
			b.messages.ContactAdminButton = m.ContactAdminButton
		}
		if m.NotAuthorized != "" {
			b.messages.NotAuthorized = m.NotAuthorized
		}
		if m.RegistrationPending != "" {
			b.messages.RegistrationPending = m.RegistrationPending
		}
		if m.Approved != "" {
			b.messages.Approved = m.Approved
		}
		if m.Rejected != "" {
			b.messages.Rejected = m.Rejected
		}
		if m.ContactAdminPrompt != "" {
			b.messages.ContactAdminPrompt = m.ContactAdminPrompt
		}
		if m.MessageSent != "" {
			b.messages.MessageSent = m.MessageSent
		}
		if m.MessageSendFailed != "" {
			b.messages.MessageSendFailed = m.MessageSendFailed
		}
		if m.ReplyDelivered != "" {
			b.messages.ReplyDelivered = m.ReplyDelivered
		}
		if m.ReplyFailed != "" {
			b.messages.ReplyFailed = m.ReplyFailed
		}
		if m.UnknownCommand != "" {
			b.messages.UnknownCommand = m.UnknownCommand
		}
		if m.GenericError != "" {
			b.messages.GenericError = m.GenericError
		}
		if m.CancelButton != "" {
			b.messages.CancelButton = m.CancelButton
		}
		if m.ContactAdminCancelled != "" {
			b.messages.ContactAdminCancelled = m.ContactAdminCancelled
		}
		if m.FormDoneButton != "" {
			b.messages.FormDoneButton = m.FormDoneButton
		}
		if m.FormCancelled != "" {
			b.messages.FormCancelled = m.FormCancelled
		}
		if m.FormInvalidChoice != "" {
			b.messages.FormInvalidChoice = m.FormInvalidChoice
		}
	}
}

// WithErrorHandler sets a callback for errors from handlers or background
// tasks. c is nil when called from a background goroutine (e.g. broadcast).
func WithErrorHandler(fn func(err error, c telebot.Context)) Option {
	return func(b *Bot) { b.errorHandler = fn }
}

// WithHandler registers a custom handler for the given Telegram endpoint
// (command, button, or telebot event). The handler receives an enriched
// Context with BotUser() already resolved. Mirrors Bot.Handle, but as an
// option so the whole bot can be configured through Module.
//
//	bojet.WithHandler("/help", func(c bojet.Context) error {
//	    return c.Send("Use the menu buttons below.")
//	})
func WithHandler(endpoint any, h HandlerFunc) Option {
	return func(b *Bot) {
		b.pendingHandlers = append(b.pendingHandlers, pendingHandler{endpoint: endpoint, handler: h})
	}
}

// WithOnUserRegistered registers a callback invoked when a new user submits
// their phone number (or is provisioned on a public bot).
func WithOnUserRegistered(fn func(*User) error) Option {
	return func(b *Bot) { b.hooks.onUserRegistered = append(b.hooks.onUserRegistered, fn) }
}

// WithOnUserApproved registers a callback invoked when an admin approves a user.
func WithOnUserApproved(fn func(*User) error) Option {
	return func(b *Bot) { b.hooks.onUserApproved = append(b.hooks.onUserApproved, fn) }
}

// WithOnUserRejected registers a callback invoked when an admin rejects a user.
func WithOnUserRejected(fn func(*User) error) Option {
	return func(b *Bot) { b.hooks.onUserRejected = append(b.hooks.onUserRejected, fn) }
}

// WithSchedule registers a recurring job using a standard cron expression.
// Mirrors Bot.Schedule, but as an option.
//
//	bojet.WithSchedule("0 9 * * *", func() { /* ... */ })
func WithSchedule(expr string, fn func()) Option {
	return func(b *Bot) {
		b.pendingSchedules = append(b.pendingSchedules, pendingSchedule{expr: expr, fn: fn})
	}
}

// WithScheduledBroadcast schedules a recurring broadcast of msg to all confirmed
// users using a standard cron expression.
//
//	bojet.WithScheduledBroadcast("0 9 * * *", "🌅 Good morning!")
func WithScheduledBroadcast(expr string, msg string) Option {
	return func(b *Bot) {
		b.pendingSchedules = append(b.pendingSchedules, pendingSchedule{
			expr: expr,
			fn: func() {
				if err := b.Broadcast(msg); err != nil {
					b.errorHandler(err, nil)
				}
			},
		})
	}
}
