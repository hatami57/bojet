package bojet

import "time"

// User represents a registered bot user. The identity fields are populated
// from the UserStore and persisted; ephemeral runtime state lives on Session,
// which is managed in-memory and not written by UserStore.
type User struct {
	ID          int64
	FirstName   string
	LastName    string
	Username    string
	PhoneNumber string
	IsConfirmed bool

	// Session holds ephemeral runtime state (current page, form progress,
	// scratch data). It is not persisted by UserStore and is always non-nil
	// for a user resolved through the bot.
	Session *Session

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

func (u *User) isExpired(now time.Time) bool {
	return now.After(u.expireAt)
}

func (u *User) resetExpiration(now time.Time, d time.Duration) {
	u.expireAt = now.Add(d)
}
