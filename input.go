package bojet

import "gopkg.in/telebot.v4"

// inputState routes the user's next message(s) to an active conversation step
// — a questionnaire (formState), the contact-admin flow (contactAdmin), or any
// future prompt — instead of treating them as menu navigation. While a session
// holds an inputState, handleUserMessage delegates each message to it.
type inputState interface {
	// handle processes one incoming message and returns the state to keep
	// active: itself to stay, another to chain, or nil to finish and hand
	// control back to menu navigation.
	handle(c Context, b *Bot) (inputState, error)
}

// contactAdmin captures one message from the user and forwards it to all
// admins, then finishes. Entered when the user presses the contact-admin button.
type contactAdmin struct{}

func (contactAdmin) handle(c Context, b *Bot) (inputState, error) {
	u := c.BotUser()

	if c.Text() == b.messages.CancelButton {
		return nil, c.Send(b.messages.ContactAdminCancelled, b.userKeyboard(u))
	}

	for adminID := range b.adminIDs {
		if _, err := b.tb.Forward(&telebot.User{ID: adminID}, c.Message()); err != nil {
			// Stay active so the user can retry.
			return contactAdmin{}, c.Send(b.messages.MessageSendFailed, b.cancelKeyboard())
		}
	}
	return nil, c.Send(b.messages.MessageSent, b.userKeyboard(u))
}

// cancelKeyboard is the reply keyboard shown during a simple capture prompt:
// a single Cancel button.
func (b *Bot) cancelKeyboard() *telebot.ReplyMarkup {
	rm := &telebot.ReplyMarkup{ResizeKeyboard: true}
	rm.Reply(rm.Row(rm.Text(b.messages.CancelButton)))
	return rm
}
