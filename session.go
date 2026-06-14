package bojet

// Session holds per-user runtime state for the lifetime of a conversation.
// It is created when a user is loaded into the cache and, unlike the identity
// fields on User, is never written by UserStore. Form progress can optionally
// be persisted separately (see SessionStore).
//
// A User resolved through the bot always carries a non-nil Session.
type Session struct {
	// CurrentPage is the menu screen the user is currently on.
	CurrentPage *Page
	// PageHistory is the back-navigation stack of menu screens.
	PageHistory PageHistory

	// input is the active conversation step (a questionnaire, the contact-admin
	// flow, …) that consumes the user's next message(s) instead of treating
	// them as menu navigation. nil when the user is navigating normally.
	input inputState

	// Data is a free-form, app-defined scratch space that lives for the
	// duration of the session. It is in-memory only and never persisted, so it
	// may hold values of any type. Use SessionGet/SessionSet on Context for
	// convenient access.
	Data map[string]any
}

// newSession returns a fresh session positioned on the given home page.
func newSession(home *Page) *Session {
	return &Session{CurrentPage: home, Data: map[string]any{}}
}
