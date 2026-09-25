# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Releases before 0.7.0 predate this file; see the git history for those.

## [0.8.0] - 2026-09-25

Upgrades microjet from v0.39.0 to v0.41.0 and fixes a round of bugs found in a
full review: two startup crashes, unsynchronized per-user state, and several
registration and admin-flow gaps.

### Breaking

- **`UserStore.SetConfirmed(id, false)` means "reject"** — implementations must
  record the rejection so `GetUser` reports `IsRejected`. The default store adds
  an `is_rejected` column (created by AutoMigrate; existing rows default to
  false, so users rejected before this release read as pending until rejected
  again). Approving clears it.

- **Admins use the menus** — an admin's messages used to be answered with
  "Unknown command" unless they replied to a forwarded message; they are now
  handled like any user's, and admins who are not registered users get an
  in-memory one. Replies to user messages still route to the user.

- **`/start` restarts approved users on the home page** — it cancels any active
  form or prompt. Pending and rejected users are told where their registration
  stands instead of being asked to share their phone again.

### Added

- **`Messages.ShareOwnContact`** — sent when a user shares a contact card that
  is not their own.
- **`User.IsRejected`**.

### Changed

- **`Broadcast` is paced** — messages go out one at a time at about 25 per
  second (Telegram's limit is about 30), are retried when Telegram answers 429,
  and are abandoned on shutdown. A large audience now takes minutes rather than
  flooding the API and losing most messages to rate limiting.
- **The default error handler logs** — without `WithErrorHandler`, errors went
  nowhere. They are now logged through the host's logger: client-caused errors
  (including every message from an unapproved user) at Warn, the rest at Error.
- **Contact Admin** counts a message as sent once any admin has it, so a retry
  no longer duplicates it to the admins who already got it. The button is
  hidden when no admins are configured, and from admins themselves.
- **Approved users receive the home keyboard** with the approval message.

### Fixed

- **A failed startup crashed on shutdown** — the host closes every service when
  boot fails, and `Bot.Close` dereferenced the Telegram client that `Init` never
  built (e.g. a bad token), so the process died with a nil-pointer panic
  instead of returning the error. When `Init` had succeeded but polling never
  started, `Close` blocked for its full 15s timeout.
- **Registering `bojet.Module` before `gormx.Module` panicked** — the store
  bound its connection in `Init`, before gormx had opened it. It now checks for
  the database in `Init` and binds in `Setup`, after every service is
  initialized.
- **Concurrent updates from one user raced** — telebot runs each update in its
  own goroutine, so two quick taps mutated the same session at once, losing
  updates or crashing with `concurrent map writes`. Updates are now handled one
  at a time per user; different users still run in parallel.
- **A panicking handler crashed the process** — telebot does not recover
  handler panics. They are now reported to the error handler, as are panics in
  scheduled jobs and broadcasts. Sender-less updates (channel posts) no longer
  crash custom handlers, and `StartForm` returns `ErrUserNotFound` for an
  unregistered sender.
- **`Bot.Handle` before `Start` bypassed authorization** — telebot binds
  middleware at registration, so handlers added before the middleware skipped
  the approval check. They are now queued and installed behind it.
- **Any contact card registered a user** — sharing someone else's contact
  registered *their* Telegram ID (a non-Telegram contact registered ID 0), which
  could lock that person out if an admin rejected it. Only the sender's own
  contact is accepted.
- **Deep-link `/start` was blocked** — `/start <payload>` and `/start@botname`
  reached the handler, but the middleware only admitted an exact `/start`, so
  new users were told they were not authorized.
- **The reject hook skipped uncached users** — `OnUserRejected` fired only if
  the user was in memory, so a restart between registration and rejection lost
  it.
- **Admin replies could not find the user** — they relied on the forward's
  `forward_from`, which Bot API 7.0 replaced with `forward_origin` and which is
  absent for users who hide their forwards. The bot now remembers which user
  each Contact Admin message came from, falling back to `forward_origin`.
- **The user cache grew forever** and **held its lock across store queries**,
  serializing every user behind the database; authorization also queried the
  store on every update. Expired entries are now swept, the lock is released
  during queries, and authorization reads through the cache.
- **Back on a form's first question** was stored as the answer; the question is
  now re-asked.
- **`WithConfig` documents that it ignores `Token`**, which `Validate` requires
  from the config layer before options apply.

## [0.7.0] - 2026-09-02

Upgrades the framework from microjet v0.17.0 to v0.39.0. Most of the churn is
microjet's move to module-based wiring (v0.19.0) and the `Table` write methods
that now report affected rows (v0.24.0, v0.30.0); the rest of the release spends
those new APIs on clearer failures.

### Breaking

- **Databases are installed as a module** — microjet dropped `App.WithDatabase`
  in favour of uniform `WithModule` wiring, so the host chain changes:

  ```go
  host.MustNew().
      WithModule(gormx.Module(sqlite.Driver())). // was WithDatabase(sqlite.Driver())
      WithModule(bojet.Module(...)).
      MustRun()
  ```

  This is why importing bojet no longer drags in the AWS SDK, Redis, Mongo, and
  gin: microjet's `host` stopped importing its satellite modules, and the
  dependency graph shrank with it.

- **`UserStore.SetConfirmed` reports an unknown user** — implementations must now
  return an error matching `ErrUserNotFound` when no user has that ID. The
  default store gets this from the row count `gormx.Table.UpdateMap` now returns:
  zero rows with a nil error means the ID matched nothing, not that the write
  failed. Previously an admin approving a user who had since been deleted saw
  "✅ Approved" and nothing happened; they now see "User no longer exists".

- **Config sentinels carry their own `Subject`** — `ErrStoreRequired` moved from
  subject `Config` to `UserStore`, and the new sentinels use `Database`,
  `BotToken` and `BotConfig`. `errorx` matches a sentinel by category *and*
  subject, so leaving them all on `Config` would have made
  `errors.Is(err, ErrDatabaseRequired)` true for every startup error. Code
  matching on the rendered message or subject string needs updating.

- **A missing `[bot] token` fails the boot** — `Bot` implements
  `configx.Validator`, so the token (and non-negative `pollTimeout` /
  `cacheExpiry`) is checked immediately after the config is read. The bot used to
  start and then fail with a bare 401 from Telegram on its first API call. No
  Option sets the token; to supply it from code, go through the config layer with
  `host.WithConfigValue("bot.token", token)`.

### Added

- **`ErrDatabaseRequired`** — installing `bojet.Module` without a database now
  names the missing wiring. The default store resolves its connection with
  `gormx.Lookup` rather than `gormx.Of`, which has panicked on an unregistered
  service since microjet v0.23.0.

- **`BojetModule.ModuleName`** — the host names the module `bojet` in its
  registration logs and errors instead of printing the Go type.

- **`ErrTokenRequired` / `ErrInvalidDuration`** — the sentinels `Validate`
  returns, so a caller can match a boot failure instead of parsing its message.

- **Tests** — the default `UserStore` is covered against an in-memory SQLite
  database (the not-found contract, and that a re-registration upserts profile
  fields without revoking an existing approval), alongside config validation and
  two host-level wiring tests that boot the module without a token and without a
  database.

### Fixed

- **`ErrStoreRequired` was unreachable** — `Bot.Init` resolved the store with
  `host.MustResolveService`, which panics on a missing service as of microjet
  v0.23.0, so the guard in `Bot.Start` could never run. Init now uses
  `host.ResolveService` and returns the typed error.

- **The user store closes after the bot** — `Module` registers the store before
  the bot, and microjet closes services in reverse registration order, so the bot
  stops polling Telegram before the store it reads from goes away. The previous
  order closed the store first.
