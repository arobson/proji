// Package setuptest provides a scripted setup.CommandRunner test double.
package setuptest

import "context"

// Call records one invocation of FakeCommandRunner.Run.
type Call struct {
	Name string
	Args []string
}

// FakeCommandRunner is a setup.CommandRunner test double. ErrFor, if set,
// determines the error returned for each call; otherwise Err (which may be
// nil) is returned for every call. OnCall, if set, runs after recording
// each call and before returning its error — tests use it to simulate a
// command's real-world side effects (e.g. ssh-keygen creating key files).
type FakeCommandRunner struct {
	ErrFor func(name string, args []string) error
	Err    error
	OnCall func(name string, args []string)

	Calls []Call
}

func (f *FakeCommandRunner) Run(_ context.Context, name string, args ...string) error {
	f.Calls = append(f.Calls, Call{Name: name, Args: append([]string(nil), args...)})
	if f.OnCall != nil {
		f.OnCall(name, args)
	}
	if f.ErrFor != nil {
		return f.ErrFor(name, args)
	}
	return f.Err
}
