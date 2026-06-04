package bojet

import (
	"time"

	"gopkg.in/telebot.v4"
)

// Option configures a Bot at construction time.
type Option func(*Bot)

// WithStore sets the UserStore used for persistent user data.
// This option is required — Start() returns an error if no store is set.
func WithStore(store UserStore) Option {
	return func(b *Bot) { b.store = store }
}

// WithAdmins sets the Telegram user IDs that have admin privileges.
func WithAdmins(ids ...int64) Option {
	return func(b *Bot) {
		for _, id := range ids {
			b.adminIDs[id] = struct{}{}
		}
	}
}

// WithProxy sets an HTTP proxy URL for all Telegram API requests.
func WithProxy(proxyURL string) Option {
	return func(b *Bot) { b.proxyURL = proxyURL }
}

// WithPollTimeout sets the long-polling timeout (default: 10s).
func WithPollTimeout(d time.Duration) Option {
	return func(b *Bot) { b.pollTimeout = d }
}

// WithCacheExpiry sets how long a user is cached in memory before being
// reloaded from the store (default: 30m).
func WithCacheExpiry(d time.Duration) Option {
	return func(b *Bot) { b.cacheExpiry = d }
}

// WithHomePage sets the root navigation page shown to confirmed users.
func WithHomePage(page *Page) Option {
	return func(b *Bot) { b.homePage = page }
}

// WithRegistrationFlow replaces the default PhoneVerificationFlow.
// Use NoRegistrationFlow for open bots, or supply a custom implementation.
func WithRegistrationFlow(flow RegistrationFlow) Option {
	return func(b *Bot) { b.registration = flow }
}

// WithContactAdmin enables or disables the "Contact Admin" feature (default: true).
// When enabled, confirmed users see a button to send messages to all admins.
func WithContactAdmin(enabled bool) Option {
	return func(b *Bot) { b.contactAdmin = enabled }
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
	}
}

// WithErrorHandler sets a callback for errors from handlers or background
// tasks. c is nil when called from a background goroutine (e.g. broadcast).
func WithErrorHandler(fn func(err error, c telebot.Context)) Option {
	return func(b *Bot) { b.errorHandler = fn }
}
