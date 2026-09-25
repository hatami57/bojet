package bojet

// UserStore is the persistence layer for user data. Implement this interface
// to use any database backend. NewDBStore provides the default implementation
// over the host's gormx database.
type UserStore interface {
	// GetUser returns the user with the given Telegram ID, or nil if not found.
	GetUser(id int64) (*User, error)

	// SaveUser inserts or updates the user record.
	SaveUser(user *User) error

	// SetConfirmed records an admin's decision on the user's registration:
	// true approves them (and clears any rejection), false rejects them, which
	// also sets IsRejected so the user can be told their request was declined.
	SetConfirmed(id int64, confirmed bool) error

	// ListConfirmedIDs returns the Telegram IDs of all confirmed users.
	ListConfirmedIDs() ([]int64, error)
}
