// Complex example: phone verification, multi-level pages, event hooks,
// Persian messages, scheduled broadcasts, and a custom error handler — all
// configured declaratively through bojet.Module and run by the microjet host.
//
// Admins, the Telegram token, and the database path come from config.toml (or
// APP_* environment variables). The whole bot is described by options, so there
// is no imperative wiring after construction.
package main

import (
	"gopkg.in/telebot.v4"

	"github.com/hatami57/bojet"
	"github.com/hatami57/microjet/core"
	"github.com/hatami57/microjet/gormx/sqlite"
	"github.com/hatami57/microjet/host"
)

func main() {
	// Structured logger from microjet/core, used by the hooks and error handler.
	logger := core.NewLogger(&core.LogConfig{Level: "info", Format: "text"}, false)

	// --- Page tree ---
	supportPage := bojet.NewPage(
		"🛠 Support",
		bojet.ActionItem("📞 Call us", func(c bojet.Context) error {
			return c.Send("Call us at +98-21-00000000")
		}),
		bojet.ActionItem("📧 Email us", func(c bojet.Context) error {
			return c.Send("support@example.com")
		}),
	)

	accountPage := bojet.NewPage(
		"👤 My Account",
		bojet.ActionItem("📋 My Info", func(c bojet.Context) error {
			u := c.BotUser()
			return c.Send("Name: " + u.FullName() + "\nPhone: " + u.PhoneNumber)
		}),
		bojet.ActionItem("🔒 Change Password", func(c bojet.Context) error {
			return c.Send("To change your password, visit our website.")
		}),
	)

	homePage := bojet.NewPage(
		"🏠 منوی اصلی",
		bojet.NavItem("👤 حساب من", accountPage),
		bojet.NavItem("🛠 پشتیبانی", supportPage),
		bojet.ActionItem("📰 اخبار", func(c bojet.Context) error {
			return c.Send("آخرین اخبار: ...")
		}),
	)

	host.MustNew().
		WithDatabase(sqlite.Driver()).
		WithModule(bojet.Module(
			// Admins and the token are read from the [bot] config section.
			bojet.WithHomePage(homePage),

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

			// Event hooks.
			bojet.WithOnUserRegistered(func(u *bojet.User) error {
				logger.Info("new registration", "user", u.String(), "phone", u.PhoneNumber)
				return nil
			}),
			bojet.WithOnUserApproved(func(u *bojet.User) error {
				logger.Info("user approved", "user", u.String())
				return nil
			}),
			bojet.WithOnUserRejected(func(u *bojet.User) error {
				logger.Info("user rejected", "user", u.String())
				return nil
			}),

			// Custom command.
			bojet.WithHandler("/help", func(c bojet.Context) error {
				return c.Send("از دکمه‌های منو برای ناوبری استفاده کنید.")
			}),

			// Scheduled broadcasts.
			bojet.WithScheduledBroadcast("0 9 * * *", "🌅 صبح بخیر!"),
			bojet.WithScheduledBroadcast("0 20 * * 5", "🎉 آخر هفته مبارک!"),

			bojet.WithErrorHandler(func(err error, c telebot.Context) {
				if c != nil {
					logger.Error("handler error", "user_id", c.Sender().ID, "error", err)
				} else {
					logger.Error("background error", "error", err)
				}
			}),
		)).
		MustRun()
}
