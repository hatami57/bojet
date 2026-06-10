package bojet

import (
	"time"

	"github.com/hatami57/microjet/core"
)

// Config is the root configuration for a bojet application.
// Use LoadConfig() to populate it from config.toml and environment variables.
type Config struct {
	Log *core.LogConfig `mapstructure:"log"`
	Bot *BotConfig      `mapstructure:"bot"`
}

// BotConfig holds all bot-specific settings that can be loaded from config.toml.
//
//	[bot]
//	token        = "YOUR_TOKEN"
//	proxyUrl     = ""
//	pollTimeout  = "10s"
//	cacheExpiry  = "30m"
//	adminIds     = [123456789]
//	contactAdmin = true
type BotConfig struct {
	Token        string        `mapstructure:"token"`
	ProxyURL     string        `mapstructure:"proxyUrl"`
	PollTimeout  time.Duration `mapstructure:"pollTimeout"`
	CacheExpiry  time.Duration `mapstructure:"cacheExpiry"`
	AdminIDs     []int64       `mapstructure:"adminIds"`
	ContactAdmin bool          `mapstructure:"contactAdmin"`
}

// LoadConfig reads config.toml (or config.local.toml) from the working directory
// and merges environment variable overrides (prefix: APP_).
func LoadConfig() (*Config, error) {
	reader, err := core.NewViperConfigReader("")
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	return cfg, reader.ReadAll(cfg)
}

// WithConfig applies all configurable BotConfig fields as bot options.
// It is designed to be used alongside LoadConfig():
//
//	cfg, _ := bojet.LoadConfig()
//	bot, err := bojet.New(cfg.Bot.Token, bojet.WithConfig(cfg.Bot), ...)
func WithConfig(cfg *BotConfig) Option {
	return func(b *Bot) {
		if cfg == nil {
			return
		}
		for _, id := range cfg.AdminIDs {
			b.adminIDs[id] = struct{}{}
		}
		if cfg.ProxyURL != "" {
			b.proxyURL = cfg.ProxyURL
		}
		if cfg.PollTimeout > 0 {
			b.pollTimeout = cfg.PollTimeout
		}
		if cfg.CacheExpiry > 0 {
			b.cacheExpiry = cfg.CacheExpiry
		}
		b.contactAdmin = cfg.ContactAdmin
	}
}

// NewFromConfig is a convenience constructor that extracts the token from cfg.Bot
// and applies WithConfig automatically.
//
//	cfg, err := bojet.LoadConfig()
//	bot, err := bojet.NewFromConfig(cfg, bojet.WithStore(store), bojet.WithHomePage(home))
func NewFromConfig(cfg *Config, opts ...Option) (*Bot, error) {
	token := ""
	allOpts := make([]Option, 0, 2+len(opts))

	if cfg != nil && cfg.Bot != nil {
		token = cfg.Bot.Token
		allOpts = append(allOpts, WithConfig(cfg.Bot))
	}
	if cfg != nil && cfg.Log != nil {
		allOpts = append(allOpts, WithLogger(core.NewLogger(cfg.Log, false)))
	}
	allOpts = append(allOpts, opts...)

	return New(token, allOpts...)
}
