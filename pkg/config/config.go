package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type Config struct {
	Distro     string           `json:"distro"`
	Release    string           `json:"release"`
	System     SystemConfig     `json:"system"`
	Repository RepositoryConfig `json:"repository"`
	Packages   PackageConfig    `json:"packages"`
	Installer  InstallerConfig  `json:"installer"`
	Network    NetworkConfig    `json:"network"`
	Security   SecurityConfig   `json:"security"`
}

type SystemConfig struct {
	Hostname     string `json:"hostname"`
	BlockSnapd   bool   `json:"block_snapd"`
	Architecture string `json:"architecture"`
	Locale       string `json:"locale"`
	Timezone     string `json:"timezone"`
}

type RepositoryConfig struct {
	Mirror          string           `json:"mirror"`
	UseProposed     bool             `json:"use_proposed"`
	AdditionalRepos []AdditionalRepo `json:"additional_repos"`
}

type AdditionalRepo struct {
	Name       string   `json:"name"`
	URI        string   `json:"uri"`
	Suite      string   `json:"suite"`
	Components []string `json:"components"`
	Key        string   `json:"key"`
}

type InstallerConfig struct {
	Type            string            `json:"type"`
	Slideshow       string            `json:"slideshow"`
	CalamaresConfig string            `json:"calamares_config"`
	Branding        BrandingConfig    `json:"branding"`
	Settings        map[string]string `json:"settings"`
}

type BrandingConfig struct {
	ProductName      string `json:"product_name"`
	ShortProductName string `json:"short_product_name"`
	ProductUrl       string `json:"product_url"`
	SupportUrl       string `json:"support_url"`
	Version          string `json:"version"`
}

type PackageConfig struct {
	Essential     []string `json:"essential"`
	Additional    []string `json:"additional"`
	Desktop       string   `json:"desktop"`
	RemoveList    []string `json:"remove_list"`
	Kernel        string   `json:"kernel"`
	EnableFlatpak bool     `json:"enable_flatpak"`
}

type NetworkConfig struct {
	Manager string `json:"manager"`
}

type SecurityConfig struct {
	EnableFirewall    bool     `json:"enable_firewall"`
	DisableServices   []string `json:"disable_services"`
	BlockSnapdForever bool     `json:"block_snapd_forever"`
}

func inferDistro(cfg *Config) string {
	debianCodenames := map[string]bool{
		"stable": true, "testing": true, "unstable": true, "sid": true,
		"bookworm": true, "trixie": true, "bullseye": true, "buster": true,
		"forky": true, "stretch": true, "jessie": true,
	}
	if debianCodenames[strings.ToLower(cfg.Release)] {
		return "debian"
	}
	if strings.Contains(cfg.Repository.Mirror, "debian.org") {
		return "debian"
	}
	return "ubuntu"
}

func NewDefaultConfig(release string) *Config {
	return &Config{
		Distro:  "ubuntu",
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
			Kernel: "linux-generic",
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

func (c *Config) Validate() error {
	if c.Distro == "" {
		c.Distro = inferDistro(c)
	}

	if c.Distro != "ubuntu" && c.Distro != "debian" {
		return errors.New("distro must be either 'ubuntu' or 'debian'")
	}

	if c.Distro == "ubuntu" {
		validUbuntu := map[string]bool{
			"focal": true, "jammy": true, "noble": true,
			"resolute": true, "devel": true,
		}
		if !validUbuntu[c.Release] {
			return errors.New("unsupported Ubuntu release; accepted values: focal, jammy, noble, resolute, devel")
		}
	}

	if c.System.Architecture != "amd64" && c.System.Architecture != "i386" && c.System.Architecture != "arm64" {
		return errors.New("unsupported architecture; accepted values: amd64, i386, arm64")
	}

	validDesktops := map[string]bool{
		"gnome": true, "kde": true, "xfce": true,
		"lxde": true, "lxqt": true, "mate": true, "none": true,
	}
	if !validDesktops[c.Packages.Desktop] {
		return errors.New("unsupported desktop environment identifier")
	}

	validInstallers := map[string]bool{
		"ubiquity":  true,
		"subiquity": true,
		"calamares": true,
	}
	if !validInstallers[c.Installer.Type] {
		return errors.New("unsupported installer type; accepted values: ubiquity, subiquity, calamares")
	}

	return nil
}

func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Distro == "" {
		cfg.Distro = inferDistro(&cfg)
	}

	return &cfg, nil
}

func (c *Config) SaveToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
