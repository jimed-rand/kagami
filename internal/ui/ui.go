// Package ui provides terminal output helpers (colored banners, step headers,
// key-value rows, log lines) that mirror the visual style of
// references/build.sh.
//
// Color is automatically disabled when stdout is not a TTY, when TERM=dumb, or
// when NO_COLOR is set in the environment.
package ui

import (
	"fmt"
	"io"
	"os"
)

// UI writes formatted output to a paired stdout/stderr. Color is enabled per
// instance based on whether stdout looks like a usable TTY.
type UI struct {
	Out      io.Writer
	ErrOut   io.Writer
	UseColor bool
}

// New returns a UI bound to os.Stdout / os.Stderr with color auto-detected.
func New() *UI {
	return &UI{
		Out:      os.Stdout,
		ErrOut:   os.Stderr,
		UseColor: shouldUseColor(),
	}
}

func shouldUseColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func (u *UI) out() io.Writer {
	if u.Out != nil {
		return u.Out
	}
	return os.Stdout
}

func (u *UI) errw() io.Writer {
	if u.ErrOut != nil {
		return u.ErrOut
	}
	return os.Stderr
}

func (u *UI) c(code string) string {
	if !u.UseColor {
		return ""
	}
	return "\033[" + code + "m"
}

func (u *UI) reset() string {
	if !u.UseColor {
		return ""
	}
	return "\033[0m"
}

// Banner prints a thick double-bordered title banner.
func (u *UI) Banner(title string) {
	bar := "=================================================================="
	fmt.Fprintln(u.Out)
	fmt.Fprintf(u.Out, "%s%s%s\n", u.c("1;36"), bar, u.reset())
	fmt.Fprintf(u.Out, "%s  %s%s\n", u.c("1;36"), title, u.reset())
	fmt.Fprintf(u.Out, "%s%s%s\n", u.c("1;36"), bar, u.reset())
	fmt.Fprintln(u.Out)
}

// Heading prints a thin colored heading line ("--- title ---").
func (u *UI) Heading(title string) {
	fmt.Fprintln(u.Out)
	fmt.Fprintf(u.Out, "%s--- %s ---%s\n", u.c("1;34"), title, u.reset())
}

// Step prints a numbered step header ("[n/total] name").
func (u *UI) Step(n, total int, name string) {
	fmt.Fprintln(u.Out)
	fmt.Fprintf(u.Out, "%s[%d/%d] %s%s\n", u.c("1;33"), n, total, name, u.reset())
}

// OK prints a green "OK" status line.
func (u *UI) OK(msg string) {
	fmt.Fprintf(u.Out, "%s  OK    %s%s\n", u.c("32"), msg, u.reset())
}

// Warn prints a yellow "WARN" status line on stderr.
func (u *UI) Warn(msg string) {
	fmt.Fprintf(u.errw(), "%s  WARN  %s%s\n", u.c("33"), msg, u.reset())
}

// Err prints a red "ERROR" status line on stderr.
func (u *UI) Err(msg string) {
	fmt.Fprintf(u.errw(), "%s  ERROR %s%s\n", u.c("1;31"), msg, u.reset())
}

// Info prints a cyan "info" status line.
func (u *UI) Info(msg string) {
	fmt.Fprintf(u.Out, "%s  info  %s%s\n", u.c("36"), msg, u.reset())
}

// KV prints a key/value row using a 22-column key field.
func (u *UI) KV(key, value string) {
	fmt.Fprintf(u.Out, "    %-22s %s\n", key, value)
}

// Println passes through to the underlying writer.
func (u *UI) Println(args ...any) {
	fmt.Fprintln(u.Out, args...)
}

// Printf passes through to the underlying writer.
func (u *UI) Printf(format string, args ...any) {
	fmt.Fprintf(u.Out, format, args...)
}
