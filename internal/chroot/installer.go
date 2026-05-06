package chroot

import (
	"context"
	"fmt"
)

// installInstaller installs Calamares (with custom config) or Ubiquity
// according to p.Config.Installer.
func (p *Pipeline) installInstaller(ctx context.Context) error {
	p.UI.Info("Live installer: " + p.Config.Installer)
	switch p.Config.Installer {
	case "calamares":
		// Install with --no-install-recommends so the calamares-settings-*
		// metapackages are not pulled in; configuration is provided
		// entirely from references/calamares.
		if err := p.Runner.Run(ctx, "apt-get", "install", "-y", "--no-install-recommends", "calamares"); err != nil {
			return fmt.Errorf("apt-get install calamares: %w", err)
		}
		return p.applyCalamaresConfig()
	case "ubiquity":
		if p.Config.Release != "jammy" {
			return fmt.Errorf("Ubiquity is supported only on jammy; got release=%q", p.Config.Release)
		}
		// --no-install-recommends prevents ubiquity-slideshow-ubuntu from
		// being pulled in.
		return p.Runner.Run(ctx, "apt-get", "install", "-y", "--no-install-recommends",
			"ubiquity", "ubiquity-frontend-gtk")
	}
	return fmt.Errorf("unsupported installer %q", p.Config.Installer)
}
