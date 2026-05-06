package chroot

import (
	"context"
	"fmt"
	"os"

	"kagami/internal/build"
)

// installPackages runs the install_pkg step from references/build.sh:
// apt upgrade, install the curated core list, install the kernel metapackage,
// install the live installer (calamares + custom config or ubiquity),
// customize the live image, then run apt cleanup.
func (p *Pipeline) installPackages(ctx context.Context) error {
	if err := p.Runner.Run(ctx, "apt-get", "-y", "upgrade"); err != nil {
		return fmt.Errorf("apt-get upgrade: %w", err)
	}

	if err := p.Runner.Run(ctx, "apt-get", append([]string{"install", "-y"}, build.CoreInstall...)...); err != nil {
		return fmt.Errorf("install core packages: %w", err)
	}

	kernelPkg, err := build.KernelMetaPackage(p.Config.Release, p.Config.KernelFlavor)
	if err != nil {
		return err
	}
	p.UI.Info("Installing kernel metapackage: " + kernelPkg)
	if err := p.Runner.Run(ctx, "apt-get", "install", "-y", kernelPkg); err != nil {
		return fmt.Errorf("install %s: %w", kernelPkg, err)
	}

	if err := p.installInstaller(ctx); err != nil {
		return err
	}
	if err := p.customizeImage(ctx); err != nil {
		return err
	}

	if err := p.Runner.Run(ctx, "apt-get", "autoremove", "-y"); err != nil {
		return fmt.Errorf("apt-get autoremove: %w", err)
	}
	if err := p.Runner.Run(ctx, "dpkg-reconfigure", "locales"); err != nil {
		return fmt.Errorf("dpkg-reconfigure locales: %w", err)
	}
	if err := writeNetworkManagerConfig(); err != nil {
		return fmt.Errorf("write NetworkManager.conf: %w", err)
	}
	if err := p.Runner.Run(ctx, "dpkg-reconfigure", "network-manager"); err != nil {
		return fmt.Errorf("dpkg-reconfigure network-manager: %w", err)
	}
	if err := p.Runner.Run(ctx, "apt-get", "clean", "-y"); err != nil {
		return fmt.Errorf("apt-get clean: %w", err)
	}
	return nil
}

func writeNetworkManagerConfig() error {
	body := `[main]
rc-manager=none
plugins=ifupdown,keyfile
dns=systemd-resolved

[ifupdown]
managed=false
`
	if err := os.MkdirAll("/etc/NetworkManager", 0o755); err != nil {
		return err
	}
	return os.WriteFile("/etc/NetworkManager/NetworkManager.conf", []byte(body), 0o644)
}
