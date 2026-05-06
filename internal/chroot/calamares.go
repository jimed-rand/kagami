package chroot

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// applyCalamaresConfig installs the project's custom Calamares configuration
// from /root/calamares-config (copied in by the host phase) into
// /etc/calamares, templating branding.desc with the active release.
//
// Mirrors apply_calamares_custom_config in references/build.sh.
func (p *Pipeline) applyCalamaresConfig() error {
	const src = "/root/calamares-config"

	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("calamares config not found in chroot at %s (host did not stage it): %w", src, err)
	}
	settings := filepath.Join(src, "settings.conf")
	if _, err := os.Stat(settings); err != nil {
		return fmt.Errorf("calamares config missing settings.conf: %w", err)
	}

	if err := os.MkdirAll("/etc/calamares/modules", 0o755); err != nil {
		return err
	}
	if err := copyFile(settings, "/etc/calamares/settings.conf"); err != nil {
		return fmt.Errorf("copy settings.conf: %w", err)
	}
	if err := copyDirContents(filepath.Join(src, "modules"), "/etc/calamares/modules"); err != nil {
		return fmt.Errorf("copy modules: %w", err)
	}

	if i18n := filepath.Join(src, "i18n", "SUPPORTED"); fileExists(i18n) {
		if err := os.MkdirAll("/usr/share/i18n", 0o755); err != nil {
			return err
		}
		if fileExists("/usr/share/i18n/SUPPORTED") {
			if err := copyFile("/usr/share/i18n/SUPPORTED", "/usr/share/i18n/SUPPORTED.kagami-backup"); err != nil {
				return fmt.Errorf("backup stock SUPPORTED: %w", err)
			}
		}
		if err := copyFile(i18n, "/usr/share/i18n/SUPPORTED"); err != nil {
			return fmt.Errorf("install custom SUPPORTED: %w", err)
		}
	}

	// Copy branding assets, then template branding.desc.
	if err := os.MkdirAll("/etc/calamares/branding/ubuntu", 0o755); err != nil {
		return err
	}
	if err := copyDirContents(filepath.Join(src, "branding", "ubuntu"), "/etc/calamares/branding/ubuntu"); err != nil {
		return fmt.Errorf("copy branding: %w", err)
	}

	version := p.Config.ReleaseLTSVersion()
	if version == "" {
		return fmt.Errorf("no marketing version known for release %q", p.Config.Release)
	}
	rawPath := filepath.Join(src, "branding", "ubuntu", "branding.desc")
	rendered, err := renderBranding(rawPath, version, p.Config.Release)
	if err != nil {
		return fmt.Errorf("render branding.desc: %w", err)
	}
	return os.WriteFile("/etc/calamares/branding/ubuntu/branding.desc", rendered, 0o644)
}

func renderBranding(path, version, codename string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := strings.ReplaceAll(string(raw), "@VERSION@", version)
	out = strings.ReplaceAll(out, "@CODENAME@", codename)
	return []byte(out), nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	st, err := os.Stat(src)
	if err == nil {
		_ = os.Chmod(dst, st.Mode())
	}
	return out.Sync()
}

// copyDirContents shells out to `cp -a src/. dst/` to preserve attributes,
// symlinks, and ownership identically to references/build.sh.
func copyDirContents(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("cp", "-a", src+"/.", dst+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
