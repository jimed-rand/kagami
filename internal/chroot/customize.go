package chroot

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"kagami/internal/build"
)

// customizeImage installs the desktop environment, common utilities, Brave,
// flatpak + flathub, and removes packages we don't want shipping in the live
// image. Mirrors customize_image in references/build.sh.
func (p *Pipeline) customizeImage(ctx context.Context) error {
	if err := blockSnapd(); err != nil {
		return fmt.Errorf("re-pin snapd: %w", err)
	}

	switch p.Config.Desktop {
	case "gnome":
		p.UI.Info("Desktop flavor: gnome")
		args := []string{"install", "-y"}
		if !p.Config.GnomeRecommends {
			p.UI.Info("GNOME package recommends: disabled (default lightweight mode)")
			args = append(args, "--no-install-recommends")
		} else {
			p.UI.Info("GNOME package recommends: enabled")
		}
		args = append(args, build.GnomeFull...)
		if err := p.Runner.Run(ctx, "apt-get", args...); err != nil {
			return fmt.Errorf("install gnome: %w", err)
		}
	case "xfce":
		p.UI.Info("Desktop flavor: xfce")
		args := append([]string{"install", "-y"}, build.XFCEFull...)
		if err := p.Runner.Run(ctx, "apt-get", args...); err != nil {
			return fmt.Errorf("install xfce: %w", err)
		}
	default:
		return fmt.Errorf("unsupported desktop %q", p.Config.Desktop)
	}

	if err := p.Runner.Run(ctx, "apt-get", append([]string{"install", "-y"}, build.PlymouthSet...)...); err != nil {
		return fmt.Errorf("install plymouth: %w", err)
	}
	if err := p.Runner.Run(ctx, "apt-get", append([]string{"install", "-y"}, build.NetTools...)...); err != nil {
		return fmt.Errorf("install net tools: %w", err)
	}

	if err := installBrave(ctx, p.Runner); err != nil {
		return err
	}

	if err := p.Runner.Run(ctx, "apt-get", append([]string{"install", "-y"}, build.CommonCLI...)...); err != nil {
		return fmt.Errorf("install common CLI tools: %w", err)
	}
	if err := installFlatpak(ctx, p.Runner); err != nil {
		return err
	}

	if p.Config.Desktop == "gnome" {
		if err := p.Runner.Run(ctx, "apt-get", append([]string{"install", "-y"}, build.GnomeStoreSet...)...); err != nil {
			return fmt.Errorf("install gnome software: %w", err)
		}
	}

	purgeArgs := append([]string{"purge", "-y", "--ignore-missing"}, build.CommonPurge...)
	if err := p.Runner.Run(ctx, "apt-get", purgeArgs...); err != nil {
		return fmt.Errorf("purge common packages: %w", err)
	}
	if p.Config.Desktop == "gnome" {
		if err := p.Runner.Run(ctx, "apt-get", append([]string{"purge", "-y", "--ignore-missing"}, build.GnomePurge...)...); err != nil {
			return fmt.Errorf("purge gnome games: %w", err)
		}
	}
	// Slideshow purges are best-effort (these may not be installed at all).
	_ = p.Runner.Run(ctx, "apt-get", append([]string{"purge", "-y", "--ignore-missing"}, build.SlideshowPurge...)...)
	return nil
}

func installBrave(ctx context.Context, r interface {
	Run(ctx context.Context, name string, args ...string) error
}) error {
	if err := os.MkdirAll("/usr/share/keyrings", 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/apt/sources.list.d", 0o755); err != nil {
		return err
	}
	if err := r.Run(ctx, "curl", "-fsSLo",
		"/usr/share/keyrings/brave-browser-archive-keyring.gpg",
		"https://brave-browser-apt-release.s3.brave.com/brave-browser-archive-keyring.gpg"); err != nil {
		return fmt.Errorf("download brave keyring: %w", err)
	}
	if err := r.Run(ctx, "curl", "-fsSLo",
		"/etc/apt/sources.list.d/brave-browser-release.sources",
		"https://brave-browser-apt-release.s3.brave.com/brave-browser.sources"); err != nil {
		return fmt.Errorf("download brave sources: %w", err)
	}
	if err := r.Run(ctx, "apt-get", "update"); err != nil {
		return fmt.Errorf("apt-get update after adding brave: %w", err)
	}
	if err := r.Run(ctx, "apt-get", "install", "-y", "brave-browser"); err != nil {
		return fmt.Errorf("install brave-browser: %w", err)
	}
	return nil
}

func installFlatpak(ctx context.Context, r interface {
	Run(ctx context.Context, name string, args ...string) error
}) error {
	if err := r.Run(ctx, "apt-get", "install", "-y", "flatpak"); err != nil {
		return fmt.Errorf("install flatpak: %w", err)
	}
	cmd := exec.CommandContext(ctx, "flatpak",
		"remote-add", "--if-not-exists", "--system", "flathub",
		"https://flathub.org/repo/flathub.flatpakrepo")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("flatpak remote-add flathub: %w", err)
	}
	return nil
}
