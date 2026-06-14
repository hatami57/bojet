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

// UserProvisioner is an optional RegistrationFlow extension for open/public
// bots. When a flow implements it, the bot provisions (creates and persists) a
// user on first contact instead of rejecting unknown senders. Flows that gate
// access behind approval (the default) do not implement it, so unknown senders
// are turned away until registered.
type UserProvisioner interface {
	// Provision builds the User to persist for a first-time sender, or returns
	// nil to decline (falling back to the not-authorized path).
	Provision(sender *telebot.User) *User
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

// NoRegistrationFlow admits all users without any verification and provisions
// them on first contact. Use it (or WithPublicAccess) for public bots open to
// everyone with no admin approval step.
type NoRegistrationFlow struct{}

func (f *NoRegistrationFlow) SetupHandlers(_ *Bot) {}

func (f *NoRegistrationFlow) IsAllowed(_ int64, _ UserStore) (bool, error) {
	return true, nil
}

// Provision creates a confirmed user from the sender's Telegram profile, so
// public users are persisted (and reachable via Broadcast) on first contact.
func (f *NoRegistrationFlow) Provision(sender *telebot.User) *User {
	return &User{
		ID:          sender.ID,
		FirstName:   sender.FirstName,
		LastName:    sender.LastName,
		Username:    sender.Username,
		IsConfirmed: true,
	}
}
