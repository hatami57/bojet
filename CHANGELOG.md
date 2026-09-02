# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Releases before 0.7.0 predate this file; see the git history for those.

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
