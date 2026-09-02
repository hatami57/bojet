package bojet

import (
	"errors"
	"testing"
	"time"

	"github.com/hatami57/microjet/core/configx"
	"github.com/hatami57/microjet/gormx"
	"github.com/hatami57/microjet/gormx/sqlite"
	"github.com/hatami57/microjet/host"
)

// The host validates every service's config before it initializes any of them,
// so a missing token is caught before the bot ever reaches Telegram.
func TestBootFailsWithoutToken(t *testing.T) {
	app, err := host.New()
	if err != nil {
		t.Fatalf("host.New: %v", err)
	}

	err = app.
		WithModule(gormx.Module(sqlite.Driver())).
		WithModule(Module()).
		InitServices().
		Err()

	if !errors.Is(err, ErrTokenRequired) {
		t.Fatalf("boot error = %v; want ErrTokenRequired", err)
	}
}

// The store is registered before the bot, so it initializes first and a missing
// database is reported before the bot tries to build its Telegram client.
//
// This also covers host.WithConfigValue as the README documents it: config
// validation runs before any service initializes, so reaching the database error
// at all proves the token was supplied through the config layer.
func TestBootFailsWithoutDatabase(t *testing.T) {
	app, err := host.New(host.WithConfigValue("bot.token", "test-token"))
	if err != nil {
		t.Fatalf("host.New: %v", err)
	}

	err = app.WithModule(Module()).InitServices().Err()

	if !errors.Is(err, ErrDatabaseRequired) {
		t.Fatalf("boot error = %v; want ErrDatabaseRequired", err)
	}
}

// ReadConfig registers the defaults that apply when nothing configures the bot
// section. Seeded through an empty reader so the assertion is about bojet's own
// SetDefault calls (see the note below about partially-seeded map readers).
func TestReadConfigDefaults(t *testing.T) {
	bot := New()
	if err := bot.ReadConfig(configx.NewMapReader(nil)); err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if bot.config.PollTimeout != 10*time.Second {
		t.Fatalf("pollTimeout = %v; want 10s", bot.config.PollTimeout)
	}
	if bot.config.CacheExpiry != 30*time.Minute {
		t.Fatalf("cacheExpiry = %v; want 30m", bot.config.CacheExpiry)
	}
	if !bot.config.ContactAdmin {
		t.Fatal("contactAdmin = false; want the default true")
	}
}

// A fully specified [bot] section overrides every default.
func TestReadConfigReadsSection(t *testing.T) {
	// NOTE: every key is spelled out on purpose. configx.NewMapReader drops a
	// section's defaults as soon as that section appears in the seeded map
	// (viper's UnmarshalKey reads the config layer alone), so a partial map here
	// would zero the keys it omits rather than default them.
	reader := configx.NewMapReader(map[string]any{
		"bot": map[string]any{
			"token":        "test-token",
			"pollTimeout":  "3s",
			"cacheExpiry":  "1m",
			"contactAdmin": false,
			"adminIds":     []int64{7},
		},
	})

	bot := New()
	if err := configx.ReadAndValidate(reader, bot); err != nil {
		t.Fatalf("ReadAndValidate: %v", err)
	}

	if bot.config.Token != "test-token" {
		t.Fatalf("token = %q; want %q", bot.config.Token, "test-token")
	}
	if bot.config.PollTimeout != 3*time.Second {
		t.Fatalf("pollTimeout = %v; want 3s", bot.config.PollTimeout)
	}
	if bot.config.CacheExpiry != time.Minute {
		t.Fatalf("cacheExpiry = %v; want 1m", bot.config.CacheExpiry)
	}
	if bot.config.ContactAdmin {
		t.Fatal("contactAdmin = true; want the configured false")
	}
	if len(bot.config.AdminIDs) != 1 || bot.config.AdminIDs[0] != 7 {
		t.Fatalf("adminIds = %v; want [7]", bot.config.AdminIDs)
	}
}
