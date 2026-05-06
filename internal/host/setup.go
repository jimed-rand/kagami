package host

import (
	"context"
	"fmt"
	"strings"

	"kagami/internal/build"
)

// setupHost installs host-side tooling (debootstrap, squashfs-tools, xorriso)
// and prepares a clean workspace tree.
func (p *Pipeline) setupHost(ctx context.Context) error {
	if err := p.Runner.RunPriv(ctx, "apt-get", "update"); err != nil {
		return fmt.Errorf("apt-get update: %w", err)
	}
	args := append([]string{"install", "-y"}, build.HostTools...)
	if err := p.Runner.RunPriv(ctx, "apt-get", args...); err != nil {
		return fmt.Errorf("install host tools (%s): %w", strings.Join(build.HostTools, " "), err)
	}
	if err := p.Runner.RunPriv(ctx, "rm", "-rf", p.Paths.Root); err != nil {
		return fmt.Errorf("clean workspace %q: %w", p.Paths.Root, err)
	}
	if err := p.Runner.RunPriv(ctx, "mkdir", "-p", p.Paths.Chroot); err != nil {
		return fmt.Errorf("create chroot dir %q: %w", p.Paths.Chroot, err)
	}
	return nil
}

// debootstrap creates the minimal Ubuntu rootfs at <workspace>/chroot.
func (p *Pipeline) debootstrap(ctx context.Context) error {
	p.UI.Info("Bootstrapping " + p.Config.Release + " into " + p.Paths.Chroot + " (this can take a few minutes)")
	return p.Runner.RunPriv(ctx,
		"debootstrap",
		"--arch=amd64",
		"--variant=minbase",
		p.Config.Release,
		p.Paths.Chroot,
		p.Config.Mirror,
	)
}
