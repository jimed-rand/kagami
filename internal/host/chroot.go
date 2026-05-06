package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// runChroot enters the bootstrapped chroot, copies kagami + the references/
// directory into it, then re-execs kagami inside the chroot in
// KAGAMI_PHASE=chroot mode. Bind mounts are torn down afterward.
func (p *Pipeline) runChroot(ctx context.Context) error {
	if err := p.enterChroot(ctx); err != nil {
		return err
	}
	// Always tear down on exit even if the body fails so we don't leave
	// stray bind mounts blocking workspace removal.
	p.Cleanup.Add(func() { _ = p.exitChroot(context.Background()) })
	defer func() { _ = p.exitChroot(context.Background()) }()

	if err := p.installKagamiBinary(ctx); err != nil {
		return err
	}
	if err := p.installCalamaresAssets(ctx); err != nil {
		return err
	}

	env := []string{
		"DEBIAN_FRONTEND=" + envOr("DEBIAN_FRONTEND", "readline"),
		"HOME=/root",
		"LC_ALL=C",
		"KAGAMI_PHASE=chroot",
		"TARGET_UBUNTU_VERSION=" + p.Config.Release,
		"TARGET_UBUNTU_MIRROR=" + p.Config.Mirror,
		"TARGET_KERNEL_FLAVOR=" + p.Config.KernelFlavor,
		"TARGET_INSTALLER=" + p.Config.Installer,
		"TARGET_DESKTOP=" + p.Config.Desktop,
		"TARGET_GNOME_INSTALL_RECOMMENDS=" + boolBit(p.Config.GnomeRecommends),
		"TARGET_NAME=" + p.Config.Name,
		"GRUB_LIVEBOOT_LABEL=" + p.Config.LiveBootLabel,
	}

	return p.Runner.Chroot(ctx, p.Paths.Chroot, env, "/root/kagami", "--phase=chroot", "--yes")
}

func (p *Pipeline) enterChroot(ctx context.Context) error {
	if err := p.Runner.RunPriv(ctx, "mount", "--bind", "/dev", filepath.Join(p.Paths.Chroot, "dev")); err != nil {
		return fmt.Errorf("mount --bind /dev: %w", err)
	}
	if err := p.Runner.RunPriv(ctx, "mount", "--bind", "/run", filepath.Join(p.Paths.Chroot, "run")); err != nil {
		return fmt.Errorf("mount --bind /run: %w", err)
	}
	mounts := []struct {
		fstype string
		target string
	}{
		{"proc", "/proc"},
		{"sysfs", "/sys"},
		{"devpts", "/dev/pts"},
	}
	for _, m := range mounts {
		// Use chroot to mount inside, mirroring references/build.sh.
		if err := p.Runner.RunPriv(ctx, "chroot", p.Paths.Chroot, "mount", "none", "-t", m.fstype, m.target); err != nil {
			return fmt.Errorf("mount %s inside chroot: %w", m.fstype, err)
		}
	}
	return nil
}

// exitChroot unmounts the chroot bind/special mounts. Errors are non-fatal
// (the caller may be running this as part of a teardown sequence).
func (p *Pipeline) exitChroot(ctx context.Context) error {
	targets := []string{
		filepath.Join(p.Paths.Chroot, "dev/pts"),
		filepath.Join(p.Paths.Chroot, "proc"),
		filepath.Join(p.Paths.Chroot, "sys"),
		filepath.Join(p.Paths.Chroot, "run"),
		filepath.Join(p.Paths.Chroot, "dev"),
	}
	for _, t := range targets {
		_ = p.Runner.RunPriv(ctx, "umount", "-l", t)
	}
	return nil
}

func (p *Pipeline) installKagamiBinary(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate kagami binary: %w", err)
	}
	dst := filepath.Join(p.Paths.Chroot, "root", "kagami")
	if err := p.Runner.RunPriv(ctx, "cp", exe, dst); err != nil {
		return fmt.Errorf("copy kagami into chroot: %w", err)
	}
	if err := p.Runner.RunPriv(ctx, "chmod", "0755", dst); err != nil {
		return fmt.Errorf("chmod kagami in chroot: %w", err)
	}
	return nil
}

func (p *Pipeline) installCalamaresAssets(ctx context.Context) error {
	dst := filepath.Join(p.Paths.Chroot, "root", "calamares-config")
	_ = p.Runner.RunPriv(ctx, "rm", "-rf", dst)
	if _, err := os.Stat(p.References.CalamaresRoot); err != nil {
		return nil
	}
	// Recreate parent and copy contents into it (matches `cp -a src dst`
	// where dst doesn't exist).
	if err := p.Runner.RunPriv(ctx, "mkdir", "-p", filepath.Dir(dst)); err != nil {
		return err
	}
	return p.Runner.RunPriv(ctx, "cp", "-a", p.References.CalamaresRoot, dst)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func boolBit(b bool) string {
	if b {
		return "1"
	}
	return strconv.Itoa(0)
}
