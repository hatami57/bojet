package bojet

import (
	"reflect"
	"testing"
)

func TestStaticSourceProgression(t *testing.T) {
	src := StaticSource(
		&Question{Key: "a", Prompt: "A?"},
		&Question{Key: "b", Prompt: "B?"},
	)
	ans := Answers{}

	q, err := src.Next(FormContext{Answers: ans})
	if err != nil || q == nil || q.Key != "a" {
		t.Fatalf("first: got %v, %v; want question a", q, err)
	}

	ans["a"] = "x"
	q, _ = src.Next(FormContext{Answers: ans})
	if q == nil || q.Key != "b" {
		t.Fatalf("second: got %v; want question b", q)
	}

	ans["b"] = "y"
	q, _ = src.Next(FormContext{Answers: ans})
	if q != nil {
		t.Fatalf("exhausted: got %v; want nil", q)
	}
}

func TestResolveBack(t *testing.T) {
	tests := []struct {
		name      string
		allowBack bool
		policy    BackPolicy
		want      BackPolicy
	}{
		{"inherit-allow", true, BackInherit, BackAllow},
		{"inherit-deny", false, BackInherit, BackDeny},
		{"explicit-allow", false, BackAllow, BackAllow},
		{"explicit-deny", true, BackDeny, BackDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Form{AllowBack: tt.allowBack}
			got := f.resolveBack(&Question{Back: tt.policy})
			if got != tt.want {
				t.Fatalf("got %v; want %v", got, tt.want)
			}
		})
	}
}

func TestMatchChoice(t *testing.T) {
	q := &Question{Choices: []Choice{
		{Value: "dev", Label: "Developer"},
		{Value: "plain"}, // label falls back to value
	}}

	if v, ok := matchChoice(q, "Developer"); !ok || v != "dev" {
		t.Fatalf("label match: got %q, %v", v, ok)
	}
	if v, ok := matchChoice(q, "✅ Developer"); !ok || v != "dev" {
		t.Fatalf("multichoice prefix match: got %q, %v", v, ok)
	}
	if v, ok := matchChoice(q, "plain"); !ok || v != "plain" {
		t.Fatalf("value-as-label match: got %q, %v", v, ok)
	}
	if _, ok := matchChoice(q, "nope"); ok {
		t.Fatalf("unexpected match for unknown text")
	}
}

func TestToggleAndAnswersValues(t *testing.T) {
	var sel []string
	sel = toggle(sel, "a")
	sel = toggle(sel, "b")
	sel = toggle(sel, "a") // removes a
	if !reflect.DeepEqual(sel, []string{"b"}) {
		t.Fatalf("toggle: got %v; want [b]", sel)
	}

	a := Answers{"topics": "go" + multiSep + "rust"}
	if got := a.Values("topics"); !reflect.DeepEqual(got, []string{"go", "rust"}) {
		t.Fatalf("Values: got %v; want [go rust]", got)
	}
	if got := a.Values("missing"); got != nil {
		t.Fatalf("Values(missing): got %v; want nil", got)
	}
}
