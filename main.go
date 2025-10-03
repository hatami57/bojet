package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"gopkg.in/telebot.v4"
)

var db *sql.DB
var adminID int64 = 188550979

func main() {
	var err error
	db, err = sql.Open("sqlite", "./users.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create table if not exists
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            tg_id BIGINT UNIQUE,
            phone TEXT,
            is_confirmed BOOLEAN DEFAULT 0
        );
    `)
	if err != nil {
		log.Fatal(err)
	}

	pref := telebot.Settings{
		Token:  "1062231928:AAEborYND1XwgGvsBsrvM0oaLsJueZP_lo4",
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}

	// Middleware: check if user is confirmed
	b.Use(func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			if c.Sender().ID == adminID {
				return next(c) // admin always allowed
			}

			if c.Message() != nil {
				if c.Message().Text == "/start" || c.Message().Contact != nil {
					return next(c)
				}
			}

			var confirmed bool
			err := db.QueryRow("SELECT is_confirmed FROM users WHERE tg_id=?", c.Sender().ID).Scan(&confirmed)
			if err != nil || !confirmed {
				return c.Send("⛔ You are not authorized to use this bot. Please wait for admin approval.")
			}
			return next(c)
		}
	})

	// Start command
	b.Handle("/start", func(c telebot.Context) error {
		keyboard := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}
		btn := keyboard.Contact("📱 Share phone number")
		keyboard.Reply(keyboard.Row(btn))
		return c.Send("Welcome! Please share your phone number:", keyboard)
	})

	// Handle contact
	b.Handle(telebot.OnContact, func(c telebot.Context) error {
		contact := c.Message().Contact
		_, err := db.Exec("INSERT OR IGNORE INTO users (tg_id, phone) VALUES (?, ?)", c.Sender().ID, contact.PhoneNumber)
		if err != nil {
			return c.Send("⚠️ Failed to save your phone number.")
		}
		inline := &telebot.ReplyMarkup{}
		approveBtn := inline.Data("✅ Approve", "approve", strconv.FormatInt(c.Sender().ID, 10))
		rejectBtn := inline.Data("❌ Reject", "reject", strconv.FormatInt(c.Sender().ID, 10))
		inline.Inline(inline.Row(approveBtn, rejectBtn))

		// Notify admin
		b.Send(&telebot.User{ID: adminID},
			fmt.Sprintf("📥 New request:\nUser: @%s\nPhone: %s",
				c.Sender().Username, contact.PhoneNumber),
			inline,
		)

		return c.Send("✅ Your request has been submitted. Please wait for admin approval.")
	})

	b.Handle(&telebot.Btn{Unique: "approve"}, func(c telebot.Context) error {
		if c.Sender().ID != adminID {
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
		b.Send(&telebot.User{ID: userID}, "🎉 Your request has been approved! You can now use the bot.")
		return c.Edit(fmt.Sprintf("✅ Approved user %d", userID))
	})

	// Reject handler
	b.Handle(&telebot.Btn{Unique: "reject"}, func(c telebot.Context) error {
		if c.Sender().ID != adminID {
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
		b.Send(&telebot.User{ID: userID}, "❌ Sorry, your request was rejected by admin.")
		return c.Edit(fmt.Sprintf("❌ Rejected user %d", userID))
	})

	// Example: confirmed users can run /hello
	b.Handle("/hello", func(c telebot.Context) error {
		return c.Send("Hello, confirmed user!")
	})

	log.Println("Bot started...")
	b.Start()
}
