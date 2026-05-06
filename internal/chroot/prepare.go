package chroot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// prepare configures /etc/apt/sources.list, hostname, machine-id, the snapd
// block, and the initctl divert. It is the first step of the chroot phase.
func (p *Pipeline) prepare(ctx context.Context) error {
	if err := p.writeAptSources(); err != nil {
		return fmt.Errorf("write sources.list: %w", err)
	}
	if err := os.WriteFile("/etc/hostname", []byte(p.Config.Name+"\n"), 0o644); err != nil {
		return fmt.Errorf("write /etc/hostname: %w", err)
	}
	if err := p.Runner.Run(ctx, "apt-get", "update"); err != nil {
		return fmt.Errorf("apt-get update: %w", err)
	}
	if err := blockSnapd(); err != nil {
		return fmt.Errorf("block snapd: %w", err)
	}
	if err := p.Runner.Run(ctx, "apt-get", append([]string{"install", "-y"}, baseChrootBootstrap...)...); err != nil {
		return fmt.Errorf("install base chroot tools: %w", err)
	}
	if err := setupMachineID(ctx, p.Runner); err != nil {
		return fmt.Errorf("setup machine-id: %w", err)
	}
	if err := divertInitctl(ctx, p.Runner); err != nil {
		return fmt.Errorf("divert /sbin/initctl: %w", err)
	}
	return nil
}

var baseChrootBootstrap = []string{"libterm-readline-gnu-perl", "systemd-sysv"}

func (p *Pipeline) writeAptSources() error {
	body := fmt.Sprintf(`deb %s %s main restricted universe multiverse
deb-src %s %s main restricted universe multiverse

deb %s %s-security main restricted universe multiverse
deb-src %s %s-security main restricted universe multiverse

deb %s %s-updates main restricted universe multiverse
deb-src %s %s-updates main restricted universe multiverse
`,
		p.Config.Mirror, p.Config.Release,
		p.Config.Mirror, p.Config.Release,
		p.Config.Mirror, p.Config.Release,
		p.Config.Mirror, p.Config.Release,
		p.Config.Mirror, p.Config.Release,
		p.Config.Mirror, p.Config.Release,
	)
	return os.WriteFile("/etc/apt/sources.list", []byte(body), 0o644)
}

// blockSnapd installs an APT preference that pins snapd at -1, preventing it
// from being installed as a dependency.
func blockSnapd() error {
	if err := os.MkdirAll("/etc/apt/preferences.d", 0o755); err != nil {
		return err
	}
	body := "Package: snapd\nPin: release *\nPin-Priority: -1\n"
	return os.WriteFile("/etc/apt/preferences.d/nosnap.pref", []byte(body), 0o644)
}

// setupMachineID generates a fresh machine-id and links /var/lib/dbus to it.
func setupMachineID(ctx context.Context, r interface {
	Run(ctx context.Context, name string, args ...string) error
}) error {
	out, err := exec.CommandContext(ctx, "dbus-uuidgen").Output()
	if err != nil {
		return fmt.Errorf("dbus-uuidgen: %w", err)
	}
	if err := os.WriteFile("/etc/machine-id", out, 0o644); err != nil {
		return err
	}
	_ = os.Remove("/var/lib/dbus/machine-id")
	if err := os.MkdirAll("/var/lib/dbus", 0o755); err != nil {
		return err
	}
	return os.Symlink("/etc/machine-id", "/var/lib/dbus/machine-id")
}

// divertInitctl replaces /sbin/initctl with a symlink to /bin/true so package
// scripts that invoke initctl (a leftover from upstart-era Ubuntu) succeed
// silently inside the chroot.
func divertInitctl(ctx context.Context, r interface {
	Run(ctx context.Context, name string, args ...string) error
}) error {
	if err := r.Run(ctx, "dpkg-divert", "--local", "--rename", "--add", "/sbin/initctl"); err != nil {
		return err
	}
	_ = os.Remove("/sbin/initctl")
	return os.Symlink("/bin/true", "/sbin/initctl")
}
