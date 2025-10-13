package bot

import (
	"time"

	"gopkg.in/telebot.v4"
)

type User struct {
	ID               int64
	FirstName        string
	LastName         string
	Username         string
	PhoneNumber      string
	IsConfirmed      bool
	IsSendingMessage bool
	CurrentPage      *Page
	PageHistory      PageHistory

	ExpireAt time.Time
}

func NewUser(page *Page, expirationDuration time.Duration) *User {
	user := &User{
		ID:               0,
		FirstName:        "",
		LastName:         "",
		Username:         "",
		PhoneNumber:      "",
		IsConfirmed:      false,
		IsSendingMessage: false,
		CurrentPage:      page,
		PageHistory:      PageHistory{},
	}
	user.ResetExpiration(expirationDuration)

	return user
}

func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

func (u *User) String() string {
	return u.FullName() + " (" + u.Username + ")"
}

func (u *User) IsExpired() bool {
	return u.ExpireAt.Before(time.Now())
}

func (u *User) ResetExpiration(duration time.Duration) {
	u.ExpireAt = time.Now().Add(duration)
}

func (u *User) Keyboard() *telebot.ReplyMarkup {
	return u.CurrentPage.GetKeyboard(!u.PageHistory.IsEmpty())
}
