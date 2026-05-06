// Package resources resolves and verifies on-disk reference assets shipped
// alongside kagami (the references/ directory: calamares config, etc.).
package resources

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	kagamiembed "kagami"
)

// References describes the on-disk layout of the references/ directory. All
// paths are absolute.
type References struct {
	Root string
	// Ephemeral is true when Root points to a temporary staged copy extracted
	// from embedded assets.
	Ephemeral bool

	CalamaresRoot         string
	CalamaresSettings     string
	CalamaresModules      string
	CalamaresBranding     string // .../branding/ubuntu
	CalamaresBrandingDesc string
	CalamaresI18N         string // .../i18n/SUPPORTED  (may be missing)
}

// Locate builds a References for the given references/ directory and verifies
// that the required calamares assets exist. When referencesDir is missing or
// incomplete, Locate falls back to embedded references/calamares content.
func Locate(referencesDir string) (References, error) {
	if r, err := locateFromDir(referencesDir, false); err == nil {
		return r, nil
	}
	return locateFromEmbedded()
}

func locateFromDir(referencesDir string, ephemeral bool) (References, error) {
	r := References{
		Root:                  referencesDir,
		Ephemeral:             ephemeral,
		CalamaresRoot:         filepath.Join(referencesDir, "calamares"),
		CalamaresSettings:     filepath.Join(referencesDir, "calamares", "settings.conf"),
		CalamaresModules:      filepath.Join(referencesDir, "calamares", "modules"),
		CalamaresBranding:     filepath.Join(referencesDir, "calamares", "branding", "ubuntu"),
		CalamaresBrandingDesc: filepath.Join(referencesDir, "calamares", "branding", "ubuntu", "branding.desc"),
		CalamaresI18N:         filepath.Join(referencesDir, "calamares", "i18n", "SUPPORTED"),
	}
	required := []string{
		r.CalamaresSettings,
		r.CalamaresModules,
		r.CalamaresBrandingDesc,
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			return References{}, fmt.Errorf("missing required reference asset %q: %w", p, err)
		}
	}
	return r, nil
}

func locateFromEmbedded() (References, error) {
	tmpRoot, err := os.MkdirTemp("", "kagami-references-*")
	if err != nil {
		return References{}, fmt.Errorf("create temp references dir: %w", err)
	}
	if err := writeEmbeddedCalamares(tmpRoot); err != nil {
		_ = os.RemoveAll(tmpRoot)
		return References{}, err
	}
	return locateFromDir(tmpRoot, true)
}

func writeEmbeddedCalamares(root string) error {
	const prefix = "references/calamares"
	return fs.WalkDir(kagamiembed.CalamaresFS, prefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(prefix, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dst := filepath.Join(root, "calamares", rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		// Exclude shell scripts from embedded staging.
		if strings.HasSuffix(strings.ToLower(d.Name()), ".sh") {
			return nil
		}
		content, err := kagamiembed.CalamaresFS.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, content, 0o644)
	})
}

// RenderBrandingDesc reads branding.desc from disk and substitutes the
// `@VERSION@` and `@CODENAME@` placeholders, matching the templating performed
// by references/build.sh's apply_calamares_custom_config.
func (r References) RenderBrandingDesc(version, codename string) ([]byte, error) {
	raw, err := os.ReadFile(r.CalamaresBrandingDesc)
	if err != nil {
		return nil, fmt.Errorf("read branding.desc: %w", err)
	}
	out := strings.ReplaceAll(string(raw), "@VERSION@", version)
	out = strings.ReplaceAll(out, "@CODENAME@", codename)
	return []byte(out), nil
}
