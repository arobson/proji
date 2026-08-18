// Package prompttest provides a scripted prompt.Prompter test double for
// use by other packages' tests.
package prompttest

import "fmt"

// Fake is a scripted prompt.Prompter. Answers is consumed in call order by
// Ask and AskSecret; Confirms is consumed in call order by Confirm. Every
// prompt text seen is recorded in Prompts for assertions.
type Fake struct {
	Answers  []string
	Confirms []bool

	Prompts []string

	askCalls     int
	secretCalls  int
	confirmCalls int
}

func (f *Fake) Ask(prompt string) (string, error) {
	f.Prompts = append(f.Prompts, prompt)
	if f.askCalls+f.secretCalls >= len(f.Answers) {
		return "", fmt.Errorf("prompttest: no scripted answer for Ask(%q)", prompt)
	}
	answer := f.Answers[f.askCalls+f.secretCalls]
	f.askCalls++
	return answer, nil
}

func (f *Fake) AskSecret(prompt string) (string, error) {
	f.Prompts = append(f.Prompts, prompt)
	if f.askCalls+f.secretCalls >= len(f.Answers) {
		return "", fmt.Errorf("prompttest: no scripted answer for AskSecret(%q)", prompt)
	}
	answer := f.Answers[f.askCalls+f.secretCalls]
	f.secretCalls++
	return answer, nil
}

func (f *Fake) Confirm(prompt string, defaultYes bool) (bool, error) {
	f.Prompts = append(f.Prompts, prompt)
	if f.confirmCalls >= len(f.Confirms) {
		return defaultYes, nil
	}
	answer := f.Confirms[f.confirmCalls]
	f.confirmCalls++
	return answer, nil
}
