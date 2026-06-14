# bojet

A small framework for building Telegram bots in Go, on top of
[telebot.v4](https://github.com/tucnak/telebot). It gives you menu-style page
navigation, user registration/approval, admin messaging, scheduled broadcasts,
and a **questionnaire form engine** for collecting multi-step answers with
branching.

## Install

```bash
go get github.com/hatami57/bojet
go get github.com/hatami57/microjet/host github.com/hatami57/microjet/gormx/sqlite
```

## Quick start

bojet is a [microjet](https://github.com/hatami57/microjet) module. The host
builds the app, opens the database, and drives the bot's lifecycle; you describe
the whole bot declaratively with `Module(opts...)`.

```go
package main

import (
	"github.com/hatami57/bojet"
	"github.com/hatami57/microjet/gormx/sqlite"
	"github.com/hatami57/microjet/host"
)

func main() {
	home := bojet.NewPage("🏠 Main Menu",
		bojet.ActionItem("👋 Say hi", func(c bojet.Context) error {
			return c.Send("Hello, " + c.BotUser().FullName())
		}),
	)

	host.MustNew().
		WithDatabase(sqlite.Driver()).
		WithModule(bojet.Module(
			bojet.WithAdmins(123456789),
			bojet.WithHomePage(home),
		)).
		MustRun()
}
```

The Telegram token and the database path come from configuration, not code.
Provide a `config.toml` next to the binary (or set the matching `APP_*`
environment variables):

```toml
[bot]
token = "YOUR_BOT_TOKEN" # or set APP_BOT_TOKEN

[database]
name = "./bot.db"
```

The bot stores its users in the app's database via the default SQLite
`UserStore`, which `Module` registers for you. To use a different backend,
register your own `UserStore` service with the app.

By default new users register by sharing their phone number and must be approved
by an admin before they can use the bot. See [Access mode](#access-mode) to make
the bot public instead.

## Access mode

You choose whether the bot is gated behind admin approval or open to everyone:

```go
// Default: admin approval. Users share their phone number and an admin must
// approve them (Approve/Reject buttons) before they can use the bot.
bojet.Module(bojet.WithAdmins(123456789), ...)

// Public: open to all. Senders are provisioned (created and persisted as
// confirmed) on first contact — no phone number, no approval step.
bojet.Module(bojet.WithPublicAccess(), ...)
```

`WithPublicAccess()` is shorthand for `WithRegistrationFlow(&bojet.NoRegistrationFlow{})`.
Public users are stored as confirmed, so they're reachable via `Broadcast`. For
custom logic (allowlists, external SSO, etc.) implement the `RegistrationFlow`
interface yourself; implement the optional `UserProvisioner` extension if your
flow should create users on first contact rather than reject unknown senders.

## Pages

A `Page` is a menu screen rendered as a reply keyboard. Items are one of:

- `NavItem(title, page)` — navigate to a sub-page (with automatic Back button).
- `ActionItem(title, handler)` — run a handler when pressed.
- `FormItem(title, form)` — start a questionnaire (see below).

```go
home := bojet.NewPage("🏠 Main Menu",
	bojet.NavItem("📊 Stats", statsPage),
	bojet.ActionItem("🕐 Server time", func(c bojet.Context) error {
		return c.Send(time.Now().Format(time.RFC1123))
	}),
	bojet.FormItem("📝 Take the survey", survey),
)
```

See `examples/` for runnable static, dynamic, and complex page setups.

## Forms (questionnaires)

A `Form` collects a sequence of answers. Each question is free text, single
choice, or multiple choice, and the **next** question is decided at runtime —
so you can branch, skip, or load questions from a database with a
nondeterministic count.

### Anatomy

```go
type Form struct {
	ID         string            // identifies the form (logging/persistence)
	Source     QuestionSource    // supplies the questions (required)
	AllowBack  bool              // default Back behavior for questions
	OnComplete func(c Context, a Answers) error // runs when the source is exhausted
}

type Question struct {
	Key      string                  // Answers map key (unique within a form)
	Prompt   string                  // text sent to the user
	Kind     QuestionKind            // TextQuestion | SingleChoice | MultiChoice
	Choices  []Choice                // options for choice questions
	Validate func(answer string) error // reject + re-ask on error
	Back     BackPolicy              // override Form.AllowBack for this question
}

type Choice struct {
	Value string // stored in Answers
	Label string // button text (falls back to Value if empty)
}
```

The engine sends each question, captures and validates the reply, stores it
under `Question.Key`, then asks the source for the next one until the source
returns `nil` — at which point `OnComplete` runs and the user is returned to
their menu. A **Cancel** button is always shown; a **Back** button appears when
the previous question is reachable.

Start a form from a page (`FormItem`) or from any handler with
`c.StartForm(form)`.

### Question sources

The set of questions is **pull-based**: after each answer the engine calls
`Source.Next(fc)` with the answers gathered so far. Returning `(nil, nil)`
finishes the form.

```go
type QuestionSource interface {
	Next(fc FormContext) (*Question, error)
}

type FormContext struct {
	Ctx     Context // BotUser(), session data, DB handles via closures
	Answers Answers // everything answered so far
}
```

Two adapters cover most needs:

- **`StaticSource(qs...)`** — asks a fixed list in order (replay-safe; progress
  is derived purely from the answers collected).
- **`SourceFunc(fn)`** — the dynamic escape hatch: branch, skip, or load the
  next question from a database. Branching logic lives here.

#### Static, linear form

```go
feedback := &bojet.Form{
	ID:        "feedback",
	AllowBack: true,
	Source: bojet.StaticSource(
		&bojet.Question{
			Key:     "rating",
			Prompt:  "How would you rate the bot?",
			Kind:    bojet.SingleChoice,
			Choices: []bojet.Choice{{Value: "👍"}, {Value: "😐"}, {Value: "👎"}},
		},
		&bojet.Question{
			Key:    "email",
			Prompt: "Leave your email (or type 'skip'):",
			Kind:   bojet.TextQuestion,
			Validate: func(s string) error {
				if s == "skip" || strings.Contains(s, "@") {
					return nil
				}
				return errors.New("That doesn't look like an email.")
			},
		},
		&bojet.Question{
			Key:     "topics",
			Prompt:  "Which areas should we improve? (pick any, then Done)",
			Kind:    bojet.MultiChoice,
			Choices: []bojet.Choice{{Value: "speed"}, {Value: "ui"}, {Value: "docs"}},
		},
	),
	OnComplete: func(c bojet.Context, a bojet.Answers) error {
		return c.Send(fmt.Sprintf("Thanks! Rating %s, improve: %s",
			a["rating"], strings.Join(a.Values("topics"), ", ")))
	},
}
```

#### Dynamic, database-backed form with branching

`SourceFunc` lets you decide the next question from previous answers and load it
from anywhere. The total number of questions need not be known up front.

```go
survey := &bojet.Form{
	ID: "survey",
	Source: bojet.SourceFunc(func(fc bojet.FormContext) (*bojet.Question, error) {
		for _, dbq := range questionBank { // e.g. a SELECT against your DB
			if _, done := fc.Answers[dbq.Code]; done {
				continue // already answered
			}
			// Branch: only developers get the language question.
			if dbq.Code == "lang" && fc.Answers["role"] != "Developer" {
				continue
			}
			return toQuestion(dbq), nil
		}
		return nil, nil // no more questions -> form completes
	}),
	OnComplete: func(c bojet.Context, a bojet.Answers) error {
		// persist a, notify, etc.
		return c.Send("Survey complete ✅")
	},
}
```

### Reading answers

`Answers` is a `map[string]string` keyed by `Question.Key`. Text and single
choice store a single string; multiple choice stores the selected `Choice`
values, which you read back with `Answers.Values`:

```go
a["rating"]            // "👍"
a.Values("topics")     // []string{"speed", "docs"}
```

### Back navigation

`Form.AllowBack` sets the default; each question can override it with `Back`:

| `BackPolicy`  | Effect                                                             |
|---------------|--------------------------------------------------------------------|
| `BackInherit` | Use `Form.AllowBack` (the default).                                |
| `BackAllow`   | The user can go Back to and re-answer this question.               |
| `BackDeny`    | Locked: cannot be re-asked, and acts as a **checkpoint barrier** — questions before it become unreachable too. |

Use `BackDeny` for answers that are final (a confirmation, an irreversible
branch). Pressing Back pops the last reachable question, clears its answer, and
re-asks it.

### Session scratch data

Each user has a `Session` with a free-form `Data map[string]any` scratch space,
separate from form answers. Access it from any handler:

```go
c.SessionSet("cart", []string{"a", "b"})
v, ok := c.SessionGet("cart")
m := c.SessionData()
```

> Note: these persist for the life of the session and are distinct from
> telebot's per-update `Get`/`Set`. The scratch bag is in-memory only.

### Persistence across cache expiry

In-progress forms are stored in a `SessionStore` so they survive the in-memory
user-cache expiry. The default is `MemorySessionStore` (in process memory, lost
on restart); the store only holds sessions for users who are mid-form and
self-cleans on completion or cancel.

```go
bojet.Module(
	bojet.WithSessionStore(bojet.NewMemorySessionStore()), // default
	// bojet.WithSessionStore(nil),                        // disable persistence
)
```

Implement the `SessionStore` interface to back sessions with a database and
resume forms across restarts.

A complete runnable example with both a static and a database-backed form lives
in [`examples/questionnaire`](examples/questionnaire/main.go).

## Customizing messages

All user-facing strings — including the shared `Cancel` button (used by forms
and the contact-admin prompt), the form `Done` button, and validation prompts —
are overridable via `WithMessages`. Only non-empty fields override the defaults:

```go
bojet.Module(
	bojet.WithMessages(bojet.Messages{
		CancelButton:   "Annuleren",
		FormDoneButton: "Klaar",
	}),
)
```

## Other features

Everything is configured through `Module` options:

- `WithHandler(endpoint, fn)` — register custom commands, buttons, or telebot events.
- `WithSchedule(cronExpr, fn)` / `WithScheduledBroadcast(cronExpr, msg)` — recurring jobs.
- Lifecycle hooks: `WithOnUserRegistered`, `WithOnUserApproved`, `WithOnUserRejected`.
- `WithProxy`, `WithPollTimeout`, `WithCacheExpiry`, `WithErrorHandler`, `WithMessages`.

Settings in the `[bot]` config section (`token`, `proxyUrl`, `pollTimeout`,
`cacheExpiry`, `adminIds`, `contactAdmin`) are read automatically; matching
options override them. The bot logs through the host's configured logger.
