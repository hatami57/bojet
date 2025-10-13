package bot

import (
	"database/sql"
	"fmt"
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
	publicPage     *Page
	userHomePage   *Page

	users map[int64]*User
	pages map[int]*Page
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
		pages:          map[int]*Page{},
	}
}

func (b *SonateBot) Start() error {
	if err := b.LoadPages(); err != nil {
		return err
	}

	b.RegisterHandlers()
	b.StartSchedulers()

	log.Println("🚀 Bot started")
	b.TgBot.Start()

	return nil
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
	user, err := b.User(userID)
	if err != nil {
		log.Println("Error getting user:", err)
		return nil
	}
	if user == nil {
		log.Println("No user to get keyboard for")
		return nil
	}

	if user.IsSendingMessage {
		return nil
	}

	return user.CurrentPage.GetKeyboard(!user.PageHistory.IsEmpty())
}

func (b *SonateBot) User(id int64) (*User, error) {
	if user, ok := b.users[id]; ok {
		user.ResetExpiration(b.UserCacheExpirationDuration)
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

func (b *SonateBot) ProcessPage(c telebot.Context) (bool, error) {
	userID := c.Sender().ID

	user, err := b.User(userID)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, fmt.Errorf("no user to process page for")
	}

	text := c.Text()
	if text == "" {
		return false, fmt.Errorf("no page action")
	}

	if !user.PageHistory.IsEmpty() && text == PageBackText {
		user.CurrentPage = user.PageHistory.Pop()
		return true, c.Send(user.CurrentPage.Title, user.Keyboard())
	}

	for _, item := range user.CurrentPage.Items {
		if item.Title == text {
			if item.ShowPage != nil {
				user.PageHistory.Push(user.CurrentPage)
				user.CurrentPage = item.ShowPage
			} else if len(item.ForwardMessageIDs) > 0 {
				c.Send("TODO: forward some messages...")
			}

			return true, c.Send(user.CurrentPage.Title, user.Keyboard())
		}
	}

	return false, nil
}
