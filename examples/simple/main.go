// Simple example: an open bot with no registration, a couple of commands,
// and a basic two-level menu. Intended as a minimal starting point.
//
// The bot is wired as a microjet module: host.New builds the App, opens the
// SQLite database (from the [database] config section), and runs the bojet
// module through the standard config → init → setup → start lifecycle. Provide
// the Telegram token via the [bot] section of config.toml or the APP_BOT_TOKEN
// environment variable.
package main

import (
	"github.com/hatami57/bojet"
	"github.com/hatami57/microjet/gormx/sqlite"
	"github.com/hatami57/microjet/host"
)

func main() {
	homePage := bojet.NewPage(
		"🏠 Menu",
		bojet.ActionItem("👋 Say Hello", func(c bojet.Context) error {
			return c.Send("Hello, " + c.BotUser().FirstName + "!")
		}),
		bojet.ActionItem("ℹ️ About", func(c bojet.Context) error {
			return c.Send("This is a simple bojet bot.")
		}),
	)

	host.MustNew().
		WithDatabase(sqlite.Driver()).
		WithModule(bojet.Module(
			bojet.WithPublicAccess(),
			bojet.WithContactAdmin(false),
			bojet.WithHomePage(homePage),
			bojet.WithHandler("/help", func(c bojet.Context) error {
				return c.Send("Use the menu buttons below.")
			}),
		)).
		MustRun()
}
