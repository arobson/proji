package cli_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/arobson/proji/internal/cli"
)

func TestInit_GitAlreadyInstalled_SkipsInstallStep(t *testing.T) {
	td := newTestDeps(t)
	b := &stubBootstrapper{}
	td.Deps.NewBootstrapper = func() cli.GitBootstrapper { return b }

	if err := runCLI(t, td.Deps, "init"); err != nil {
		t.Fatalf("init: unexpected error: %v", err)
	}
	if b.installCalled {
		t.Error("InstallGit should not be called when git is already on PATH")
	}
	if !b.configureCalled {
		t.Error("ConfigureGit should always be called")
	}
	if !strings.Contains(td.Out.String(), "git is already installed") {
		t.Errorf("output = %q, want a skip message", td.Out.String())
	}
}

func TestInit_GitMissing_InstallsThenConfigures(t *testing.T) {
	td := newTestDeps(t)
	td.Deps.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	b := &stubBootstrapper{}
	td.Deps.NewBootstrapper = func() cli.GitBootstrapper { return b }

	if err := runCLI(t, td.Deps, "init"); err != nil {
		t.Fatalf("init: unexpected error: %v", err)
	}
	if !b.installCalled {
		t.Error("InstallGit should be called when git is missing")
	}
	if !b.configureCalled {
		t.Error("ConfigureGit should be called after a successful install")
	}
}

func TestInit_InstallFails_SkipsConfigure(t *testing.T) {
	td := newTestDeps(t)
	td.Deps.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	b := &stubBootstrapper{installErr: errors.New("user declined")}
	td.Deps.NewBootstrapper = func() cli.GitBootstrapper { return b }

	err := runCLI(t, td.Deps, "init")
	if err == nil {
		t.Fatal("init: expected an error when InstallGit fails")
	}
	if b.configureCalled {
		t.Error("ConfigureGit should not run after a failed install")
	}
}

func TestInit_ConfigureFails(t *testing.T) {
	td := newTestDeps(t)
	b := &stubBootstrapper{configureErr: errors.New("could not set identity")}
	td.Deps.NewBootstrapper = func() cli.GitBootstrapper { return b }

	err := runCLI(t, td.Deps, "init")
	if err == nil {
		t.Fatal("init: expected an error when ConfigureGit fails")
	}
}
