package chroot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// finishUp performs the final cleanup inside the chroot: truncate the
// machine-id (so it's regenerated on first boot of the live system), restore
// the initctl divert, and clear /tmp + bash history. Mirrors finish_up in
// references/build.sh.
func (p *Pipeline) finishUp(ctx context.Context) error {
	if err := os.Truncate("/etc/machine-id", 0); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("truncate /etc/machine-id: %w", err)
	}

	_ = os.Remove("/sbin/initctl")
	if err := p.Runner.Run(ctx, "dpkg-divert", "--rename", "--remove", "/sbin/initctl"); err != nil {
		return fmt.Errorf("dpkg-divert remove /sbin/initctl: %w", err)
	}

	if err := clearDir("/tmp"); err != nil {
		return fmt.Errorf("clear /tmp: %w", err)
	}
	_ = os.Remove("/root/.bash_history")
	return nil
}

// clearDir removes the contents of dir but not the directory itself.
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
