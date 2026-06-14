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

// WithConfig applies all configurable Config fields as bot options. The host
// normally populates Config from the [bot] section automatically (see
// Bot.ReadConfig); use this only to inject a Config you built yourself.
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
