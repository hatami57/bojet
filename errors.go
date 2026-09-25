package bojet

import (
	"github.com/hatami57/microjet/core/errorx"
)

// Bot startup / configuration errors. Each carries its own Subject, because
// errorx matches sentinels by category *and* subject: sharing one subject would
// make errors.Is(err, ErrTokenRequired) true for any of them.
var (
	ErrStoreRequired    = errorx.NewInternalError("UserStore", "UserStore is required — register one with the app (Module provides the default SQLite store)")
	ErrDatabaseRequired = errorx.NewInternalError("Database", "a database is required — install one with the app (e.g. gormx.Module(sqlite.Driver()))")
	ErrTokenRequired    = errorx.NewInternalError("BotToken", "[bot] token is required — set it in config.toml, via APP_BOT_TOKEN, or with host.WithConfigValue(\"bot.token\", …)")
	ErrInvalidDuration  = errorx.NewInternalError("BotConfig", "duration must not be negative")
)

// Registration / authorization errors.
var (
	ErrUserNotFound    = errorx.NewNotFoundError("User", "User not found")
	ErrUserNotApproved = errorx.NewForbiddenError("User", "User is not approved")
)
