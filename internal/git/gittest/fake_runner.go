// Package gittest provides a scripted git.Runner test double.
package gittest

import (
	"context"
	"fmt"
	"strings"
)

// Call records one invocation of FakeRunner.Run.
type Call struct {
	Dir  string
	Args []string
}

// Response describes what FakeRunner should return for calls whose args
// Match reports true for. Responses are consulted in order; the first
// matching, not-yet-exhausted Response is used.
type Response struct {
	Match          func(args []string) bool
	Stdout, Stderr string
	Err            error

	consumed bool
}

// FakeRunner is a git.Runner test double driven by a scripted list of
// Responses, recording every call it receives for assertions.
type FakeRunner struct {
	Responses []*Response
	Calls     []Call
}

// Run implements git.Runner.
func (f *FakeRunner) Run(_ context.Context, dir string, args ...string) (string, string, error) {
	f.Calls = append(f.Calls, Call{Dir: dir, Args: append([]string(nil), args...)})

	for _, resp := range f.Responses {
		if resp.consumed || resp.Match == nil || !resp.Match(args) {
			continue
		}
		resp.consumed = true
		return resp.Stdout, resp.Stderr, resp.Err
	}
	return "", "", fmt.Errorf("gittest: no scripted response for git %s", strings.Join(args, " "))
}

// MatchPrefix returns a Match func that matches when args starts with the
// given prefix words, e.g. MatchPrefix("merge", "--ff-only").
func MatchPrefix(prefix ...string) func(args []string) bool {
	return func(args []string) bool {
		if len(args) < len(prefix) {
			return false
		}
		for i, p := range prefix {
			if args[i] != p {
				return false
			}
		}
		return true
	}
}
