package bojet

import (
	"github.com/hatami57/microjet/core/errorx"
)

// Bot startup / configuration errors.
var (
	ErrStoreRequired = errorx.NewInternalError("Config", "UserStore is required — register one with the app (Module provides the default SQLite store)")
)

// Registration / authorization errors.
var (
	ErrUserNotFound    = errorx.NewNotFoundError("User", "User not found")
	ErrUserNotApproved = errorx.NewForbiddenError("User", "User is not approved")
)
