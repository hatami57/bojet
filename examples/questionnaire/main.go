// Questionnaire example: collecting answers through a multi-step form.
//
// It shows the two ways to drive a form:
//
//   - StaticSource — a fixed, linear list of questions (the "Feedback" form).
//   - SourceFunc   — a dynamic source whose next question (and total count) is
//     decided at runtime from previous answers or an external data source such
//     as a database (the "Survey" form). Branching and skipping live here.
//
// Questions can be free text, single choice, or multiple choice, and each may
// opt in or out of the Back button via BackPolicy.
package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hatami57/bojet"
	"github.com/hatami57/microjet/gormx/sqlite"
	"github.com/hatami57/microjet/host"
)

// --- A pretend questions table -------------------------------------------------
// In a real bot this would be your database. The dynamic survey loads its next
// question from here based on what the user has already answered.

type dbQuestion struct {
	Code    string
	Text    string
	Options []string // empty => free text
}

var questionBank = []dbQuestion{
	{Code: "role", Text: "What best describes you?", Options: []string{"Developer", "Designer", "Manager"}},
	{Code: "lang", Text: "Which language do you use most?"}, // developers only (see branching)
	{Code: "team", Text: "How big is your team?", Options: []string{"1-5", "6-20", "20+"}},
}

func main() {
	// --- Static, linear form -------------------------------------------------
	feedback := &bojet.Form{
		ID:        "feedback",
		AllowBack: true, // every question is re-answerable unless it says otherwise
		Source: bojet.StaticSource(
			&bojet.Question{
				Key:     "rating",
				Prompt:  "How would you rate the bot?",
				Kind:    bojet.SingleChoice,
				Choices: []bojet.Choice{{Value: "👍"}, {Value: "😐"}, {Value: "👎"}},
			},
			&bojet.Question{
				Key:    "email",
				Prompt: "Leave your email if you'd like a reply (or type 'skip'):",
				Kind:   bojet.TextQuestion,
				Validate: func(s string) error {
					if s == "skip" || strings.Contains(s, "@") {
						return nil
					}
					return errors.New("That doesn't look like an email. Try again or type 'skip'.")
				},
			},
			&bojet.Question{
				Key:     "topics",
				Prompt:  "Which areas should we improve? (pick any, then Done)",
				Kind:    bojet.MultiChoice,
				Choices: []bojet.Choice{{Value: "speed"}, {Value: "ui"}, {Value: "docs"}, {Value: "features"}},
			},
		),
		OnComplete: func(c bojet.Context, a bojet.Answers) error {
			return c.Send(fmt.Sprintf(
				"Thanks! 🙏\nRating: %s\nEmail: %s\nImprove: %s",
				a["rating"], a["email"], strings.Join(a.Values("topics"), ", "),
			))
		},
	}

	// --- Dynamic, DB-backed form with branching ------------------------------
	survey := &bojet.Form{
		ID: "survey",
		Source: bojet.SourceFunc(func(fc bojet.FormContext) (*bojet.Question, error) {
			for _, dbq := range questionBank {
				if _, done := fc.Answers[dbq.Code]; done {
					continue // already answered
				}
				// Branching: only developers get the language question.
				if dbq.Code == "lang" && fc.Answers["role"] != "Developer" {
					continue
				}
				return toQuestion(dbq), nil
			}
			return nil, nil // no more questions -> form completes
		}),
		OnComplete: func(c bojet.Context, a bojet.Answers) error {
			var b strings.Builder
			b.WriteString("Survey complete ✅\n")
			for _, dbq := range questionBank {
				if v, ok := a[dbq.Code]; ok {
					fmt.Fprintf(&b, "• %s: %s\n", dbq.Text, v)
				}
			}
			// Persist results to your store here.
			return c.Send(b.String())
		},
	}

	home := bojet.NewPage("🏠 Main Menu",
		bojet.FormItem("📝 Give feedback", feedback),
		bojet.FormItem("📊 Take the survey", survey),
	)

	host.MustNew().
		WithDatabase(sqlite.Driver()).
		WithModule(bojet.Module(
			bojet.WithPublicAccess(),
			bojet.WithHomePage(home),
		)).
		MustRun()
}

func toQuestion(dbq dbQuestion) *bojet.Question {
	q := &bojet.Question{Key: dbq.Code, Prompt: dbq.Text, Kind: bojet.TextQuestion}
	if len(dbq.Options) > 0 {
		q.Kind = bojet.SingleChoice
		for _, opt := range dbq.Options {
			q.Choices = append(q.Choices, bojet.Choice{Value: opt})
		}
	}
	return q
}
