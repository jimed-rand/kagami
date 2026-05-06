// Package workspace resolves the build workspace directory and manages its
// lifecycle, including WSL/DrvFs detection (where debootstrap cannot reliably
// unpack tar archives) and signal-driven abort cleanup.
package workspace

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Paths describes the resolved workspace layout.
type Paths struct {
	// Root is the workspace parent (e.g. <repo>/workspace).
	Root string
	// Chroot is <Root>/chroot, the debootstrap target.
	Chroot string
	// Image is <Root>/image, the live image staging tree.
	Image string
	// Relocated is true when the workspace was moved off a Windows-mounted
	// filesystem (DrvFs / 9p / /mnt / /media) into the user's cache dir.
	Relocated bool
}

// Resolve determines the final workspace location.
//
// Priority:
//  1. explicitOverride (e.g. --workspace, KAGAMI_WORKSPACE) — used as a parent
//     and "/workspace" appended.
//  2. If repoRoot is on a Windows-mounted FS, fall back to
//     ~/.cache/kagami/workspace.
//  3. Otherwise <repoRoot>/workspace.
//
// home and cacheDir are passed in (rather than read from env) so callers can
// substitute deterministic values in tests.
func Resolve(explicitOverride, repoRoot, cacheDir string) Paths {
	if explicitOverride != "" {
		root := strings.TrimRight(explicitOverride, "/") + "/workspace"
		return paths(root, false)
	}
	if isWindowsMount(repoRoot) {
		root := filepath.Join(cacheDir, "kagami", "workspace")
		return paths(root, true)
	}
	return paths(filepath.Join(repoRoot, "workspace"), false)
}

func paths(root string, relocated bool) Paths {
	return Paths{
		Root:      root,
		Chroot:    filepath.Join(root, "chroot"),
		Image:     filepath.Join(root, "image"),
		Relocated: relocated,
	}
}

// isWindowsMount reports whether path lives on a filesystem that debootstrap
// cannot use (DrvFs / 9p) or under /mnt|/media (the typical WSL exposure).
func isWindowsMount(path string) bool {
	if strings.HasPrefix(path, "/mnt/") || strings.HasPrefix(path, "/media/") {
		return true
	}
	out, err := exec.Command("df", "-T", path).Output()
	if err != nil {
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return false
	}
	// Second line, second column is the FS type.
	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return false
	}
	fs := strings.ToLower(fields[1])
	return fs == "9p" || fs == "drvfs"
}

// FormatRelocationNotice returns the user-facing notice to print when the
// workspace was relocated off a Windows-mounted filesystem.
func FormatRelocationNotice(p Paths, repoRoot string) string {
	if !p.Relocated {
		return ""
	}
	return fmt.Sprintf(
		"Windows/WSL filesystem at %s is not safe for debootstrap; using Linux-native workspace: %s",
		repoRoot, p.Root,
	)
}
