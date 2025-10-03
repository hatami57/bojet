package config

import "time"

type Config struct {
	Token       string
	AdminIDs    []int64
	DBPath      string
	PollTimeout time.Duration
}

func Load() *Config {
	return &Config{
		Token: "1062231928:AAEborYND1XwgGvsBsrvM0oaLsJueZP_lo4",
		AdminIDs: []int64{
			188550979,
		},
		DBPath:      "./bot.db",
		PollTimeout: 10 * time.Second,
	}
}
