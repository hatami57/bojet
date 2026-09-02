package bojet

// UserStore is the persistence layer for user data. Implement this interface
// to use any database backend. The sqlite sub-package provides a default
// SQLite implementation.
type UserStore interface {
	// GetUser returns the user with the given Telegram ID, or nil if not found.
	GetUser(id int64) (*User, error)

	// SaveUser inserts or updates the user record.
	SaveUser(user *User) error

	// SetConfirmed updates the is_confirmed flag for the given user. It returns
	// an error matching ErrUserNotFound when no user has that ID, so an admin
	// approving a user who has since been deleted is told so rather than
	// getting a silent success.
	SetConfirmed(id int64, confirmed bool) error

	// ListConfirmedIDs returns the Telegram IDs of all confirmed users.
	ListConfirmedIDs() ([]int64, error)
}
