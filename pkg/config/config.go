package config

// NOTE for non-APT distribution users:
// If you are running Kagami on a non-APT system (e.g., Fedora, Arch, openSUSE),
// it is recommended to run it inside a Distrobox container (Ubuntu/Debian)
// with your home folder mapped to ensure proper file access and workspace management.

import (
	"encoding/json"
	"errors"
	"os"
)

// Config represents the build configuration
type Config struct {
	Release    string           `json:"release"`
	System     SystemConfig     `json:"system"`
	Repository RepositoryConfig `json:"repository"`
	Packages   PackageConfig    `json:"packages"`
	Installer  InstallerConfig  `json:"installer"`
	Network    NetworkConfig    `json:"network"`
	Security   SecurityConfig   `json:"security"`
}

// SystemConfig contains system-level configurations
type SystemConfig struct {
	Hostname     string `json:"hostname"`
	BlockSnapd   bool   `json:"block_snapd"`
	Architecture string `json:"architecture"`
	Locale       string `json:"locale"`
	Timezone     string `json:"timezone"`
}

// RepositoryConfig defines APT repository settings
type RepositoryConfig struct {
	Mirror          string           `json:"mirror"`
	UseProposed     bool             `json:"use_proposed"`
	AdditionalRepos []AdditionalRepo `json:"additional_repos"`
}

// AdditionalRepo represents an additional APT repository
type AdditionalRepo struct {
	Name       string   `json:"name"`
	URI        string   `json:"uri"`
	Suite      string   `json:"suite"`
	Components []string `json:"components"`
	Key        string   `json:"key"` // URL to GPG key or inline key
}

// InstallerConfig defines installer preferences
type InstallerConfig struct {
	Type            string            `json:"type"`             // ubiquity, subiquity, or calamares
	Slideshow       string            `json:"slideshow"`        // ubuntu, kubuntu, xubuntu, lubuntu, ubuntu-mate
	CalamaresConfig string            `json:"calamares_config"` // Path to custom Calamares configuration directory
	Settings        map[string]string `json:"settings"`         // Additional installer settings
}

// PackageConfig defines package installation preferences
type PackageConfig struct {
	Essential     []string `json:"essential"`
	Additional    []string `json:"additional"`
	Desktop       string   `json:"desktop"` // gnome, kde, xfce, lxde, mate, none
	RemoveList    []string `json:"remove_list"`
	EnableFlatpak bool     `json:"enable_flatpak"`
}

// NetworkConfig for network settings
type NetworkConfig struct {
	Manager string `json:"manager"` // network-manager, systemd-networkd
}

// SecurityConfig for security hardening
type SecurityConfig struct {
	EnableFirewall    bool     `json:"enable_firewall"`
	DisableServices   []string `json:"disable_services"`
	BlockSnapdForever bool     `json:"block_snapd_forever"`
}

// NewDefaultConfig creates a default configuration for the specified release
func NewDefaultConfig(release string) *Config {
	return &Config{
		Release: release,
		System: SystemConfig{
			Hostname:     "ubuntu-kagami",
			BlockSnapd:   true,
			Architecture: "amd64",
			Locale:       "en_US.UTF-8",
			Timezone:     "UTC",
		},
		Repository: RepositoryConfig{
			Mirror:          "http://archive.ubuntu.com/ubuntu/",
			UseProposed:     false,
			AdditionalRepos: []AdditionalRepo{},
		},
		Packages: PackageConfig{
			Essential: []string{
				"sudo",
				"ubuntu-standard",
				"casper",
				"discover",
				"laptop-detect",
				"os-prober",
				"network-manager",
				"net-tools",
				"wireless-tools",
				"wpagui",
				"locales",
				"grub-common",
				"grub-gfxpayload-lists",
				"grub-pc",
				"grub-pc-bin",
				"grub2-common",
				"grub-efi-amd64-signed",
				"shim-signed",
				"mtools",
				"binutils",
			},
			Additional: []string{
				"vim",
				"nano",
				"curl",
				"wget",
				"git",
				"htop",
			},
			Desktop: "none",
			RemoveList: []string{
				"ubuntu-advantage-tools",
				"ubuntu-report",
				"whoopsie",
				"apport",
				"popularity-contest",
			},
		},
		Installer: InstallerConfig{
			Type:      "ubiquity",
			Slideshow: "ubuntu",
		},
		Network: NetworkConfig{
			Manager: "network-manager",
		},
		Security: SecurityConfig{
			EnableFirewall:    false,
			BlockSnapdForever: true,
			DisableServices:   []string{},
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	validReleases := map[string]bool{
		// Ubuntu
		"focal":    true, // 20.04 LTS
		"jammy":    true, // 22.04 LTS
		"noble":    true, // 24.04 LTS
		"resolute": true, // 26.04 LTS (Upcoming)
		"devel":    true, // Rolling/Development
		// Debian
		"bookworm": true, // Stable
		"trixie":   true, // Testing
		"sid":      true, // Unstable
	}

	if !validReleases[c.Release] {
		return errors.New("invalid release. Must be one of: focal, jammy, noble, resolute, devel (Ubuntu) or bookworm, trixie, sid (Debian)")
	}

	if c.System.Architecture != "amd64" && c.System.Architecture != "i386" && c.System.Architecture != "arm64" {
		return errors.New("invalid architecture. Must be amd64, i386 or arm64")
	}

	validDesktops := map[string]bool{
		"gnome": true,
		"kde":   true,
		"xfce":  true,
		"lxde":  true,
		"lxqt":  true,
		"mate":  true,
		"none":  true,
	}

	if !validDesktops[c.Packages.Desktop] {
		return errors.New("invalid desktop environment")
	}

	validInstallers := map[string]bool{
		"ubiquity":  true,
		"subiquity": true,
		"calamares": true, // Added support for Calamares
	}

	if !validInstallers[c.Installer.Type] {
		return errors.New("invalid installer type. Must be ubiquity, subiquity, or calamares")
	}

	return nil
}

// Preset Repositories
const (
	RepoUbuntuArchive = "http://archive.ubuntu.com/ubuntu/"
)

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveToFile saves configuration to a JSON file
func (c *Config) SaveToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
