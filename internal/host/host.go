// Package host implements the host-side build pipeline: install host tools,
// debootstrap a minimal Ubuntu chroot, enter that chroot to run the chroot
// phase (re-execing kagami inside it), then package the resulting tree into a
// hybrid BIOS+UEFI ISO with checksums.
package host

import (
	"context"
	"fmt"

	"kagami/internal/config"
	"kagami/internal/resources"
	"kagami/internal/runner"
	"kagami/internal/ui"
	"kagami/internal/workspace"
)

// Pipeline is the host-phase driver.
type Pipeline struct {
	Config     config.Config
	Runner     runner.Runner
	UI         *ui.UI
	Paths      workspace.Paths
	References resources.References
	Cleanup    *workspace.CleanupHandler
}

type step struct {
	name string
	fn   func(context.Context) error
}

// Execute runs the full host pipeline: setup_host, debootstrap, run_chroot,
// build_iso. Step labels match the names used in references/build.sh.
func (p *Pipeline) Execute(ctx context.Context) error {
	steps := []step{
		{name: "setup_host", fn: p.setupHost},
		{name: "debootstrap", fn: p.debootstrap},
		{name: "run_chroot", fn: p.runChroot},
		{name: "build_iso", fn: p.buildISO},
	}
	for i, s := range steps {
		p.UI.Step(i+1, len(steps), s.name)
		if err := s.fn(ctx); err != nil {
			return fmt.Errorf("%s failed: %w", s.name, err)
		}
	}
	return nil
}
