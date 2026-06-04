// Complex example: phone verification, multi-level pages, event hooks,
// Persian messages, scheduled broadcast, and a custom error handler.
package main

import (
	"bojet"
	"bojet/sqlite"
	"log"
	"os"
	"time"

	"gopkg.in/telebot.v4"
)

func main() {
	store, err := sqlite.NewStore("./complex.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// --- Page tree ---
	supportPage := bojet.NewPage("🛠 Support",
		bojet.ActionItem("📞 Call us", func(c bojet.Context) error {
			return c.Send("Call us at +98-21-00000000")
		}),
		bojet.ActionItem("📧 Email us", func(c bojet.Context) error {
			return c.Send("support@example.com")
		}),
	)

	accountPage := bojet.NewPage("👤 My Account",
		bojet.ActionItem("📋 My Info", func(c bojet.Context) error {
			u := c.BotUser()
			return c.Send("Name: " + u.FullName() + "\nPhone: " + u.PhoneNumber)
		}),
		bojet.ActionItem("🔒 Change Password", func(c bojet.Context) error {
			return c.Send("To change your password, visit our website.")
		}),
	)

	homePage := bojet.NewPage("🏠 منوی اصلی",
		bojet.NavItem("👤 حساب من", accountPage),
		bojet.NavItem("🛠 پشتیبانی", supportPage),
		bojet.ActionItem("📰 اخبار", func(c bojet.Context) error {
			return c.Send("آخرین اخبار: ...")
		}),
	)

	// --- Bot ---
	bot, err := bojet.New(os.Getenv("BOT_TOKEN"),
		bojet.WithStore(store),
		bojet.WithAdmins(adminIDFromEnv()),
		bojet.WithProxy(os.Getenv("BOT_PROXY_URL")),
		bojet.WithHomePage(homePage),
		bojet.WithCacheExpiry(15*time.Minute),

		// Override only the strings you need.
		bojet.WithMessages(bojet.Messages{
			Welcome:             "سلام! برای ثبت‌نام شماره تلفن خود را به اشتراک بگذارید:",
			SharePhoneButton:    "📱 اشتراک‌گذاری شماره",
			ContactAdminButton:  "📞 تماس با ادمین",
			NotAuthorized:       "⛔ دسترسی ندارید. لطفاً منتظر تأیید ادمین بمانید.",
			RegistrationPending: "✅ درخواست شما ثبت شد. منتظر تأیید ادمین باشید.",
			Approved:            "🎉 درخواست شما تأیید شد! اکنون می‌توانید از ربات استفاده کنید.",
			Rejected:            "🚫 متأسفانه درخواست شما رد شد.",
		}),

		bojet.WithErrorHandler(func(err error, c telebot.Context) {
			if c != nil {
				log.Printf("handler error (user %d): %v", c.Sender().ID, err)
			} else {
				log.Println("background error:", err)
			}
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	// --- Event hooks ---
	bot.OnUserRegistered(func(u *bojet.User) error {
		log.Printf("new registration: %s (%s)", u, u.PhoneNumber)
		return nil
	})

	bot.OnUserApproved(func(u *bojet.User) error {
		log.Printf("approved: %s", u)
		return nil
	})

	bot.OnUserRejected(func(u *bojet.User) error {
		log.Printf("rejected: %s", u)
		return nil
	})

	// --- Custom commands ---
	bot.Handle("/help", func(c bojet.Context) error {
		return c.Send("از دکمه‌های منو برای ناوبری استفاده کنید.")
	})

	// --- Scheduled messages ---
	if err := bot.ScheduleBroadcast("0 9 * * *", "🌅 صبح بخیر!"); err != nil {
		log.Fatal(err)
	}
	if err := bot.ScheduleBroadcast("0 20 * * 5", "🎉 آخر هفته مبارک!"); err != nil {
		log.Fatal(err)
	}

	log.Println("started")
	if err := bot.Start(); err != nil {
		log.Fatal(err)
	}
}

func adminIDFromEnv() int64 {
	// In a real app, parse BOT_ADMIN_IDS from env.
	return 123456789
}
