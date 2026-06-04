// Dynamic pages example: page structure is fixed at startup, but ActionItem
// handlers compute their responses at call time — fetching from a database,
// calling an API, or tailoring content to the individual user.
// Use this pattern for dashboards, reports, or any content that changes.
package main

import (
	"bojet"
	"bojet/sqlite"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
)

// --- Mock data layer ---------------------------------------------------------
// In a real bot, these would query your database or call an external API.

type Order struct {
	ID     int
	Item   string
	Status string
}

func fetchUserOrders(userID int64) []Order {
	// Simulated per-user orders.
	return []Order{
		{ID: 1001, Item: "Laptop", Status: "Delivered"},
		{ID: 1002, Item: "Keyboard", Status: "In Transit"},
	}
}

func fetchLiveStats() string {
	// Simulated live dashboard data.
	return fmt.Sprintf(
		"📊 Live Stats\nOnline users: %d\nOrders today: %d\nRevenue: $%d",
		rand.Intn(200)+50,
		rand.Intn(500)+100,
		rand.Intn(50000)+10000,
	)
}

func fetchWeatherForecast() string {
	forecasts := []string{
		"🌤 Tehran: 28°C, Partly Cloudy",
		"⛅ Tehran: 22°C, Overcast",
		"☀️ Tehran: 35°C, Sunny",
		"🌧 Tehran: 18°C, Rain expected",
	}
	return forecasts[rand.Intn(len(forecasts))]
}

// -----------------------------------------------------------------------------

func main() {
	rand.Seed(time.Now().UnixNano())

	store, err := sqlite.NewStore("./dynamic.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// --- Page tree ---
	// The structure (items) is static. The CONTENT each item sends is computed
	// fresh on every button press.

	myOrdersPage := bojet.NewPage("📦 My Orders",
		bojet.ActionItem("📋 View all orders", func(c bojet.Context) error {
			orders := fetchUserOrders(c.BotUser().ID)
			if len(orders) == 0 {
				return c.Send("You have no orders yet.")
			}
			msg := "Your orders:\n"
			for _, o := range orders {
				msg += fmt.Sprintf("• #%d — %s (%s)\n", o.ID, o.Item, o.Status)
			}
			return c.Send(msg)
		}),
		bojet.ActionItem("🔍 Track latest order", func(c bojet.Context) error {
			orders := fetchUserOrders(c.BotUser().ID)
			if len(orders) == 0 {
				return c.Send("No orders to track.")
			}
			latest := orders[len(orders)-1]
			return c.Send(fmt.Sprintf("Order #%d (%s): %s", latest.ID, latest.Item, latest.Status))
		}),
	)

	dashboardPage := bojet.NewPage("📊 Dashboard",
		bojet.ActionItem("📈 Live stats", func(c bojet.Context) error {
			// Content fetched fresh on every press.
			return c.Send(fetchLiveStats())
		}),
		bojet.ActionItem("🌤 Weather", func(c bojet.Context) error {
			return c.Send(fetchWeatherForecast())
		}),
		bojet.ActionItem("🕐 Server time", func(c bojet.Context) error {
			return c.Send("Server time: " + time.Now().Format("2006-01-02 15:04:05"))
		}),
	)

	profilePage := bojet.NewPage("👤 My Profile",
		bojet.ActionItem("📋 View profile", func(c bojet.Context) error {
			// Personalized — content differs per user.
			u := c.BotUser()
			return c.Send(fmt.Sprintf(
				"👤 Profile\nName: %s\nUsername: @%s\nPhone: %s\nStatus: %s",
				u.FullName(),
				u.Username,
				u.PhoneNumber,
				confirmationStatus(u.IsConfirmed),
			))
		}),
	)

	homePage := bojet.NewPage("🏠 Main Menu",
		bojet.NavItem("📦 My Orders", myOrdersPage),
		bojet.NavItem("📊 Dashboard", dashboardPage),
		bojet.NavItem("👤 My Profile", profilePage),
	)

	bot, err := bojet.New(os.Getenv("BOT_TOKEN"),
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

func confirmationStatus(confirmed bool) string {
	if confirmed {
		return "✅ Confirmed"
	}
	return "⏳ Pending approval"
}
