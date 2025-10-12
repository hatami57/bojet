package bot

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"sonatebot/internal/config"
	"time"

	"gopkg.in/telebot.v4"
)

type SonateBot struct {
	TgBot                       *telebot.Bot
	UserCacheExpirationDuration time.Duration

	db             *sql.DB
	cfg            *config.Config
	publicKeyboard *telebot.ReplyMarkup
	userKeyboard   *telebot.ReplyMarkup

	users map[int64]*User
}

func NewSonateBot(cfg *config.Config, db *sql.DB) *SonateBot {
	botSettings := telebot.Settings{
		Token:  cfg.Token,
		Poller: &telebot.LongPoller{Timeout: cfg.PollTimeout},
	}

	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			log.Fatal("Invalid proxy URL:", err)
		}

		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		botSettings.Client = &http.Client{Transport: transport}
	}

	tb, err := telebot.NewBot(botSettings)
	if err != nil {
		log.Fatal(err)
	}

	publicKeyboard := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}
	publicKeyboard.Reply(
		publicKeyboard.Row(publicKeyboard.Contact("📱 Share phone number")),
		publicKeyboard.Row(publicKeyboard.Text("📞 Contact Admin")),
	)

	userKeyboard := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}
	userKeyboard.Reply(
		userKeyboard.Row(userKeyboard.Text("📞 Contact Admin")),
	)

	return &SonateBot{
		TgBot:                       tb,
		UserCacheExpirationDuration: time.Minute * 30,

		db:             db,
		cfg:            cfg,
		publicKeyboard: publicKeyboard,
		userKeyboard:   userKeyboard,
		users:          map[int64]*User{},
	}
}

func (b *SonateBot) Start() {
	b.RegisterHandlers()
	b.StartSchedulers()

	log.Println("🚀 Bot started")
	b.TgBot.Start()
}

func (b *SonateBot) Stop() {
	b.TgBot.Stop()
	log.Println("🛑 Bot stopped")
}

func (b *SonateBot) IsAdmin(userID int64) bool {
	return b.cfg.IsAdmin(userID)
}

func (b *SonateBot) PublicKeyboard() *telebot.ReplyMarkup {
	return b.publicKeyboard
}

func (b *SonateBot) UserKeyboard(userID int64) *telebot.ReplyMarkup {
	if user, ok := b.users[userID]; ok && user.IsSendingMessage {
		return nil
	}

	return b.userKeyboard
}

func (b *SonateBot) User(id int64) (*User, error) {
	if user, ok := b.users[id]; ok {
		return user, nil
	}

	user, err := b.loadUser(id)
	if err != nil {
		log.Println("Error loading user:", err)
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	b.users[id] = user
	return user, nil
}
