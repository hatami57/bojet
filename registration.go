package bojet

import "gopkg.in/telebot.v4"

// RegistrationFlow controls how users are authenticated before using the bot.
// Implement this interface to replace the default phone-verification flow.
type RegistrationFlow interface {
	// SetupHandlers registers any flow-specific Telegram handlers.
	// Called once during Bot.Start().
	SetupHandlers(b *Bot)

	// IsAllowed returns true if the user with the given ID is permitted to
	// send messages to the bot. Called on every non-whitelisted update.
	IsAllowed(userID int64, store UserStore) (bool, error)
}

// PhoneVerificationFlow is the default flow: users share their phone number
// and an admin must approve them before they can interact with the bot.
type PhoneVerificationFlow struct{}

func (f *PhoneVerificationFlow) SetupHandlers(b *Bot) {
	b.tb.Handle(telebot.OnContact, func(c telebot.Context) error {
		return b.handleContact(c)
	})
	b.tb.Handle(&telebot.Btn{Unique: "approve"}, func(c telebot.Context) error {
		return b.handleApprove(c)
	})
	b.tb.Handle(&telebot.Btn{Unique: "reject"}, func(c telebot.Context) error {
		return b.handleReject(c)
	})
}

func (f *PhoneVerificationFlow) IsAllowed(userID int64, store UserStore) (bool, error) {
	user, err := store.GetUser(userID)
	if err != nil || user == nil {
		return false, err
	}
	return user.IsConfirmed, nil
}

// NoRegistrationFlow admits all users without any verification.
// Useful for public bots or bots that manage access at the handler level.
type NoRegistrationFlow struct{}

func (f *NoRegistrationFlow) SetupHandlers(_ *Bot) {}

func (f *NoRegistrationFlow) IsAllowed(_ int64, _ UserStore) (bool, error) {
	return true, nil
}
