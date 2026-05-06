// Package runner executes external commands with consistent stdout/stderr
// wiring and optional verbose echo of the command being run. It also offers
// helpers for running as root (auto-sudo) and for running commands inside a
// chroot.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Runner is the shared command executor.
type Runner struct {
	// Verbose, when true, echoes the command line to Stdout before running.
	Verbose bool
	// Stdout / Stderr default to the process' stdio when nil.
	Stdout io.Writer
	Stderr io.Writer
}

func (r Runner) outOrDefault() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}
	return os.Stdout
}

func (r Runner) errOrDefault() io.Writer {
	if r.Stderr != nil {
		return r.Stderr
	}
	return os.Stderr
}

// Run executes name+args, streaming stdio and echoing the command line in
// verbose mode.
func (r Runner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = r.outOrDefault()
	cmd.Stderr = r.errOrDefault()
	if r.Verbose {
		fmt.Fprintf(r.outOrDefault(), "==> %s\n", strings.Join(append([]string{name}, args...), " "))
	}
	return cmd.Run()
}

// RunPriv runs a command as root, prepending sudo when the caller is not
// already root.
func (r Runner) RunPriv(ctx context.Context, name string, args ...string) error {
	if os.Geteuid() == 0 {
		return r.Run(ctx, name, args...)
	}
	full := append([]string{name}, args...)
	return r.Run(ctx, "sudo", full...)
}

// Bash runs `bash -lc script` (used for short multi-step shell snippets where
// a Go reimplementation would be unnecessarily verbose).
func (r Runner) Bash(ctx context.Context, script string) error {
	return r.Run(ctx, "bash", "-lc", script)
}

// BashPriv is Bash via sudo when not already root.
func (r Runner) BashPriv(ctx context.Context, script string) error {
	return r.RunPriv(ctx, "bash", "-lc", script)
}

// Chroot runs argv inside a chroot rooted at chrootPath, with the given env
// (KEY=VALUE strings). It implicitly runs as root via sudo if necessary.
//
// extraEnv is added on top of the inherited environment so callers can pass
// just the variables they care about.
func (r Runner) Chroot(ctx context.Context, chrootPath string, extraEnv []string, argv ...string) error {
	if len(argv) == 0 {
		return fmt.Errorf("chroot: empty argv")
	}
	cmdArgs := []string{chrootPath, "/usr/bin/env"}
	cmdArgs = append(cmdArgs, extraEnv...)
	cmdArgs = append(cmdArgs, argv...)
	return r.RunPriv(ctx, "chroot", cmdArgs...)
}

// CommandOutput runs name+args and returns trimmed stdout. Stderr is
// suppressed.
func (r Runner) CommandOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
