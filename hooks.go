package bojet

type hooks struct {
	onUserRegistered []func(*User) error
	onUserApproved   []func(*User) error
	onUserRejected   []func(*User) error
}

// OnUserRegistered registers a callback invoked when a new user submits their
// phone number. Errors returned by the callback are logged via the error handler.
func (b *Bot) OnUserRegistered(fn func(*User) error) {
	b.hooks.onUserRegistered = append(b.hooks.onUserRegistered, fn)
}

// OnUserApproved registers a callback invoked when an admin approves a user.
func (b *Bot) OnUserApproved(fn func(*User) error) {
	b.hooks.onUserApproved = append(b.hooks.onUserApproved, fn)
}

// OnUserRejected registers a callback invoked when an admin rejects a user.
func (b *Bot) OnUserRejected(fn func(*User) error) {
	b.hooks.onUserRejected = append(b.hooks.onUserRejected, fn)
}

func (b *Bot) fireHooks(fns []func(*User) error, u *User) {
	for _, fn := range fns {
		if err := fn(u); err != nil {
			b.errorHandler(err, nil)
		}
	}
}
