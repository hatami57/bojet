package bojet

import "sync"

// SessionStore persists a user's Session so an in-progress form survives the
// in-memory user cache expiring. The bot saves a session while a form is
// active and deletes it once the form completes or is cancelled, so the store
// only ever holds sessions for users who are mid-questionnaire.
//
// The default implementation, MemorySessionStore, keeps sessions in process
// memory and loses them on restart. To resume forms across restarts, implement
// this interface against a database. Note that Session carries runtime-only
// pointers (CurrentPage, the active form, validators) that a serialising store
// cannot round-trip verbatim — such a store would persist the answers plus the
// pending question and re-link to the registered Form by ID on load.
type SessionStore interface {
	// LoadSession returns the stored session for the user, or nil if none.
	LoadSession(userID int64) (*Session, error)
	// SaveSession stores (or replaces) the session for the user.
	SaveSession(userID int64, s *Session) error
	// DeleteSession removes any stored session for the user.
	DeleteSession(userID int64) error
}

// MemorySessionStore is an in-memory SessionStore. Sessions live for the life
// of the process and are not persisted across restarts. It is the default.
type MemorySessionStore struct {
	mu       sync.Mutex
	sessions map[int64]*Session
}

// NewMemorySessionStore returns an empty in-memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: map[int64]*Session{}}
}

func (m *MemorySessionStore) LoadSession(userID int64) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[userID], nil
}

func (m *MemorySessionStore) SaveSession(userID int64, s *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[userID] = s
	return nil
}

func (m *MemorySessionStore) DeleteSession(userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, userID)
	return nil
}
