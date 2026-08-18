// Package prompt provides a small abstraction over interactive terminal
// input, so command logic and its tests never talk to os.Stdin directly.
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Prompter asks the user questions and reads their answers.
type Prompter interface {
	// Ask prints prompt and returns the trimmed line the user typed.
	Ask(prompt string) (string, error)
	// AskSecret behaves like Ask but avoids echoing input when possible.
	AskSecret(prompt string) (string, error)
	// Confirm asks a yes/no question. An empty answer resolves to defaultYes.
	Confirm(prompt string, defaultYes bool) (bool, error)
}

// IOPrompter is the real, terminal-backed Prompter.
type IOPrompter struct {
	In  io.Reader
	Out io.Writer

	reader *bufio.Reader
}

// NewIOPrompter returns a Prompter reading from in and writing to out.
func NewIOPrompter(in io.Reader, out io.Writer) *IOPrompter {
	return &IOPrompter{In: in, Out: out}
}

func (p *IOPrompter) Ask(prompt string) (string, error) {
	if _, err := fmt.Fprint(p.Out, prompt); err != nil {
		return "", err
	}
	if p.reader == nil {
		p.reader = bufio.NewReader(p.In)
	}
	line, err := p.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// AskSecret reads input without echoing it when In is the process's real
// stdin and it's an interactive terminal; otherwise it falls back to a
// plain line read (e.g. when input is piped, as in tests and scripts).
func (p *IOPrompter) AskSecret(prompt string) (string, error) {
	if f, ok := p.In.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if _, err := fmt.Fprint(p.Out, prompt); err != nil {
			return "", err
		}
		data, err := term.ReadPassword(int(f.Fd()))
		if _, ferr := fmt.Fprintln(p.Out); ferr != nil {
			return "", ferr
		}
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	return p.Ask(prompt)
}

func (p *IOPrompter) Confirm(prompt string, defaultYes bool) (bool, error) {
	suffix := " [y/N] "
	if defaultYes {
		suffix = " [Y/n] "
	}
	answer, err := p.Ask(prompt + suffix)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(answer) {
	case "":
		return defaultYes, nil
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
