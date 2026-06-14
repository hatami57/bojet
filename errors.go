package bojet

import "github.com/hatami57/microjet/core"

// Bot startup / configuration errors.
var (
	ErrStoreRequired = core.NewInternalError("Config", "UserStore is required — register one with the app (Module provides the default SQLite store)")
)

// Registration / authorization errors.
var (
	ErrUserNotFound    = core.NewNotFoundError("User", "User not found")
	ErrUserNotApproved = core.NewForbiddenError("User", "User is not approved")
)
