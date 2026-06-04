package bojet

import "github.com/hatami57/microjet/core"

// Bot startup / configuration errors.
var (
	ErrStoreRequired = core.NewInternalError("Config", "UserStore is required — use WithStore()")
)

// Registration / authorization errors.
var (
	ErrUserNotFound    = core.NewNotFoundError("User", "User not found")
	ErrUserNotApproved = core.NewForbiddenError("User", "User is not approved")
)
