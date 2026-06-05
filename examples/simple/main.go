// Simple example: an open bot with no registration, a couple of commands,
// and a basic two-level menu. Intended as a minimal starting point.
package main

import (
	"bojet"
	"bojet/sqlite"
	"log"

	"github.com/hatami57/microjet/utils"
)

func main() {
	store, err := sqlite.NewStore("./simple.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	homePage := bojet.NewPage("🏠 Menu",
		bojet.ActionItem("👋 Say Hello", func(c bojet.Context) error {
			return c.Send("Hello, " + c.BotUser().FirstName + "!")
		}),
		bojet.ActionItem("ℹ️ About", func(c bojet.Context) error {
			return c.Send("This is a simple bojet bot.")
		}),
	)

	bot, err := bojet.New(utils.GetEnvString("BOT_TOKEN", ""),
		bojet.WithStore(store),
		bojet.WithRegistrationFlow(&bojet.NoRegistrationFlow{}),
		bojet.WithContactAdmin(false),
		bojet.WithHomePage(homePage),
	)
	if err != nil {
		log.Fatal(err)
	}

	bot.Handle("/help", func(c bojet.Context) error {
		return c.Send("Use the menu buttons below.")
	})

	log.Println("started")
	if err := bot.Start(); err != nil {
		log.Fatal(err)
	}
}
