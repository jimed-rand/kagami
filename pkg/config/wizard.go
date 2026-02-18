package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"kagami/pkg/system"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WizardOption represents a selectable option in the wizard
type WizardOption struct {
	Key         string
	Label       string
	Description string
}

// RunWizard runs the interactive configuration wizard and returns a Config
func RunWizard() (*Config, string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("----------------------------------------------------------------")
	fmt.Println("               Kagami Configuration Wizard")
	fmt.Println("           Interactive ISO Build Configuration")
	fmt.Println("----------------------------------------------------------------")
	fmt.Println()

	// --- Distribution ---------------------------------------------------
	fmt.Println("[ Distribution ]")
	distOptions := []WizardOption{
		{"ubuntu", "Ubuntu", "Ubuntu-based ISO (LTS or Rolling)"},
		{"debian", "Debian", "Debian-based ISO (Stable, Testing, or Sid)"},
	}
	distChoice := promptChoice(reader, "Select base distribution:", distOptions)
	isDebian := distChoice == "debian"
	fmt.Println()

	// --- Release --------------------------------------------------------
	fmt.Println("[ Release ]")
	var releaseOptions []WizardOption
	if isDebian {
		releaseOptions = []WizardOption{
			{"trixie", "Trixie", "Debian 13 (Stable)"},
			{"testing", "Testing", "Debian Testing"},
			{"sid", "Unstable", "Debian Unstable (Rolling)"},
		}
	} else {
		releaseOptions = []WizardOption{
			{"noble", "Noble Numbat", "Ubuntu 24.04 LTS"},
			{"resolute", "Resolute", "Ubuntu 26.04 LTS (Upcoming)"},
			{"jammy", "Jammy Jellyfish", "Ubuntu 22.04 LTS"},
			{"devel", "Development", "Ubuntu Rolling/Development"},
		}
	}
	release := promptChoice(reader, "Select release:", releaseOptions)
	fmt.Println()

	// --- Build Mode -----------------------------------------------------
	fmt.Println("[ Build Mode ]")
	modeOptions := []WizardOption{
		{"desktop", "Desktop ISO", "Full desktop environment with installer"},
		{"minimal", "Minimal Installer", "Minimal live environment (like ALCI) - boots into WM with Calamares installer only"},
	}
	buildMode := promptChoice(reader, "Select build mode:", modeOptions)
	isMinimal := buildMode == "minimal"
	fmt.Println()

	// --- Desktop / WM Selection -----------------------------------------
	var desktop string
	var additionalPkgs []string

	if isMinimal {
		// Minimal installer mode (ALCI-style)
		fmt.Println("[ Window Manager (Minimal Installer) ]")
		fmt.Println("  The minimal installer boots into a lightweight WM with")
		fmt.Println("  Calamares auto-started. Similar to ALCI (Arch Linux")
		fmt.Println("  Calamares Installer).")
		fmt.Println("----------------------------------------------------------------")

		wmOptions := []WizardOption{
			{"openbox", "Openbox", "Lightweight stacking WM (recommended, ~50MB)"},
			{"i3", "i3", "Tiling window manager (~20MB)"},
			{"xfce4-minimal", "Xfce4 (Minimal)", "Xfce4 panel + Desktop only (~150MB)"},
		}
		wmChoice := promptChoice(reader, "Select window manager for live environment:", wmOptions)

		desktop = "none"
		additionalPkgs = getMinimalWMPackages(wmChoice, isDebian)
	} else {
		fmt.Println("[ Desktop Environment ]")
		var desktopOptions []WizardOption

		if isDebian {
			desktopOptions = []WizardOption{
				{"xfce", "Xfce", "Lightweight and fast (task-xfce-desktop)"},
				{"gnome", "GNOME", "Modern full-featured desktop (task-gnome-desktop)"},
				{"kde", "KDE Plasma", "Feature-rich and customizable (task-kde-desktop)"},
				{"lxqt", "LXQt", "Lightweight Qt-based desktop (task-lxqt-desktop)"},
				{"mate", "MATE", "Traditional GNOME 2 fork (task-mate-desktop)"},
				{"lxde", "LXDE", "Very lightweight GTK desktop (task-lxde-desktop)"},
				{"none", "None (Vanilla)", "Manually specify packages in additional list"},
			}
		} else {
			desktopOptions = []WizardOption{
				{"gnome", "GNOME", "Ubuntu GNOME Desktop"},
				{"kde", "KDE Plasma", "Kubuntu Desktop"},
				{"xfce", "Xfce", "Xubuntu Desktop"},
				{"mate", "MATE", "Ubuntu MATE Desktop"},
				{"lxqt", "LXQt", "Lubuntu Desktop (LXQt)"},
				{"lxde", "LXDE", "Lubuntu Desktop (LXDE)"},
				{"none", "None (Vanilla)", "Manually specify packages in additional list"},
			}
		}
		desktop = promptChoice(reader, "Select desktop environment:", desktopOptions)
	}
	fmt.Println()

	// --- Installer ------------------------------------------------------
	var installerType string
	var slideshow string

	if isMinimal {
		// Minimal mode always uses Calamares
		installerType = "calamares"
		fmt.Println("[ Installer ]")
		fmt.Println("  Minimal installer mode uses Calamares (auto-configured).")
		fmt.Println("----------------------------------------------------------------")
	} else if isDebian {
		// Debian defaults to Calamares
		installerType = "calamares"
		fmt.Println("[ Installer ]")
		fmt.Println("  Debian uses Calamares as the default installer.")
		fmt.Println("----------------------------------------------------------------")
	} else {
		// Ubuntu: choice between ubiquity and calamares
		fmt.Println("[ Installer ]")
		installerOptions := []WizardOption{
			{"ubiquity", "Ubiquity", "Traditional Ubuntu installer (GTK-based)"},
			{"calamares", "Calamares", "Universal installer framework (modern, distro-agnostic)"},
		}
		installerType = promptChoice(reader, "Select installer:", installerOptions)

		if installerType == "ubiquity" {
			slideshowOptions := []WizardOption{
				{"ubuntu", "Ubuntu", "Default Ubuntu slideshow"},
				{"kubuntu", "Kubuntu", "KDE Plasma slideshow"},
				{"xubuntu", "Xubuntu", "Xfce slideshow"},
				{"lubuntu", "Lubuntu", "LXQt slideshow"},
				{"ubuntu-mate", "Ubuntu MATE", "MATE slideshow"},
			}
			slideshow = promptChoice(reader, "Select installer slideshow:", slideshowOptions)
		}
	}
	fmt.Println()

	// --- System Settings ------------------------------------------------
	fmt.Println("[ System Settings ]")
	defaultHostname := "ubuntu-kagami"
	if isDebian {
		defaultHostname = "debian-kagami"
	}
	if isMinimal {
		defaultHostname += "-installer"
	}
	hostname := promptString(reader, fmt.Sprintf("Hostname [%s]:", defaultHostname), defaultHostname)
	fmt.Println()

	// --- Architecture ---------------------------------------------------
	archOptions := []WizardOption{
		{"amd64", "amd64", "64-bit x86 (most common)"},
		{"arm64", "arm64", "64-bit ARM"},
	}
	arch := promptChoice(reader, "Select architecture:", archOptions)
	fmt.Println()

	// --- Additional Packages --------------------------------------------
	fmt.Println("[ Additional Packages ]")
	fmt.Println("  Enter additional packages (comma-separated), or press")
	fmt.Println("  Enter to use defaults.")
	fmt.Println("----------------------------------------------------------------")
	extraPkgsInput := promptString(reader, "Additional packages:", "")
	if extraPkgsInput != "" {
		for _, pkg := range strings.Split(extraPkgsInput, ",") {
			pkg = strings.TrimSpace(pkg)
			if pkg != "" {
				additionalPkgs = append(additionalPkgs, pkg)
			}
		}
	}
	fmt.Println()

	// --- Mirror ---------------------------------------------------------
	defaultMirror := "http://archive.ubuntu.com/ubuntu/"
	if isDebian {
		defaultMirror = "http://deb.debian.org/debian/"
	}
	mirror := promptString(reader, fmt.Sprintf("APT mirror [%s]:", defaultMirror), defaultMirror)
	fmt.Println()

	// --- Calamares Configuration ----------------------------------------
	var calamaresConfigPath string
	if installerType == "calamares" {
		fmt.Println("[ Calamares Configuration ]")
		fmt.Println("  Optionally provide a path to a custom Calamares config")
		fmt.Println("  directory. Leave empty to use defaults.")
		fmt.Println("----------------------------------------------------------------")
		calamaresConfigPath = promptString(reader, "Calamares config path (optional):", "")
	}
	fmt.Println()

	// --- Application Support --------------------------------------------
	fmt.Println("[ Application Support ]")
	enableFlatpakStr := promptString(reader, "Enable Flatpak support? [Y/n]:", "Y")
	enableFlatpak := strings.ToLower(enableFlatpakStr) != "n"
	fmt.Println()

	// --- Security -------------------------------------------------------
	fmt.Println("[ Security ]")
	blockSnapd := true
	if !isDebian {
		blockSnapdStr := promptString(reader, "Block snapd? [Y/n]:", "Y")
		blockSnapd = strings.ToLower(blockSnapdStr) != "n"
	}
	enableFirewall := false
	fwStr := promptString(reader, "Enable firewall (ufw)? [y/N]:", "N")
	enableFirewall = strings.ToLower(fwStr) == "y"
	fmt.Println()

	// --- Generate Configuration -----------------------------------------
	cfg := buildWizardConfig(wizardParams{
		release:         release,
		isDebian:        isDebian,
		isMinimal:       isMinimal,
		desktop:         desktop,
		additionalPkgs:  additionalPkgs,
		installerType:   installerType,
		slideshow:       slideshow,
		hostname:        hostname,
		arch:            arch,
		mirror:          mirror,
		calamaresConfig: calamaresConfigPath,
		blockSnapd:      blockSnapd,
		enableFirewall:  enableFirewall,
		enableFlatpak:   enableFlatpak,
	})

	// --- Save Configuration ---------------------------------------------
	fmt.Println("[ Save Configuration ]")
	defaultOutputName := fmt.Sprintf("kagami-%s-%s.json", release, desktop)
	if isMinimal {
		defaultOutputName = fmt.Sprintf("kagami-%s-minimal-installer.json", release)
	}

	configDir, _ := system.GetAppPaths()
	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("[WARNING] Failed to create config directory %s: %v\n", configDir, err)
		// Fallback to current directory
		configDir = "."
	}

	defaultOutputPath := filepath.Join(configDir, defaultOutputName)
	outputPath := promptString(reader, fmt.Sprintf("Output file [%s]:", defaultOutputPath), defaultOutputPath)
	fmt.Println()

	// --- Preview --------------------------------------------------------
	prettyJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal config: %v", err)
	}

	fmt.Println("----------------------------------------------------------------")
	fmt.Println("                   Configuration Preview")
	fmt.Println("----------------------------------------------------------------")
	fmt.Println(string(prettyJSON))
	fmt.Println()

	confirm := promptString(reader, "Save this configuration? [Y/n]:", "Y")
	if strings.ToLower(confirm) == "n" {
		fmt.Println("Configuration discarded.")
		return cfg, "", nil
	}

	// Save to file
	if err := cfg.SaveToFile(outputPath); err != nil {
		return nil, "", fmt.Errorf("failed to save config: %v", err)
	}

	absPath, _ := filepath.Abs(outputPath)
	fmt.Printf("\n[SUCCESS] Configuration saved to: %s\n", absPath)
	fmt.Printf("[INFO] Build with: sudo kagami --config %s\n\n", absPath)

	return cfg, outputPath, nil
}

type wizardParams struct {
	release         string
	isDebian        bool
	isMinimal       bool
	desktop         string
	additionalPkgs  []string
	installerType   string
	slideshow       string
	hostname        string
	arch            string
	mirror          string
	calamaresConfig string
	blockSnapd      bool
	enableFirewall  bool
	enableFlatpak   bool
}

func buildWizardConfig(p wizardParams) *Config {
	// Essential packages differ between Ubuntu and Debian
	essential := getEssentialPackages(p.isDebian)

	// Default additional packages
	defaultAdditional := []string{"vim", "curl", "wget", "git", "htop"}
	additional := append(defaultAdditional, p.additionalPkgs...)

	// Remove list
	var removeList []string
	var disableServices []string
	if !p.isDebian {
		removeList = []string{
			"ubuntu-advantage-tools",
			"ubuntu-report",
			"whoopsie",
			"apport",
			"popularity-contest",
		}
		disableServices = []string{
			"whoopsie",
			"apport",
			"ubuntu-report",
		}
	}

	cfg := &Config{
		Release: p.release,
		System: SystemConfig{
			Hostname:     p.hostname,
			BlockSnapd:   p.blockSnapd,
			Architecture: p.arch,
			Locale:       "en_US.UTF-8",
			Timezone:     "UTC",
		},
		Repository: RepositoryConfig{
			Mirror:          p.mirror,
			UseProposed:     false,
			AdditionalRepos: []AdditionalRepo{},
		},
		Packages: PackageConfig{
			Essential:     essential,
			Additional:    additional,
			Desktop:       p.desktop,
			RemoveList:    removeList,
			EnableFlatpak: p.enableFlatpak,
		},
		Installer: InstallerConfig{
			Type:            p.installerType,
			Slideshow:       p.slideshow,
			CalamaresConfig: p.calamaresConfig,
		},
		Network: NetworkConfig{
			Manager: "network-manager",
		},
		Security: SecurityConfig{
			EnableFirewall:    p.enableFirewall,
			BlockSnapdForever: p.blockSnapd,
			DisableServices:   disableServices,
		},
	}

	return cfg
}

// getEssentialPackages returns the essential packages list for the distro
func getEssentialPackages(isDebian bool) []string {
	if isDebian {
		return []string{
			"sudo",
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
			"grub-efi-amd64",
			"shim-signed",
			"mtools",
			"binutils",
		}
	}
	return []string{
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
	}
}

// getMinimalWMPackages returns the package list for a minimal ALCI-style live
// environment: a WM + Xorg + display manager + terminal + Calamares autostart
func getMinimalWMPackages(wm string, isDebian bool) []string {
	// Common base packages for any minimal WM installer ISO
	base := []string{
		"xorg",
		"xinit",
		"xterm",
		"lightdm",
		"lightdm-gtk-greeter",
		"network-manager-gnome",
		"dbus-x11",
		"fonts-dejavu-core",
		"lxappearance",
		"pcmanfm",
		"mousepad",
	}

	switch wm {
	case "openbox":
		base = append(base,
			"openbox",
			"obconf",
			"tint2",
			"feh",
			"dunst",
			"lxpolkit",
		)
	case "i3":
		base = append(base,
			"i3-wm",
			"i3status",
			"i3lock",
			"dmenu",
			"dunst",
			"lxpolkit",
			"feh",
		)
	case "xfce4-minimal":
		base = append(base,
			"xfce4-panel",
			"xfce4-session",
			"xfce4-settings",
			"xfce4-terminal",
			"xfdesktop4",
			"xfwm4",
			"thunar",
		)
	}

	return base
}

// --- Helper functions -------------------------------------------------------

func promptChoice(reader *bufio.Reader, prompt string, options []WizardOption) string {
	fmt.Println(prompt)
	for i, opt := range options {
		fmt.Printf("  %d) %-20s %s\n", i+1, opt.Label, opt.Description)
	}

	for {
		fmt.Print("\n  Enter choice [1]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			return options[0].Key
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(options) {
			fmt.Printf("  Invalid choice. Please enter 1-%d.\n", len(options))
			continue
		}
		return options[idx-1].Key
	}
}

func promptString(reader *bufio.Reader, prompt, defaultVal string) string {
	fmt.Printf("  %s ", prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}
