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

// WithConfig applies a Config you built yourself on top of the one the host
// read from the [bot] section (see Bot.ReadConfig). Empty strings and zero
// durations keep the configured value and AdminIDs are added to the configured
// admins. ContactAdmin is a plain bool with no "unset" state, so it is always
// applied: set it explicitly, or the feature is turned off.
//
// Token is ignored: the token is validated as soon as the config is read,
// before options apply (see Bot.Validate). Supply it through the config layer
// instead, e.g. host.WithConfigValue("bot.token", token).
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
