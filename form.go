package bojet

import (
	"slices"
	"strings"

	"gopkg.in/telebot.v4"
)

// QuestionKind is the answer style of a Question.
type QuestionKind int

const (
	// TextQuestion accepts any free-text reply.
	TextQuestion QuestionKind = iota
	// SingleChoice presents Choices as buttons; the user picks exactly one.
	SingleChoice
	// MultiChoice presents Choices as toggle buttons plus a Done button; the
	// user may pick several. The selected Choice values are stored joined and
	// can be read back with Answers.Values.
	MultiChoice
)

// BackPolicy controls whether a question can be returned to via the Back button.
type BackPolicy int

const (
	// BackInherit defers to Form.AllowBack (the default).
	BackInherit BackPolicy = iota
	// BackAllow lets the user navigate back to (and re-answer) this question.
	BackAllow
	// BackDeny locks the question: once answered it cannot be re-asked, and it
	// acts as a checkpoint barrier — questions before it become unreachable too.
	BackDeny
)

// multiSep separates stored MultiChoice values. It is the ASCII unit separator,
// chosen so it won't collide with choice values.
const multiSep = "\x1f"

// Choice is a selectable option for SingleChoice / MultiChoice questions.
type Choice struct {
	// Value is what gets stored in Answers.
	Value string
	// Label is the button text shown to the user. If empty, Value is used.
	Label string
}

func (c Choice) label() string {
	if c.Label == "" {
		return c.Value
	}
	return c.Label
}

// Question is a single prompt in a form.
type Question struct {
	// Key is the Answers map key the answer is stored under. Must be unique
	// within a form.
	Key string
	// Prompt is the question text sent to the user.
	Prompt string
	// Kind selects the answer style (default TextQuestion).
	Kind QuestionKind
	// Choices are the options for SingleChoice / MultiChoice questions.
	Choices []Choice
	// Validate, if set, rejects an answer by returning an error; its message is
	// shown to the user and the question is re-asked. For choice questions it
	// receives the selected Choice value (or, for MultiChoice, the values
	// joined with the same separator Answers.Values understands).
	Validate func(answer string) error
	// Back overrides Form.AllowBack for this question.
	Back BackPolicy
}

// Answers holds collected answers keyed by Question.Key.
type Answers map[string]string

// Values returns the selected values of a MultiChoice answer (or a single-item
// slice for other kinds). Returns nil for an empty/absent answer.
func (a Answers) Values(key string) []string {
	v, ok := a[key]
	if !ok || v == "" {
		return nil
	}
	return strings.Split(v, multiSep)
}

// QuestionSource drives a form. The engine calls Next after each answer (and
// once at the start with empty answers). Returning (nil, nil) ends the form.
// Branching, skipping, and dynamic/database-backed question sets are all
// expressed by the logic inside Next.
type QuestionSource interface {
	Next(fc FormContext) (*Question, error)
}

// FormContext carries everything a QuestionSource needs to decide what to ask
// next: the bot Context (for BotUser, session data, database handles captured
// in closures) and all answers gathered so far.
type FormContext struct {
	Ctx     Context
	Answers Answers
}

// sourceFunc adapts a plain function to a QuestionSource.
type sourceFunc func(FormContext) (*Question, error)

func (f sourceFunc) Next(fc FormContext) (*Question, error) { return f(fc) }

// SourceFunc builds a QuestionSource from a function. This is the general
// escape hatch for dynamic forms: load the next question from a database, an
// API, or compute it from previous answers; return nil to finish.
func SourceFunc(fn func(fc FormContext) (*Question, error)) QuestionSource {
	return sourceFunc(fn)
}

// staticSource asks a fixed list of questions in order, skipping any already
// answered. It is replay-safe: progress is derived purely from Answers.
type staticSource struct{ qs []*Question }

func (s *staticSource) Next(fc FormContext) (*Question, error) {
	for _, q := range s.qs {
		if _, done := fc.Answers[q.Key]; !done {
			return q, nil
		}
	}
	return nil, nil
}

// StaticSource builds a QuestionSource that asks the given questions linearly.
// For branching or dynamically-sized forms, use SourceFunc instead.
func StaticSource(qs ...*Question) QuestionSource {
	return &staticSource{qs: qs}
}

// Form is a questionnaire: a question source plus a completion callback.
type Form struct {
	// ID identifies the form (useful for logging/persistence). Optional.
	ID string
	// Source supplies the questions. Required.
	Source QuestionSource
	// AllowBack is the default back behavior for questions that use BackInherit.
	AllowBack bool
	// OnComplete runs when the source is exhausted, with all collected answers.
	// After it returns the user is returned to their current menu.
	OnComplete func(c Context, a Answers) error
}

// formState is the in-progress runtime state of a form, held on the session.
type formState struct {
	form     *Form
	pending  *Question  // the question currently awaiting an answer
	answers  Answers    // answers collected so far
	history  []*Question // BackAllow questions answered so far, for back-navigation
	selected []string   // in-progress MultiChoice selections for pending
}

// resolveBack collapses BackInherit to a concrete allow/deny for a question.
func (f *Form) resolveBack(q *Question) BackPolicy {
	switch q.Back {
	case BackAllow:
		return BackAllow
	case BackDeny:
		return BackDeny
	default:
		if f.AllowBack {
			return BackAllow
		}
		return BackDeny
	}
}

// FormItem creates a page item that starts a form when pressed.
func FormItem(title string, form *Form) *PageItem {
	return &PageItem{title: title, action: &formAction{form: form}}
}

type formAction struct{ form *Form }

func (a *formAction) execute(c Context, b *Bot) error {
	return b.startForm(c, c.BotUser(), a.form)
}

// startForm initialises form state on the session and asks the first question.
func (b *Bot) startForm(c Context, u *User, f *Form) error {
	if f == nil || f.Source == nil {
		return nil
	}
	fs := &formState{form: f, answers: Answers{}}
	first, err := f.Source.Next(FormContext{Ctx: c, Answers: fs.answers})
	if err != nil {
		b.errorHandler(err, c)
		return c.Send(b.messages.GenericError, b.userKeyboard(u))
	}
	if first == nil {
		// Nothing to ask — treat as immediate completion.
		return b.completeForm(c, u, f, fs.answers)
	}
	fs.pending = first
	u.Session.input = fs
	b.saveSession(u)
	return b.askQuestion(c, fs)
}

// handle implements inputState: it processes the user's reply to the pending
// question and returns the next state — itself to stay in the form, or nil
// (set by completeForm) when the form finishes or is cancelled.
func (fs *formState) handle(c Context, b *Bot) (inputState, error) {
	u := c.BotUser()
	q := fs.pending
	text := c.Text()

	// Cancel from anywhere in the form.
	if text == b.messages.CancelButton {
		u.Session.input = nil
		b.deleteSession(u.ID)
		if u.Session.CurrentPage == nil {
			return nil, c.Send(b.messages.FormCancelled)
		}
		return nil, c.Send(b.messages.FormCancelled, b.userKeyboard(u))
	}

	// Back to the previous answered question.
	if text == PageBackText && len(fs.history) > 0 {
		prev := fs.history[len(fs.history)-1]
		fs.history = fs.history[:len(fs.history)-1]
		delete(fs.answers, prev.Key)
		fs.pending = prev
		fs.selected = nil
		b.saveSession(u)
		return fs, b.askQuestion(c, fs)
	}

	switch q.Kind {
	case MultiChoice:
		if text == b.messages.FormDoneButton {
			joined := strings.Join(fs.selected, multiSep)
			if q.Validate != nil {
				if err := q.Validate(joined); err != nil {
					return fs, c.Send(err.Error(), b.formKeyboard(fs))
				}
			}
			fs.answers[q.Key] = joined
			fs.selected = nil
			return fs.advance(c, b, q)
		}
		val, ok := matchChoice(q, text)
		if !ok {
			return fs, c.Send(b.messages.FormInvalidChoice, b.formKeyboard(fs))
		}
		fs.selected = toggle(fs.selected, val)
		b.saveSession(u)
		return fs, b.askQuestion(c, fs) // re-render with updated checkmarks

	case SingleChoice:
		val, ok := matchChoice(q, text)
		if !ok {
			return fs, c.Send(b.messages.FormInvalidChoice, b.formKeyboard(fs))
		}
		if q.Validate != nil {
			if err := q.Validate(val); err != nil {
				return fs, c.Send(err.Error(), b.formKeyboard(fs))
			}
		}
		fs.answers[q.Key] = val
		return fs.advance(c, b, q)

	default: // TextQuestion
		if q.Validate != nil {
			if err := q.Validate(text); err != nil {
				return fs, c.Send(err.Error(), b.formKeyboard(fs))
			}
		}
		fs.answers[q.Key] = text
		return fs.advance(c, b, q)
	}
}

// advance records back-history for the answered question, then asks the source
// for the next one (or completes the form). It returns the next inputState.
func (fs *formState) advance(c Context, b *Bot, answered *Question) (inputState, error) {
	u := c.BotUser()

	if fs.form.resolveBack(answered) == BackAllow {
		fs.history = append(fs.history, answered)
	} else {
		fs.history = nil // checkpoint barrier — nothing before this is reachable
	}

	next, err := fs.form.Source.Next(FormContext{Ctx: c, Answers: fs.answers})
	if err != nil {
		b.errorHandler(err, c)
		u.Session.input = nil
		b.deleteSession(u.ID)
		return nil, c.Send(b.messages.GenericError, b.userKeyboard(u))
	}
	if next == nil {
		// completeForm clears the input (and OnComplete may start a new one),
		// so read the resulting state after it runs.
		err := b.completeForm(c, u, fs.form, fs.answers)
		return u.Session.input, err
	}
	fs.pending = next
	fs.selected = nil
	b.saveSession(u)
	return fs, b.askQuestion(c, fs)
}

// completeForm clears the active form, runs OnComplete, and returns the user to
// their menu — unless OnComplete started a new conversation (e.g. a follow-up
// form), in which case that new state is left untouched.
func (b *Bot) completeForm(c Context, u *User, f *Form, ans Answers) error {
	u.Session.input = nil
	b.deleteSession(u.ID)
	if f.OnComplete != nil {
		if err := f.OnComplete(c, ans); err != nil {
			b.errorHandler(err, c)
			return c.Send(b.messages.GenericError, b.userKeyboard(u))
		}
	}
	if u.Session.input == nil && u.Session.CurrentPage != nil {
		return c.Send(u.Session.CurrentPage.Title, b.userKeyboard(u))
	}
	return nil
}

// askQuestion sends the pending question with its appropriate keyboard.
func (b *Bot) askQuestion(c Context, fs *formState) error {
	return c.Send(fs.pending.Prompt, b.formKeyboard(fs))
}

// formKeyboard builds the reply keyboard for the pending question: choice
// buttons (with checkmarks for selected MultiChoice options), a Done button for
// MultiChoice, plus Back (when reachable) and Cancel controls.
func (b *Bot) formKeyboard(fs *formState) *telebot.ReplyMarkup {
	q := fs.pending

	rm := &telebot.ReplyMarkup{ResizeKeyboard: true}
	var rows []telebot.Row

	switch q.Kind {
	case SingleChoice:
		for _, ch := range q.Choices {
			rows = append(rows, rm.Row(rm.Text(ch.label())))
		}
	case MultiChoice:
		for _, ch := range q.Choices {
			label := ch.label()
			if slices.Contains(fs.selected, ch.Value) {
				label = "✅ " + label
			}
			rows = append(rows, rm.Row(rm.Text(label)))
		}
		rows = append(rows, rm.Row(rm.Text(b.messages.FormDoneButton)))
	}

	var controls []telebot.Btn
	if len(fs.history) > 0 {
		controls = append(controls, rm.Text(PageBackText))
	}
	controls = append(controls, rm.Text(b.messages.CancelButton))
	rows = append(rows, rm.Row(controls...))

	rm.Reply(rows...)
	return rm
}

// matchChoice resolves pressed button text to a Choice value, tolerating the
// MultiChoice "✅ " selection prefix.
func matchChoice(q *Question, text string) (string, bool) {
	text = strings.TrimPrefix(text, "✅ ")
	for _, ch := range q.Choices {
		if text == ch.label() {
			return ch.Value, true
		}
	}
	return "", false
}

// toggle adds v if absent, removes it if present.
func toggle(s []string, v string) []string {
	for i, x := range s {
		if x == v {
			return append(s[:i], s[i+1:]...)
		}
	}
	return append(s, v)
}
