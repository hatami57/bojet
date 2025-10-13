package bot

import (
	"fmt"
	"log"
	"strconv"

	"gopkg.in/telebot.v4"
)

func (b *SonateBot) RegisterHandlers() {
	b.TgBot.Handle("/start", b.handleStart)

	b.TgBot.Handle(telebot.OnContact, b.handleContactMessage)
	b.TgBot.Handle(&telebot.Btn{Unique: "approve"}, b.handleApproveUser)
	b.TgBot.Handle(&telebot.Btn{Unique: "reject"}, b.handleRejectUser)

	b.TgBot.Handle(telebot.OnText, b.messageHandler)
	b.TgBot.Handle(telebot.OnVoice, b.messageHandler)
	b.TgBot.Handle(telebot.OnAudio, b.messageHandler)
	b.TgBot.Handle(telebot.OnVideo, b.messageHandler)
}

func (b *SonateBot) handleStart(c telebot.Context) error {
	return c.Send("Welcome! Please share your phone number:", b.PublicKeyboard())
}

func (b *SonateBot) handleContactMessage(c telebot.Context) error {
	contact := c.Message().Contact
	if err := b.SaveContact(contact); err != nil {
		return c.Send("⚠️ Failed to save your phone number.")
	}

	inline := &telebot.ReplyMarkup{}
	approveBtn := inline.Data("✅ Approve", "approve", strconv.FormatInt(contact.UserID, 10))
	rejectBtn := inline.Data("❌ Reject", "reject", strconv.FormatInt(contact.UserID, 10))
	inline.Inline(inline.Row(approveBtn, rejectBtn))
	msg := fmt.Sprintf("📥 New request:\nFrom User: @%s\nFirst Name: %s\nLast Name: %s\nPhone Number: %s",
		c.Sender().Username, contact.FirstName, contact.LastName, contact.PhoneNumber)

	for adminID := range b.cfg.AdminIDs {
		if _, err := b.TgBot.Send(&telebot.User{ID: adminID}, msg, inline); err != nil {
			return c.Send("⚠️ Failed to send request to admin.")
		}
	}

	return c.Send("✅ Your request has been submitted. Please wait for admin approval.")
}

func (b *SonateBot) handleApproveUser(c telebot.Context) error {
	if !b.IsAdmin(c.Sender().ID) {
		return c.Respond(&telebot.CallbackResponse{Text: "🚫 Not authorized"})
	}
	userID, err := strconv.ParseInt(c.Data(), 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Invalid user ID"})
	}
	if err = b.SetUserConfirmation(userID, true); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "DB error"})
	}

	_, err = b.TgBot.Send(&telebot.User{ID: userID}, "🎉 Your request has been approved! You can now use the bot.")
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Failed to send message to user"})
	}

	return c.Edit(fmt.Sprintf("✅ Approved user %d", userID))
}

func (b *SonateBot) handleRejectUser(c telebot.Context) error {
	if !b.IsAdmin(c.Sender().ID) {
		return c.Respond(&telebot.CallbackResponse{Text: "🚫 Not authorized"})
	}
	userID, err := strconv.ParseInt(c.Data(), 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Invalid user ID"})
	}
	if err = b.SetUserConfirmation(userID, false); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "DB error"})
	}

	_, err = b.TgBot.Send(&telebot.User{ID: userID}, "🚫 Sorry, your request was rejected by admin.")
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Failed to send message to user"})
	}

	return c.Edit(fmt.Sprintf("❌ Rejected user %d", userID))
}

func (b *SonateBot) messageHandler(c telebot.Context) error {
	if b.IsAdmin(c.Sender().ID) {
		return b.handleAdminMessage(c)
	}

	return b.handleUserMessage(c)
}

func (b *SonateBot) handleAdminMessage(c telebot.Context) error {
	msg := c.Message()
	if msg.ReplyTo != nil && msg.ReplyTo.OriginalSender != nil {
		return b.handleReplyToUserMessage(c)
	}

	return c.Send("⚠️ Unknown command")
}

func (b *SonateBot) handleUserMessage(c telebot.Context) error {
	userID := c.Sender().ID
	user, err := b.User(userID)
	if err != nil {
		return c.Send("⚠️ An error has occurred, please try again later.")
	}
	if user == nil {
		return c.Send("⚠️ You are not registered. Please share your phone number.", b.PublicKeyboard())
	}

	if user.IsSendingMessage {
		for adminID := range b.cfg.AdminIDs {
			if _, err := b.TgBot.Forward(&telebot.User{ID: adminID}, c.Message()); err != nil {
				return c.Send("⚠️ Failed to forward message to admin.")
			}
		}

		user.IsSendingMessage = false
		return c.Send("✅ Your message has been sent to the admin.")
	} else if c.Text() == "📞 Contact Admin" {
		user.IsSendingMessage = true
		return c.Send("✍️ Please type or record your message for the admin. It will be delivered directly.")
	}

	if !user.IsConfirmed {
		return c.Send("🚫 You are not approved to use this bot.", b.PublicKeyboard())
	}

	processed, err := b.ProcessPage(c)
	if processed {
		return nil
	}
	if err != nil {
		return c.Send("⚠️ An error has occurred, please try again later.", user.Keyboard())
	}

	return c.Send("⚠️ If you want to send a message to admin, use \"📞 Contact Admin\" button.", user.Keyboard())
}

func (b *SonateBot) handleReplyToUserMessage(c telebot.Context) error {
	originalSender := c.Message().ReplyTo.OriginalSender
	_, err := b.TgBot.Send(originalSender, c.Text())
	if err != nil {
		log.Println("Reply to user error:", err)
		return c.Send("⚠️ Failed to send reply to user.")
	}

	return c.Send("✅ Reply delivered.")
}
