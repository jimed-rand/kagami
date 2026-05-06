package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Parse turns argv into a validated Config. cwd is the process' working
// directory (used to resolve default paths for --references-dir and the
// workspace).
func Parse(args []string, cwd string) (Config, error) {
	fs := flag.NewFlagSet("kagami", flag.ContinueOnError)

	release := fs.String("release", envOr("TARGET_UBUNTU_VERSION", ""), "Target Ubuntu release: jammy|noble|resolute (omit to be prompted on a TTY)")
	mirror := fs.String("mirror", envOr("TARGET_UBUNTU_MIRROR", "http://archive.ubuntu.com/ubuntu/"), "Ubuntu package mirror")
	kernel := fs.String("kernel", envOr("TARGET_KERNEL_FLAVOR", ""), "Kernel flavor: generic|lowlatency (omit to be prompted on a TTY)")
	installer := fs.String("installer", envOr("TARGET_INSTALLER", ""), "Live installer: calamares|ubiquity (omit to be prompted on a TTY)")
	desktop := fs.String("desktop", envOr("TARGET_DESKTOP", ""), "Desktop flavor: gnome|xfce (omit to be prompted on a TTY)")
	gnomeRecommends := fs.String("gnome-recommends", envOr("TARGET_GNOME_INSTALL_RECOMMENDS", ""), "Install GNOME with Recommends: 0|1 (only meaningful when desktop=gnome)")
	name := fs.String("name", envOr("TARGET_NAME", "ubuntu"), "Output ISO base name (e.g. \"ubuntu\" produces ubuntu.iso)")
	liveBoot := fs.String("liveboot-label", envOr("GRUB_LIVEBOOT_LABEL", "Try Ubuntu without installing"), "GRUB live-boot menu entry label")
	workspace := fs.String("workspace", workspaceFromEnv(), "Workspace path (default: <repo>/workspace; auto-relocated on WSL/DrvFs)")
	referencesDir := fs.String("references-dir", filepath.Join(cwd, "references"), "Path to references directory (calamares config, etc.)")
	noConfirm := fs.Bool("yes", false, "Run without interactive confirmation")
	interactive := fs.Bool("interactive", false, "Force interactive prompts even when env/flags supply defaults (alias: -i)")
	fs.BoolVar(interactive, "i", false, "Force interactive prompts even when env/flags supply defaults")
	phase := fs.String("phase", envOr("KAGAMI_PHASE", "host"), "Pipeline phase: host|chroot")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	repoRoot := cwd

	cfg := Config{
		Release:         *release,
		Mirror:          *mirror,
		KernelFlavor:    *kernel,
		Installer:       *installer,
		Desktop:         *desktop,
		GnomeRecommends: *gnomeRecommends == "1",
		LiveBootLabel:   *liveBoot,
		Name:            *name,
		Workspace:       *workspace,
		ReferencesDir:   *referencesDir,
		RepoRoot:        repoRoot,
		NoConfirm:       *noConfirm,
		Interactive:     *interactive,
	}

	switch *phase {
	case "host", "chroot":
	default:
		return Config{}, fmt.Errorf("invalid --phase value %q (expected host|chroot)", *phase)
	}

	if os.Getenv("KAGAMI_PHASE") == "" {
		_ = os.Setenv("KAGAMI_PHASE", *phase)
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// workspaceFromEnv reads KAGAMI_WORKSPACE (preferred) or the legacy
// UBUNTU_VANILLA_WORKSPACE for a parent directory. Returns the empty string
// when neither is set so the caller can apply repo-root defaulting.
func workspaceFromEnv() string {
	if v := os.Getenv("KAGAMI_WORKSPACE"); v != "" {
		return v
	}
	if v := os.Getenv("UBUNTU_VANILLA_WORKSPACE"); v != "" {
		fmt.Fprintln(os.Stderr, "kagami: UBUNTU_VANILLA_WORKSPACE is deprecated; use KAGAMI_WORKSPACE instead.")
		return v
	}
	return ""
}
