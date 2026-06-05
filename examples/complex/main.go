// Complex example: phone verification, multi-level pages, event hooks,
// Persian messages, scheduled broadcast, and a custom error handler.
package main

import (
	"bojet"
	"bojet/sqlite"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hatami57/microjet/core"
	"github.com/hatami57/microjet/utils"
	"gopkg.in/telebot.v4"
)

func main() {
	// Structured logger from microjet/core (text by default; set LOG_FORMAT=json
	// or LOG_LEVEL=debug to change it without touching code).
	logger := core.NewLogger(&core.LogConfig{
		Level:  utils.GetEnvString("LOG_LEVEL", "info"),
		Format: utils.GetEnvString("LOG_FORMAT", "text"),
	})

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
	bot, err := bojet.New(utils.GetEnvString("BOT_TOKEN", ""),
		bojet.WithStore(store),
		bojet.WithAdmins(adminIDsFromEnv()...),
		bojet.WithProxy(utils.GetEnvString("BOT_PROXY_URL", "")),
		bojet.WithHomePage(homePage),
		bojet.WithCacheExpiry(15*time.Minute),
		bojet.WithLogger(logger),

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
				logger.Error("handler error", "user_id", c.Sender().ID, "error", err)
			} else {
				logger.Error("background error", "error", err)
			}
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	// --- Event hooks ---
	bot.OnUserRegistered(func(u *bojet.User) error {
		logger.Info("new registration", "user", u.String(), "phone", u.PhoneNumber)
		return nil
	})

	bot.OnUserApproved(func(u *bojet.User) error {
		logger.Info("user approved", "user", u.String())
		return nil
	})

	bot.OnUserRejected(func(u *bojet.User) error {
		logger.Info("user rejected", "user", u.String())
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

	logger.Info("bot started")
	if err := bot.Start(); err != nil {
		log.Fatal(err)
	}
}

// adminIDsFromEnv parses BOT_ADMIN_IDS (a comma-separated list of Telegram user
// IDs) using microjet's utils.GetEnvString for the lookup.
func adminIDsFromEnv() []int64 {
	raw := utils.GetEnvString("BOT_ADMIN_IDS", "")
	var ids []int64
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
