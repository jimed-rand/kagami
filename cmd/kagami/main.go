// Command kagami builds a vanilla Ubuntu live ISO. It is a Go port of the
// reference build pipeline shipped under references/build.sh.
//
// kagami runs in two phases:
//   - host (default): installs host tooling, debootstraps a chroot, re-execs
//     itself inside the chroot to run the chroot phase, then assembles the
//     final hybrid BIOS+UEFI ISO.
//   - chroot (KAGAMI_PHASE=chroot): runs APT setup, package install, calamares
//     config injection, image-tree population, and final cleanup.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"kagami/internal/build"
	"kagami/internal/chroot"
	"kagami/internal/config"
	"kagami/internal/host"
	"kagami/internal/resources"
	"kagami/internal/runner"
	"kagami/internal/ui"
	"kagami/internal/workspace"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "kagami: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Parse(os.Args[1:], cwd)
	if err != nil {
		return err
	}

	u := ui.New()
	phase := os.Getenv("KAGAMI_PHASE")
	if phase == "" {
		phase = "host"
	}

	switch phase {
	case "host":
		return runHost(u, cfg)
	case "chroot":
		return runChroot(u, cfg)
	default:
		return fmt.Errorf("invalid KAGAMI_PHASE %q (expected host|chroot)", phase)
	}
}

func runHost(u *ui.UI, cfg config.Config) error {
	u.Banner("Kagami — Vanilla Ubuntu ISO Builder")
	u.KV("Version", version)
	u.KV("Started at", time.Now().Format("2006-01-02 15:04:05 MST"))
	u.KV("Process", fmt.Sprintf("pid=%d, arch=%s", os.Getpid(), runtime.GOARCH))
	u.KV("References", cfg.ReferencesDir)

	cacheDir, _ := os.UserCacheDir()
	paths := workspace.Resolve(cfg.Workspace, cfg.RepoRoot, cacheDir)
	cfg.Workspace = paths.Root
	if notice := workspace.FormatRelocationNotice(paths, cfg.RepoRoot); notice != "" {
		u.Warn(notice)
	}
	u.KV("Workspace", paths.Root)

	if err := resolveInteractivePicks(u, &cfg); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	references, err := resources.Locate(cfg.ReferencesDir)
	if err != nil {
		return fmt.Errorf("locate references: %w", err)
	}

	printBuildSummary(u, cfg, paths)

	if !cfg.NoConfirm && ui.IsTTY() {
		if !u.Confirm("Start build now?", true) {
			u.Println()
			u.Info("Build cancelled by user. No changes were made.")
			return nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleanup := workspace.NewCleanupHandler(ctx, cancel)
	defer cleanup.Stop()

	r := runner.Runner{Verbose: true}
	cleanup.Add(func() {
		u.Warn("Aborting: removing workspace " + paths.Root)
		_ = r.RunPriv(context.Background(), "rm", "-rf", paths.Root)
	})

	pipeline := host.Pipeline{
		Config:     cfg,
		Runner:     r,
		UI:         u,
		Paths:      paths,
		References: references,
		Cleanup:    cleanup,
	}
	if err := pipeline.Execute(ctx); err != nil {
		return err
	}

	printBuildResult(u, cfg)
	return nil
}

func runChroot(u *ui.UI, cfg config.Config) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("the chroot phase must run as root")
	}
	u.Banner("Kagami chroot phase")

	// Apply minimal defaults for fields that can't be empty here. The host
	// phase passes everything via env vars, but we tolerate a manual chroot
	// invocation for debugging by filling in safe defaults.
	if cfg.Desktop == "" {
		cfg.Desktop = "gnome"
	}
	if cfg.Installer == "" {
		cfg.Installer = "calamares"
	}
	if cfg.LiveBootLabel == "" {
		cfg.LiveBootLabel = "Try Ubuntu without installing"
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Sanity-check that the required HWE/kernel mapping is usable.
	if _, err := build.KernelMetaPackage(cfg.Release, cfg.KernelFlavor); err != nil {
		return err
	}

	r := runner.Runner{Verbose: true}
	pipeline := chroot.Pipeline{Config: cfg, Runner: r, UI: u}
	return pipeline.Execute(context.Background())
}

// resolveInteractivePicks fills empty config fields by prompting the user (on
// a TTY) or returning a clear error otherwise.
func resolveInteractivePicks(u *ui.UI, cfg *config.Config) error {
	mustAsk := cfg.Interactive

	if mustAsk || cfg.Release == "" {
		choice, err := u.PickRelease()
		if err != nil {
			return err
		}
		cfg.Release = choice
	}
	if mustAsk || cfg.Installer == "" {
		choice, err := u.PickInstaller(cfg.Release)
		if err != nil {
			return err
		}
		cfg.Installer = choice
	}
	if mustAsk || cfg.KernelFlavor == "" {
		choice, err := u.PickKernel(build.HWEVersion(cfg.Release))
		if err != nil {
			return err
		}
		cfg.KernelFlavor = choice
	}
	if mustAsk || cfg.Desktop == "" {
		choice, err := u.PickDesktop()
		if err != nil {
			return err
		}
		cfg.Desktop = choice
	}
	if cfg.Desktop == "gnome" {
		if mustAsk || os.Getenv("TARGET_GNOME_INSTALL_RECOMMENDS") == "" {
			yes, err := u.PickGnomeRecommends()
			if err != nil {
				return err
			}
			cfg.GnomeRecommends = yes
		}
	} else {
		cfg.GnomeRecommends = false
	}
	return nil
}

func printBuildSummary(u *ui.UI, cfg config.Config, paths workspace.Paths) {
	u.Heading("Build configuration")
	releaseLabel := cfg.Release
	if v := cfg.ReleaseLTSVersion(); v != "" {
		releaseLabel = fmt.Sprintf("%s  (Ubuntu %s LTS)", cfg.Release, v)
	}
	u.KV("Ubuntu release", releaseLabel)
	kernelLabel := cfg.KernelFlavor
	if pkg, err := build.KernelMetaPackage(cfg.Release, cfg.KernelFlavor); err == nil {
		kernelLabel = fmt.Sprintf("%s  [%s]", cfg.KernelFlavor, pkg)
	}
	u.KV("Kernel", kernelLabel)
	u.KV("Desktop", cfg.Desktop)
	if cfg.Desktop == "gnome" {
		u.KV("  with Recommends", strconv.FormatBool(cfg.GnomeRecommends))
	}
	u.KV("Installer", cfg.Installer)
	u.KV("Target name", cfg.Name)
	u.KV("Mirror", cfg.Mirror)
	u.KV("Workspace", paths.Root)
	u.KV("Output ISO", filepath.Join(cfg.RepoRoot, cfg.Name+".iso"))
	u.Println()
}

func printBuildResult(u *ui.UI, cfg config.Config) {
	iso := filepath.Join(cfg.RepoRoot, cfg.Name+".iso")
	st, err := os.Stat(iso)
	if err != nil {
		u.Heading("Build finished")
		u.Info(fmt.Sprintf("No ISO produced at %s (this is expected for partial runs).", iso))
		return
	}

	u.Heading("Build complete")
	u.KV("ISO", iso)
	u.KV("Size", humanBytes(st.Size()))
	if data, err := os.ReadFile(iso + ".sha1"); err == nil {
		u.KV("SHA1", firstField(string(data)))
	}
	if data, err := os.ReadFile(iso + ".sha256"); err == nil {
		u.KV("SHA256", firstField(string(data)))
	}

	u.Println()
	u.Println("  Next steps:")
	u.Printf("    * Write to USB on Linux (replace /dev/sdX with your stick's device):\n")
	u.Printf("        sudo dd if=%q of=/dev/sdX bs=4M status=progress conv=fsync\n", iso)
	u.Printf("    * Write to USB on Windows: Rufus or balenaEtcher in ISO/DD image mode\n")
	u.Printf("    * Test-boot in QEMU (UEFI):\n")
	u.Printf("        qemu-system-x86_64 -m 4G -enable-kvm -cdrom %q \\\n", iso)
	u.Printf("            -bios /usr/share/OVMF/OVMF_CODE.fd\n")
	u.Println()
}

func firstField(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\n' || s[i] == '\t' {
			return s[:i]
		}
	}
	return s
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	suffixes := "KMGTPE"
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), suffixes[exp])
}
