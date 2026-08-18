package setup

import (
	"context"
	"os"
	"os/exec"
)

// CommandRunner runs a system command to completion, with its stdio
// connected to the real terminal (so sudo password prompts, package
// manager output, and ssh-keygen prompts are visible to and answered by
// the person running proji).
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

// ExecCommandRunner runs commands as real subprocesses.
type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- args are fixed subcommands built internally, never raw user input
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
