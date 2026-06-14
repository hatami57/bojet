package bojet

import (
	"time"
)

// Config holds all bot-specific settings that can be loaded from config.toml.
//
//	[bot]
//	token        = "YOUR_TOKEN"
//	proxyUrl     = ""
//	pollTimeout  = "10s"
//	cacheExpiry  = "30m"
//	adminIds     = [123456789]
//	contactAdmin = true
type Config struct {
	Token        string        `mapstructure:"token"`
	ProxyURL     string        `mapstructure:"proxyUrl"`
	PollTimeout  time.Duration `mapstructure:"pollTimeout"`
	CacheExpiry  time.Duration `mapstructure:"cacheExpiry"`
	AdminIDs     []int64       `mapstructure:"adminIds"`
	ContactAdmin bool          `mapstructure:"contactAdmin"`
}

// WithConfig applies all configurable BotConfig fields as bot options.
// It is designed to be used alongside LoadConfig():
//
//	cfg, _ := bojet.LoadConfig()
//	bot, err := bojet.New(cfg.Bot.Token, bojet.WithConfig(cfg.Bot), ...)
func WithConfig(cfg *Config) Option {
	return func(b *Bot) {
		if cfg == nil {
			return
		}
		for _, id := range cfg.AdminIDs {
			b.adminIDs[id] = struct{}{}
		}
		if cfg.ProxyURL != "" {
			b.config.ProxyURL = cfg.ProxyURL
		}
		if cfg.PollTimeout > 0 {
			b.config.PollTimeout = cfg.PollTimeout
		}
		if cfg.CacheExpiry > 0 {
			b.config.CacheExpiry = cfg.CacheExpiry
		}
		b.config.ContactAdmin = cfg.ContactAdmin
	}
}
