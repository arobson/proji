package cli_test

import (
	"errors"
	"testing"

	"github.com/arobson/proji/internal/cli"
)

func TestUpgrade_DelegatesToUpgrader(t *testing.T) {
	td := newTestDeps(t)
	u := &stubUpgrader{}
	td.Deps.NewUpgrader = func() cli.Upgrader { return u }

	if err := runCLI(t, td.Deps, "upgrade"); err != nil {
		t.Fatalf("upgrade: unexpected error: %v", err)
	}
	if !u.called {
		t.Error("Upgrader.Run should have been called")
	}
}

func TestUpgrade_PropagatesError(t *testing.T) {
	td := newTestDeps(t)
	td.Deps.NewUpgrader = func() cli.Upgrader { return &stubUpgrader{err: errors.New("download failed")} }

	err := runCLI(t, td.Deps, "upgrade")
	if err == nil {
		t.Fatal("upgrade: expected an error to propagate from the Upgrader")
	}
}
