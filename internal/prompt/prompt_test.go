package prompt_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/arobson/proji/internal/prompt"
)

func TestIOPrompter_Ask_ReturnsTrimmedLine(t *testing.T) {
	in := strings.NewReader("hello world  \n")
	out := &bytes.Buffer{}
	p := prompt.NewIOPrompter(in, out)

	got, err := p.Ask("Say something: ")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if got != "hello world" {
		t.Errorf("Ask() = %q, want %q", got, "hello world")
	}
	if out.String() != "Say something: " {
		t.Errorf("prompt not written to Out: got %q", out.String())
	}
}

func TestIOPrompter_Ask_MultipleCallsConsumeSequentialLines(t *testing.T) {
	in := strings.NewReader("first\nsecond\n")
	p := prompt.NewIOPrompter(in, &bytes.Buffer{})

	first, err := p.Ask("1: ")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	second, err := p.Ask("2: ")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if first != "first" || second != "second" {
		t.Errorf("got %q, %q; want %q, %q", first, second, "first", "second")
	}
}

func TestIOPrompter_Ask_EOFWithoutNewlineReturnsWhatWasRead(t *testing.T) {
	in := strings.NewReader("no newline")
	p := prompt.NewIOPrompter(in, &bytes.Buffer{})

	got, err := p.Ask("? ")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if got != "no newline" {
		t.Errorf("Ask() = %q, want %q", got, "no newline")
	}
}

func TestIOPrompter_Confirm(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		{"empty answer uses default true", "\n", true, true},
		{"empty answer uses default false", "\n", false, false},
		{"explicit yes", "y\n", false, true},
		{"explicit YES mixed case", "Yes\n", false, true},
		{"explicit no overrides default true", "n\n", true, false},
		{"garbage treated as no", "sure\n", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := prompt.NewIOPrompter(strings.NewReader(tt.input), &bytes.Buffer{})
			got, err := p.Confirm("Continue?", tt.defaultYes)
			if err != nil {
				t.Fatalf("Confirm() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Confirm() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIOPrompter_AskSecret_FallsBackToPlainReadForNonTTYInput(t *testing.T) {
	in := strings.NewReader("ghp_secret\n")
	out := &bytes.Buffer{}
	p := prompt.NewIOPrompter(in, out)

	got, err := p.AskSecret("Token: ")
	if err != nil {
		t.Fatalf("AskSecret() error = %v", err)
	}
	if got != "ghp_secret" {
		t.Errorf("AskSecret() = %q, want %q", got, "ghp_secret")
	}
}
