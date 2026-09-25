package bojet

import (
	"fmt"
	"strconv"

	"gopkg.in/telebot.v4"
)

func (b *Bot) setupHandlers() {
	b.tb.Handle("/start", b.handleStart)

	b.tb.Handle(telebot.OnText, b.messageHandler)
	b.tb.Handle(telebot.OnVoice, b.messageHandler)
	b.tb.Handle(telebot.OnAudio, b.messageHandler)
	b.tb.Handle(telebot.OnVideo, b.messageHandler)
}

// handleStart greets a /start. Known users are answered according to their
// registration state (approved users land on the home menu); on a public bot
// (a flow implementing UserProvisioner) unknown senders are provisioned first.
// Anyone else gets the registration welcome (e.g. the share-phone prompt).
func (b *Bot) handleStart(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}
	user, err := b.userFor(sender)
	if err != nil {
		b.errorHandler(err, c)
		return c.Send(b.messages.GenericError)
	}
	if user == nil {
		return c.Send(b.messages.Welcome, b.publicKeyboard)
	}
	return b.greetRegistered(c, user)
}

// greetRegistered answers a known user: approved users (and admins) are taken
// back to the home menu, others are told where their registration stands.
func (b *Bot) greetRegistered(c telebot.Context, user *User) error {
	switch {
	case user.IsConfirmed || b.IsAdmin(user.ID):
		return b.showHome(c, user)
	case user.IsRejected:
		return c.Send(b.messages.Rejected)
	default:
		return c.Send(b.messages.RegistrationPending)
	}
}

// showHome restarts the user's conversation on the home page, abandoning any
// active form or prompt.
func (b *Bot) showHome(c telebot.Context, user *User) error {
	if user.Session.input != nil {
		user.Session.input = nil
		b.deleteSession(user.ID)
	}
	user.Session.CurrentPage = b.homePage
	user.Session.PageHistory = nil

	title := b.messages.Welcome
	if b.homePage != nil {
		title = b.homePage.Title
	}
	return c.Send(title, b.userKeyboard(user))
}

// provision returns the existing user for the sender, or creates, persists and
// caches a new one via the flow's UserProvisioner. Returns nil if the flow
// declines to provision.
func (b *Bot) provision(prov UserProvisioner, sender *telebot.User) (*User, error) {
	existing, err := b.resolveUser(sender.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	user := prov.Provision(sender)
	if user == nil {
		return nil, nil
	}
	if err := b.userStore.SaveUser(user); err != nil {
		return nil, err
	}
	user.Session = newSession(b.homePage)
	b.cacheUser(user)
	b.fireHooks(b.hooks.onUserRegistered, user)
	return user, nil
}

// messageHandler routes an admin's reply to a forwarded user message back to
// that user; every other message is handled as menu/conversation input, for
// admins and users alike.
func (b *Bot) messageHandler(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}
	if b.IsAdmin(sender.ID) {
		if target := b.replyTarget(c.Message()); target != nil {
			return b.handleReplyToUser(c, target)
		}
	}
	return b.handleUserMessage(c)
}

func (b *Bot) handleUserMessage(c telebot.Context) error {
	user, err := b.userFor(c.Sender())
	if err != nil {
		b.errorHandler(err, c)
		return c.Send(b.messages.GenericError)
	}
	if user == nil {
		return c.Send(b.messages.NotAuthorized, b.publicKeyboard)
	}

	bc := &botCtx{Context: c, bot: b, user: user}

	// An active input state (questionnaire, contact-admin, …) consumes the
	// message instead of treating it as menu navigation.
	if st := user.Session.input; st != nil {
		next, err := st.handle(bc, b)
		if err != nil {
			b.errorHandler(err, c)
			return c.Send(b.messages.GenericError, b.userKeyboard(user))
		}
		user.Session.input = next
		return nil
	}

	// Enter the contact-admin flow.
	if b.contactAdminEnabled(user) && c.Text() == b.messages.ContactAdminButton {
		user.Session.input = contactAdmin{}
		return c.Send(b.messages.ContactAdminPrompt, b.cancelKeyboard())
	}

	// Back navigation.
	if !user.Session.PageHistory.IsEmpty() && c.Text() == PageBackText {
		user.Session.CurrentPage = user.Session.PageHistory.Pop()
		return c.Send(user.Session.CurrentPage.Title, b.userKeyboard(user))
	}

	// Page item navigation.
	if user.Session.CurrentPage != nil {
		matched, err := user.Session.CurrentPage.processText(c.Text(), bc, b)
		if err != nil {
			b.errorHandler(err, c)
			return c.Send(b.messages.GenericError, b.userKeyboard(user))
		}
		if matched {
			return nil
		}
	}

	return c.Send(b.messages.UnknownCommand, b.userKeyboard(user))
}

// contactAdminEnabled reports whether u gets the contact-admin feature: it must
// be enabled, there must be an admin to reach, and u must not be one.
func (b *Bot) contactAdminEnabled(u *User) bool {
	return b.config.ContactAdmin && len(b.adminIDs) > 0 && (u == nil || !b.IsAdmin(u.ID))
}

// replyTarget returns the user an admin's message replies to: the sender of a
// contact-admin message the bot forwarded, or the original sender of any other
// forwarded message Telegram still attributes. Returns nil when msg is not a
// reply to such a message.
func (b *Bot) replyTarget(msg *telebot.Message) *telebot.User {
	if msg == nil || msg.ReplyTo == nil {
		return nil
	}
	orig := msg.ReplyTo
	if orig.Chat != nil {
		if id, ok := b.relays.lookup(orig.Chat.ID, orig.ID); ok {
			return &telebot.User{ID: id}
		}
	}
	if orig.Origin != nil && orig.Origin.Sender != nil {
		return orig.Origin.Sender
	}
	return orig.OriginalSender
}

func (b *Bot) handleReplyToUser(c telebot.Context, target *telebot.User) error {
	if _, err := b.tb.Forward(target, c.Message()); err != nil {
		b.logger().Error("forward reply to user failed", "user_id", target.ID, "error", err)
		return c.Send(b.messages.ReplyFailed)
	}
	return c.Send(b.messages.ReplyDelivered)
}

// handleContact is called by PhoneVerificationFlow when a user shares their phone.
func (b *Bot) handleContact(c telebot.Context) error {
	contact := c.Message().Contact
	sender := c.Sender()

	// Only the sender's own contact registers them; any other contact card
	// would register someone else's Telegram ID (or ID 0 for non-users).
	if sender == nil || contact.UserID != sender.ID {
		return c.Send(b.messages.ShareOwnContact, b.publicKeyboard)
	}

	existing, err := b.userFor(sender)
	if err != nil {
		b.errorHandler(err, c)
		return c.Send(b.messages.GenericError)
	}
	if existing != nil {
		return b.greetRegistered(c, existing)
	}

	user := &User{
		ID:          contact.UserID,
		FirstName:   contact.FirstName,
		LastName:    contact.LastName,
		Username:    sender.Username,
		PhoneNumber: contact.PhoneNumber,
		IsConfirmed: false,
		Session:     newSession(b.homePage),
	}

	if err := b.userStore.SaveUser(user); err != nil {
		b.errorHandler(err, c)
		return c.Send(b.messages.GenericError)
	}
	b.cacheUser(user)
	b.fireHooks(b.hooks.onUserRegistered, user)

	inline := &telebot.ReplyMarkup{}
	idStr := strconv.FormatInt(contact.UserID, 10)
	approveBtn := inline.Data("✅ Approve", "approve", idStr)
	rejectBtn := inline.Data("❌ Reject", "reject", idStr)
	inline.Inline(inline.Row(approveBtn, rejectBtn))

	adminMsg := fmt.Sprintf("📥 New registration request:\nName: %s\nUsername: @%s\nPhone: %s",
		user.FullName(), user.Username, user.PhoneNumber)

	for adminID := range b.adminIDs {
		if _, err := b.tb.Send(&telebot.User{ID: adminID}, adminMsg, inline); err != nil {
			b.errorHandler(err, c)
		}
	}

	return c.Send(b.messages.RegistrationPending)
}

// handleApprove is called by PhoneVerificationFlow when an admin presses Approve.
func (b *Bot) handleApprove(c telebot.Context) error {
	if !b.IsAdmin(c.Sender().ID) {
		return c.Respond(&telebot.CallbackResponse{Text: "🚫 Not authorized"})
	}

	userID, err := strconv.ParseInt(c.Data(), 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Invalid user ID"})
	}

	if err := b.userStore.SetConfirmed(userID, true); err != nil {
		b.errorHandler(err, c)
		return c.Respond(&telebot.CallbackResponse{Text: "DB error"})
	}

	// Update the cached user, if any, so the approval takes effect at once.
	b.updateCachedUser(userID, func(u *User) {
		u.IsConfirmed = true
		u.IsRejected = false
	})

	user, err := b.resolveUser(userID)
	if err != nil {
		b.errorHandler(err, c)
	}
	if user != nil {
		b.fireHooks(b.hooks.onUserApproved, user)
	}

	// The user still has the share-phone keyboard; hand them the home menu. It
	// is built from a fresh session so this admin goroutine never touches the
	// user's live one.
	home := b.userKeyboard(&User{ID: userID, Session: newSession(b.homePage)})
	if _, err := b.tb.Send(&telebot.User{ID: userID}, b.messages.Approved, home); err != nil {
		b.errorHandler(err, c)
	}

	return c.Edit(fmt.Sprintf("✅ Approved user %d", userID))
}

// handleReject is called by PhoneVerificationFlow when an admin presses Reject.
func (b *Bot) handleReject(c telebot.Context) error {
	if !b.IsAdmin(c.Sender().ID) {
		return c.Respond(&telebot.CallbackResponse{Text: "🚫 Not authorized"})
	}

	userID, err := strconv.ParseInt(c.Data(), 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Invalid user ID"})
	}

	if err := b.userStore.SetConfirmed(userID, false); err != nil {
		b.errorHandler(err, c)
		return c.Respond(&telebot.CallbackResponse{Text: "DB error"})
	}

	// Evict from cache so the next load reflects the rejection.
	b.mu.Lock()
	cached := b.users[userID]
	delete(b.users, userID)
	b.mu.Unlock()

	// Load the user for the hooks even if they were not cached (e.g. after a
	// restart between registration and rejection).
	user, err := b.userStore.GetUser(userID)
	if err != nil {
		b.errorHandler(err, c)
	}
	if user == nil {
		user = cached
	}
	if user != nil {
		b.fireHooks(b.hooks.onUserRejected, user)
	}

	if _, err := b.tb.Send(&telebot.User{ID: userID}, b.messages.Rejected); err != nil {
		b.errorHandler(err, c)
	}

	return c.Edit(fmt.Sprintf("❌ Rejected user %d", userID))
}
