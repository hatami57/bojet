package config

import "time"

type Config struct {
	Token       string
	AdminIDs    map[int64]struct{}
	DBPath      string
	PollTimeout time.Duration
	ProxyURL    string
}

func Load() *Config {
	return &Config{
		Token: "1062231928:AAEborYND1XwgGvsBsrvM0oaLsJueZP_lo4",
		AdminIDs: map[int64]struct{}{
			188550979: {},
		},
		DBPath:      "./bot.db",
		PollTimeout: 10 * time.Second,
		ProxyURL:    "http://127.0.0.1:2080",
	}
}

func (c *Config) IsAdmin(userID int64) bool {
	_, ok := c.AdminIDs[userID]
	return ok
}
