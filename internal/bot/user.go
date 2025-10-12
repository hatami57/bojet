package bot

import "time"

type User struct {
	ID               int64
	FirstName        string
	LastName         string
	Username         string
	PhoneNumber      string
	IsConfirmed      bool
	IsSendingMessage bool

	ExpireAt time.Time
}

func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

func (u *User) String() string {
	return u.FullName() + " (" + u.Username + ")"
}
