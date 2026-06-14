// Static pages example: a deep, fixed navigation tree where every page and
// every response is defined once at startup and never changes at runtime.
// Use this pattern for bots that serve fixed content: FAQs, catalogs,
// documentation, or any menu that doesn't depend on user or database state.
package main

import (
	"log"

	"github.com/hatami57/bojet"
	"github.com/hatami57/bojet/store"
	"github.com/hatami57/microjet/utils"
)

func main() {
	store, err := store.NewStore("./static.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// --- Build a fixed page tree ---

	// Leaf pages — no sub-navigation, just text responses.
	faqDelivery := bojet.NewPage(
		"🚚 Delivery FAQ",
		bojet.ActionItem("How long does delivery take?", func(c bojet.Context) error {
			return c.Send("Standard delivery takes 3–5 business days.")
		}),
		bojet.ActionItem("Do you ship internationally?", func(c bojet.Context) error {
			return c.Send("Yes, we ship to 50+ countries. See our website for the full list.")
		}),
	)

	faqPayment := bojet.NewPage(
		"💳 Payment FAQ",
		bojet.ActionItem("What payment methods do you accept?", func(c bojet.Context) error {
			return c.Send("We accept Visa, MasterCard, and bank transfer.")
		}),
		bojet.ActionItem("Is my payment secure?", func(c bojet.Context) error {
			return c.Send("Yes, all payments are encrypted with TLS 1.3.")
		}),
	)

	faqReturns := bojet.NewPage(
		"🔄 Returns FAQ",
		bojet.ActionItem("What is your return policy?", func(c bojet.Context) error {
			return c.Send("You may return any item within 30 days of purchase.")
		}),
		bojet.ActionItem("How do I start a return?", func(c bojet.Context) error {
			return c.Send("Contact our support team with your order number.")
		}),
	)

	// Mid-level pages group related leaf pages.
	faqPage := bojet.NewPage(
		"❓ FAQ",
		bojet.NavItem("🚚 Delivery", faqDelivery),
		bojet.NavItem("💳 Payment", faqPayment),
		bojet.NavItem("🔄 Returns", faqReturns),
	)

	contactPage := bojet.NewPage(
		"📞 Contact Us",
		bojet.ActionItem("📧 Email", func(c bojet.Context) error {
			return c.Send("hello@example.com")
		}),
		bojet.ActionItem("🌐 Website", func(c bojet.Context) error {
			return c.Send("https://example.com")
		}),
		bojet.ActionItem("📍 Address", func(c bojet.Context) error {
			return c.Send("123 Main Street, Tehran, Iran")
		}),
	)

	// Root home page.
	homePage := bojet.NewPage(
		"🏠 Welcome",
		bojet.NavItem("❓ FAQ", faqPage),
		bojet.NavItem("📞 Contact Us", contactPage),
		bojet.ActionItem("📢 About Us", func(c bojet.Context) error {
			return c.Send("We are a leading e-commerce company founded in 2010.")
		}),
	)

	bot, err := bojet.New(
		utils.GetEnvString("BOT_TOKEN", ""),
		bojet.WithStore(store),
		bojet.WithAdmins(123456789),
		bojet.WithHomePage(homePage),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("started")
	if err := bot.Start(); err != nil {
		log.Fatal(err)
	}
}
