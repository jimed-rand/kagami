package config

import (
	"errors"
	"fmt"
	"path/filepath"
)

var (
	ErrInvalidRelease   = errors.New("release must be jammy, noble, or resolute")
	ErrInvalidInstaller = errors.New("installer must be calamares or ubiquity")
	ErrInvalidDesktop   = errors.New("desktop must be gnome or xfce")
	ErrInvalidKernel    = errors.New("kernel flavor must be generic or lowlatency")
)

type Config struct {
	Release      string
	Mirror       string
	KernelFlavor string
	Installer    string
	Desktop      string

	// GnomeRecommends, when true, installs vanilla-gnome-desktop with Recommends.
	// Only meaningful when Desktop == "gnome".
	GnomeRecommends bool

	// LiveBootLabel is the GRUB live-boot menu entry label
	// (e.g. "Try Ubuntu without installing").
	LiveBootLabel string

	Name           string
	Workspace      string
	ReferencesDir  string
	RepoRoot       string

	NoConfirm   bool
	Interactive bool
}

func (c Config) Validate() error {
	switch c.Release {
	case "jammy", "noble", "resolute":
	default:
		return fmt.Errorf("%w: %q", ErrInvalidRelease, c.Release)
	}

	switch c.Installer {
	case "calamares", "ubiquity":
	default:
		return fmt.Errorf("%w: %q", ErrInvalidInstaller, c.Installer)
	}

	if c.Installer == "ubiquity" && c.Release != "jammy" {
		return errors.New("Ubiquity is only supported on Ubuntu 22.04 LTS (jammy)")
	}

	switch c.Desktop {
	case "gnome", "xfce":
	default:
		return fmt.Errorf("%w: %q", ErrInvalidDesktop, c.Desktop)
	}

	switch c.KernelFlavor {
	case "generic", "lowlatency":
	default:
		return fmt.Errorf("%w: %q", ErrInvalidKernel, c.KernelFlavor)
	}

	if c.Mirror == "" {
		return errors.New("Ubuntu mirror cannot be empty")
	}
	if c.Name == "" {
		return errors.New("target name cannot be empty")
	}
	return nil
}

func (c Config) WorkspaceChroot() string {
	return filepath.Join(c.Workspace, "chroot")
}

func (c Config) WorkspaceImage() string {
	return filepath.Join(c.Workspace, "image")
}

// ReleaseLTSVersion returns the marketing version (e.g. "24.04") for the
// configured release codename, or "" if the release is unknown.
func (c Config) ReleaseLTSVersion() string {
	switch c.Release {
	case "jammy":
		return "22.04"
	case "noble":
		return "24.04"
	case "resolute":
		return "26.04"
	}
	return ""
}
