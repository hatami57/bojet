package bot

import (
	"database/sql"
	"fmt"
	"log"
	"slices"
	"sonatebot/internal/config"
	"strconv"

	"gopkg.in/telebot.v4"
)

var adminAnchors = map[int64]int{}
var pendingContact = map[int64]bool{}

func RegisterHandlers(tb *telebot.Bot, db *sql.DB, cfg *config.Config) {
	// Send anchor messages to each admin
	for _, adminID := range cfg.AdminIDs {
		msg, err := tb.Send(&telebot.User{ID: adminID}, "💬 User messages will appear in this thread")
		if err != nil {
			log.Printf("⚠️ Failed to send anchor to admin %d: %v", adminID, err)
			continue
		}
		adminAnchors[adminID] = msg.ID
	}

	// Start command
	tb.Handle("/start", func(c telebot.Context) error {
		log.Printf("Handle /start")

		keyboard := &telebot.ReplyMarkup{ResizeKeyboard: true}
		btnShare := keyboard.Contact("📱 Share phone number")
		btnContact := keyboard.Text("📞 Contact Admin")
		keyboard.Reply(
			keyboard.Row(btnShare),
			keyboard.Row(btnContact),
		)

		return c.Send("Welcome! Please share your phone number:", keyboard)
	})

	// Handle contact
	tb.Handle(telebot.OnContact, func(c telebot.Context) error {
		log.Printf("Handle Contact")
		contact := c.Message().Contact
		_, err := db.Exec("INSERT OR IGNORE INTO users (tg_id, phone) VALUES (?, ?)", c.Sender().ID, contact.PhoneNumber)
		if err != nil {
			return c.Send("⚠️ Failed to save your phone number.")
		}
		inline := &telebot.ReplyMarkup{}
		approveBtn := inline.Data("✅ Approve", "approve", strconv.FormatInt(c.Sender().ID, 10))
		rejectBtn := inline.Data("❌ Reject", "reject", strconv.FormatInt(c.Sender().ID, 10))
		inline.Inline(inline.Row(approveBtn, rejectBtn))
		msg := fmt.Sprintf("📥 New request:\nUser: @%s\nPhone: %s",
			c.Sender().Username, contact.PhoneNumber)

		// Notify admins
		for _, adminID := range cfg.AdminIDs {
			tb.Send(&telebot.User{ID: adminID}, msg, inline)
		}

		return c.Send("✅ Your request has been submitted. Please wait for admin approval.")
	})

	tb.Handle(&telebot.Btn{Unique: "approve"}, func(c telebot.Context) error {
		log.Printf("Handle approve")
		if !slices.Contains(cfg.AdminIDs, c.Sender().ID) {
			return c.Respond(&telebot.CallbackResponse{Text: "Not authorized"})
		}
		userID, err := strconv.ParseInt(c.Data(), 10, 64)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "Invalid user ID"})
		}
		_, err = db.Exec("UPDATE users SET is_confirmed=1 WHERE tg_id=?", userID)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "DB error"})
		}
		tb.Send(&telebot.User{ID: userID}, "🎉 Your request has been approved! You can now use the bot.")
		return c.Edit(fmt.Sprintf("✅ Approved user %d", userID))
	})

	// Reject handler
	tb.Handle(&telebot.Btn{Unique: "reject"}, func(c telebot.Context) error {
		log.Printf("Handle reject")
		if !slices.Contains(cfg.AdminIDs, c.Sender().ID) {
			return c.Respond(&telebot.CallbackResponse{Text: "Not authorized"})
		}
		userID, err := strconv.ParseInt(c.Data(), 10, 64)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "Invalid user ID"})
		}
		// Optionally remove from DB
		_, err = db.Exec("DELETE FROM users WHERE tg_id=?", userID)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "DB error"})
		}
		tb.Send(&telebot.User{ID: userID}, "❌ Sorry, your request was rejected by admin.")
		return c.Edit(fmt.Sprintf("❌ Rejected user %d", userID))
	})

	// Forward confirmed user messages → reply to each admin’s anchor
	forwardHandler := func(c telebot.Context) error {
		log.Printf("Handle forward")
		if slices.Contains(cfg.AdminIDs, c.Sender().ID) {
			log.Printf("Handle forward: is admin, return.")
			return nil
		}

		user := c.Sender()
		var isConfirmed bool
		err := db.QueryRow("SELECT is_confirmed FROM users WHERE tg_id=?", user.ID).Scan(&isConfirmed)
		if err != nil || !isConfirmed {
			log.Printf("Handle forward: user is not confirmed, return.")
			return nil
		}

		for adminID, anchorMsgID := range adminAnchors {
			_, err := tb.Forward(&telebot.User{ID: adminID}, c.Message(), &telebot.SendOptions{
				ReplyTo: &telebot.Message{ID: anchorMsgID},
			})
			if err != nil {
				log.Printf("⚠️ Forward error to admin %d: %v", adminID, err)
			}
		}
		log.Printf("Handle forward: sent to admin.")

		return c.Send("✅ Sent to admin.")
	}

	tb.Handle(telebot.OnText, forwardHandler)
	tb.Handle(telebot.OnVoice, forwardHandler)
	tb.Handle(telebot.OnAudio, forwardHandler)
	tb.Handle(telebot.OnVideo, forwardHandler)

	// Admin replies → relay back to user
	tb.Handle(telebot.OnText, func(c telebot.Context) error {
		if c.Text() == "📞 Contact Admin" {
			pendingContact[c.Sender().ID] = true
			return c.Send("✍️ Please type or record your message for the admin. It will be delivered directly.")
		}

		// If user is in contact mode
		if pendingContact[c.Sender().ID] {
			user := c.Sender()

			// Forward this message to all admins as a reply to anchor
			for adminID, anchorMsgID := range adminAnchors {
				_, err := tb.Forward(&telebot.User{ID: adminID}, c.Message(), &telebot.SendOptions{
					ReplyTo: &telebot.Message{ID: anchorMsgID},
				})
				if err != nil {
					log.Printf("⚠️ Forward error to admin %d: %v", adminID, err)
				}
			}

			delete(pendingContact, user.ID)
			return c.Send("✅ Your message has been sent to the admin.")
		}

		return nil

		// log.Printf("Handle Text: check if its admin")
		// if !slices.Contains(cfg.AdminIDs, c.Sender().ID) {
		// 	log.Printf("Handle Text: its not admin, return.")
		// 	return nil
		// }
		// if c.Message().ReplyTo == nil || c.Message().ReplyTo.OriginalSender == nil {
		// 	log.Printf("Handle Text: ReplyTo == nil || ReplyTo.Sender == nil. return.")
		// 	return nil
		// }
		//
		// user := c.Message().ReplyTo.OriginalSender
		// _, err := tb.Send(user, c.Text())
		// if err != nil {
		// 	log.Println("Send error:", err)
		// 	return c.Send("⚠️ Failed to send reply to user.")
		// }
		// log.Printf("Handle Text: Reply delivered.")
		// return c.Send("✅ Reply delivered.")
	})

}
