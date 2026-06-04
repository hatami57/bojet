package bojet

import "time"

// User represents a registered bot user. Fields are populated from the
// UserStore; runtime fields (IsSendingMessage, CurrentPage, PageHistory)
// are managed in-memory and not persisted.
type User struct {
	ID          int64
	FirstName   string
	LastName    string
	Username    string
	PhoneNumber string
	IsConfirmed bool

	// runtime state — not persisted
	IsSendingMessage bool
	CurrentPage      *Page
	PageHistory      PageHistory

	expireAt time.Time
}

// FullName returns "FirstName LastName", trimming extra spaces.
func (u *User) FullName() string {
	if u.LastName == "" {
		return u.FirstName
	}
	return u.FirstName + " " + u.LastName
}

// String returns a human-readable label for logging.
func (u *User) String() string {
	if u.Username != "" {
		return u.FullName() + " (@" + u.Username + ")"
	}
	return u.FullName()
}

func (u *User) isExpired() bool {
	return time.Now().After(u.expireAt)
}

func (u *User) resetExpiration(d time.Duration) {
	u.expireAt = time.Now().Add(d)
}
