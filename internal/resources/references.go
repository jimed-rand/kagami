// Package resources resolves and verifies on-disk reference assets shipped
// alongside kagami (the references/ directory: calamares config, etc.).
package resources

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// References describes the on-disk layout of the references/ directory. All
// paths are absolute.
type References struct {
	Root string

	CalamaresRoot     string
	CalamaresSettings string
	CalamaresModules  string
	CalamaresBranding string // .../branding/ubuntu
	CalamaresBrandingDesc string
	CalamaresI18N     string // .../i18n/SUPPORTED  (may be missing)
}

// Locate builds a References for the given references/ directory and verifies
// that the required calamares assets exist.
func Locate(referencesDir string) (References, error) {
	r := References{
		Root:                  referencesDir,
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
