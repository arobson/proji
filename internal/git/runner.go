// Package git shells out to the real git binary. It is the only package in
// proji that knows how to invoke git; everything else in the codebase talks
// to it through Repo.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Runner runs a git subcommand in dir and returns its stdout/stderr.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) (stdout, stderr string, err error)
}

// ExitError is returned by ExecRunner when git exits non-zero. It carries
// enough detail for callers to distinguish specific failure modes (e.g. a
// non-fast-forward merge vs. a rebase conflict) from stderr/exit code.
type ExitError struct {
	Args     []string
	Stderr   string
	ExitCode int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("git %v: exit status %d: %s", e.Args, e.ExitCode, e.Stderr)
}

// ExecRunner runs git as a real subprocess.
type ExecRunner struct {
	// GitPath overrides the git binary to invoke. Defaults to "git" (resolved
	// via PATH) when empty.
	GitPath string
}

// NewExecRunner returns an ExecRunner that invokes "git" from PATH.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

func (r *ExecRunner) Run(ctx context.Context, dir string, args ...string) (string, string, error) {
	gitPath := r.GitPath
	if gitPath == "" {
		gitPath = "git"
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, gitPath, args...) // #nosec G204 -- args are fixed subcommands built internally, never raw user input
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.String(), stderr.String(), &ExitError{
				Args:     args,
				Stderr:   stderr.String(),
				ExitCode: exitErr.ExitCode(),
			}
		}
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}
