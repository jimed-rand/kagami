package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// stdinReader is a package-level reader so successive prompts share the same
// buffered stdin (avoids losing data across reads).
var stdinReader = bufio.NewReader(os.Stdin)

// IsTTY reports whether stdin appears to be an interactive terminal.
func IsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Confirm prompts the user with a y/n question. defaultYes selects whether
// pressing Enter (or no input) returns true.
func (u *UI) Confirm(prompt string, defaultYes bool) bool {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	for {
		fmt.Fprintf(u.Out, "  %s %s: ", prompt, hint)
		line, err := stdinReader.ReadString('\n')
		if err != nil {
			return defaultYes
		}
		ans := strings.TrimSpace(strings.ToLower(line))
		switch ans {
		case "":
			return defaultYes
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Fprintln(u.Out, "  Please answer y or n.")
		}
	}
}

// ReadLine reads a trimmed line from stdin with the given prompt.
func (u *UI) ReadLine(prompt string) (string, error) {
	fmt.Fprintf(u.Out, "  %s: ", prompt)
	line, err := stdinReader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
