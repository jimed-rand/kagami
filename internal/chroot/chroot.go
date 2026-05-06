// Package chroot implements the chroot-side build phase: APT setup, package
// install, calamares config injection, image-tree population, and final
// cleanup. It runs only when KAGAMI_PHASE=chroot (re-execed by the host
// phase inside the bootstrapped chroot).
package chroot

import (
	"context"
	"fmt"

	"kagami/internal/config"
	"kagami/internal/runner"
	"kagami/internal/ui"
)

// Pipeline drives the chroot phase.
type Pipeline struct {
	Config config.Config
	Runner runner.Runner
	UI     *ui.UI
}

type step struct {
	name string
	fn   func(context.Context) error
}

// Execute runs the chroot phase. Step names mirror references/build.sh's
// CHROOT_CMD array.
func (p *Pipeline) Execute(ctx context.Context) error {
	steps := []step{
		{name: "chroot_prepare", fn: p.prepare},
		{name: "install_pkg", fn: p.installPackages},
		{name: "build_image", fn: p.buildImage},
		{name: "finish_up", fn: p.finishUp},
	}
	for i, s := range steps {
		p.UI.Step(i+1, len(steps), s.name)
		if err := s.fn(ctx); err != nil {
			return fmt.Errorf("%s failed: %w", s.name, err)
		}
	}
	return nil
}
