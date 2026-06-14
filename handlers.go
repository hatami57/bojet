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

// handleStart greets a /start. On a public bot (a flow implementing
// UserProvisioner) it provisions the sender and shows the home menu; otherwise
// it shows the registration welcome (e.g. the share-phone prompt).
func (b *Bot) handleStart(c telebot.Context) error {
	if prov, ok := b.registration.(UserProvisioner); ok {
		user, err := b.provision(prov, c.Sender())
		if err != nil {
			b.errorHandler(err, c)
			return c.Send(b.messages.GenericError)
		}
		if user != nil {
			title := b.messages.Welcome
			if b.homePage != nil {
				title = b.homePage.Title
			}
			return c.Send(title, b.userKeyboard(user))
		}
	}
	return c.Send(b.messages.Welcome, b.publicKeyboard)
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
	if err := b.store.SaveUser(user); err != nil {
		return nil, err
	}
	user.Session = newSession(b.homePage)
	b.cacheUser(user)
	b.fireHooks(b.hooks.onUserRegistered, user)
	return user, nil
}

func (b *Bot) messageHandler(c telebot.Context) error {
	if b.IsAdmin(c.Sender().ID) {
		return b.handleAdminMessage(c)
	}
	return b.handleUserMessage(c)
}

func (b *Bot) handleAdminMessage(c telebot.Context) error {
	msg := c.Message()
	if msg != nil && msg.ReplyTo != nil && msg.ReplyTo.OriginalSender != nil {
		return b.handleReplyToUser(c)
	}
	return c.Send(b.messages.UnknownCommand)
}

func (b *Bot) handleUserMessage(c telebot.Context) error {
	user, err := b.resolveUser(c.Sender().ID)
	if err != nil {
		b.errorHandler(err, c)
		return c.Send(b.messages.GenericError)
	}
	if user == nil {
		// Public bots provision unknown senders on first contact instead of
		// rejecting them.
		if prov, ok := b.registration.(UserProvisioner); ok {
			user, err = b.provision(prov, c.Sender())
			if err != nil {
				b.errorHandler(err, c)
				return c.Send(b.messages.GenericError)
			}
		}
		if user == nil {
			return c.Send(b.messages.NotAuthorized, b.publicKeyboard)
		}
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
	if b.contactAdmin && c.Text() == b.messages.ContactAdminButton {
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

func (b *Bot) handleReplyToUser(c telebot.Context) error {
	original := c.Message().ReplyTo.OriginalSender
	if _, err := b.tb.Forward(original, c.Message()); err != nil {
		b.logger.Error("forward reply to user failed", "user_id", original.ID, "error", err)
		return c.Send(b.messages.ReplyFailed)
	}
	return c.Send(b.messages.ReplyDelivered)
}

// handleContact is called by PhoneVerificationFlow when a user shares their phone.
func (b *Bot) handleContact(c telebot.Context) error {
	contact := c.Message().Contact

	existing, err := b.resolveUser(contact.UserID)
	if err != nil {
		b.errorHandler(err, c)
		return c.Send(b.messages.GenericError)
	}
	if existing != nil {
		return c.Send(b.messages.RegistrationPending)
	}

	user := &User{
		ID:          contact.UserID,
		FirstName:   contact.FirstName,
		LastName:    contact.LastName,
		Username:    c.Sender().Username,
		PhoneNumber: contact.PhoneNumber,
		IsConfirmed: false,
		Session:     newSession(b.homePage),
	}

	if err := b.store.SaveUser(user); err != nil {
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

	if err := b.store.SetConfirmed(userID, true); err != nil {
		b.errorHandler(err, c)
		return c.Respond(&telebot.CallbackResponse{Text: "DB error"})
	}

	// update cache if present
	b.mu.Lock()
	if u, ok := b.users[userID]; ok {
		u.IsConfirmed = true
	}
	b.mu.Unlock()

	user, _ := b.resolveUser(userID)
	if user != nil {
		b.fireHooks(b.hooks.onUserApproved, user)
	}

	if _, err := b.tb.Send(&telebot.User{ID: userID}, b.messages.Approved); err != nil {
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

	if err := b.store.SetConfirmed(userID, false); err != nil {
		b.errorHandler(err, c)
		return c.Respond(&telebot.CallbackResponse{Text: "DB error"})
	}

	// evict from cache so next load reflects new state
	b.mu.Lock()
	user := b.users[userID]
	delete(b.users, userID)
	b.mu.Unlock()

	if user != nil {
		b.fireHooks(b.hooks.onUserRejected, user)
	}

	if _, err := b.tb.Send(&telebot.User{ID: userID}, b.messages.Rejected); err != nil {
		b.errorHandler(err, c)
	}

	return c.Edit(fmt.Sprintf("❌ Rejected user %d", userID))
}
