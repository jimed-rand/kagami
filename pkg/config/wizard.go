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

type WizardOption struct {
	Key         string
	Label       string
	Description string
}

func RunWizard() (*Config, string, error) {
	reader := bufio.NewReader(os.Stdin)
	var wmChoice string

	fmt.Println()
	fmt.Println("----------------------------------------------------------------")
	fmt.Println("               Kagami Configuration Wizard")
	fmt.Println("         Interactive ISO Build Configuration Interface")
	fmt.Println("----------------------------------------------------------------")
	fmt.Println()

	fmt.Println("[ Distribution ]")
	distOptions := []WizardOption{
		{"ubuntu", "Ubuntu", "Ubuntu-based ISO synthesis"},
		{"debian", "Debian", "Debian-based ISO synthesis (Stable, Testing, or Unstable)"},
	}
	distChoice := promptChoice(reader, "Select base distribution:", distOptions)
	isDebian := distChoice == "debian"
	fmt.Println()

	fmt.Println("[ Release ]")
	var releaseOptions []WizardOption
	if isDebian {
		releaseOptions = []WizardOption{
			{"stable", "Stable", "Debian Stable (current)"},
			{"testing", "Testing", "Debian Testing (current)"},
			{"unstable", "Unstable", "Debian Unstable / Sid"},
			{"bookworm", "Bookworm", "Debian 12"},
			{"trixie", "Trixie", "Debian 13"},
		}
	} else {
		releaseOptions = []WizardOption{
			{"noble", "Noble Numbat", "Ubuntu 24.04 LTS"},
			{"resolute", "Resolute Rambutan", "Ubuntu 26.04 LTS (upcoming)"},
			{"jammy", "Jammy Jellyfish", "Ubuntu 22.04 LTS"},
			{"devel", "Development", "Ubuntu Rolling Development"},
		}
	}
	release := promptChoice(reader, "Select release:", releaseOptions)
	fmt.Println()

	fmt.Println("[ Build Mode ]")
	modeOptions := []WizardOption{
		{"desktop", "Desktop ISO", "Full desktop environment with graphical installer"},
		{"minimal", "Minimal Installer", "Minimal live environment with Calamares auto-launch"},
	}
	buildMode := promptChoice(reader, "Select build mode:", modeOptions)
	isMinimal := buildMode == "minimal"
	fmt.Println()

	var desktop string
	var additionalPkgs []string

	if isMinimal {
		fmt.Println("[ Window Manager (Minimal Installer) ]")
		fmt.Println("  The minimal installer boots directly into a lightweight window")
		fmt.Println("  manager with the Calamares installer launched automatically.")
		fmt.Println("----------------------------------------------------------------")

		wmOptions := []WizardOption{
			{"openbox", "Openbox", "Lightweight stacking window manager (recommended)"},
			{"dwm", "dwm", "Dynamic window manager (ALCI style)"},
			{"xfce4-minimal", "Xfce4 (Minimal)", "Xfce4 panel and desktop only"},
		}
		wmChoice = promptChoice(reader, "Select window manager:", wmOptions)
		desktop = "none"
		additionalPkgs = getMinimalWMPackages(wmChoice)
	} else {
		fmt.Println("[ Desktop Environment ]")
		var desktopOptions []WizardOption
		if isDebian {
			desktopOptions = []WizardOption{
				{"xfce", "Xfce", "Lightweight desktop (task-xfce-desktop)"},
				{"gnome", "GNOME", "Full-featured modern desktop (task-gnome-desktop)"},
				{"kde", "KDE Plasma", "Feature-rich Qt-based desktop (task-kde-desktop)"},
				{"lxqt", "LXQt", "Lightweight Qt-based desktop (task-lxqt-desktop)"},
				{"mate", "MATE", "Traditional GNOME 2 continuation (task-mate-desktop)"},
				{"lxde", "LXDE", "Lightweight GTK-based desktop (task-lxde-desktop)"},
				{"none", "None (Manual)", "Specify packages explicitly in the additional list"},
			}
		} else {
			desktopOptions = []WizardOption{
				{"gnome", "GNOME", "Ubuntu GNOME desktop"},
				{"kde", "KDE Plasma", "Kubuntu desktop"},
				{"xfce", "Xfce", "Xubuntu desktop"},
				{"mate", "MATE", "Ubuntu MATE desktop"},
				{"lxqt", "LXQt", "Lubuntu desktop (LXQt)"},
				{"lxde", "LXDE", "Lubuntu desktop (LXDE)"},
				{"none", "None (Manual)", "Specify packages explicitly in the additional list"},
			}
		}
		desktop = promptChoice(reader, "Select desktop environment:", desktopOptions)
	}
	fmt.Println()

	var installerType string
	var slideshow string

	if isMinimal || isDebian {
		installerType = "calamares"
		fmt.Println("[ Installer ]")
		if isMinimal {
			fmt.Println("  Minimal build mode employs Calamares as the graphical installer.")
		} else {
			fmt.Println("  Debian builds employ Calamares as the graphical installer.")
		}
		fmt.Println("----------------------------------------------------------------")
	} else {
		fmt.Println("[ Installer ]")
		installerOptions := []WizardOption{
			{"ubiquity", "Ubiquity", "Traditional Ubuntu GTK-based installer"},
			{"calamares", "Calamares", "Universal distro-agnostic installer framework"},
		}
		installerType = promptChoice(reader, "Select installer:", installerOptions)

		if installerType == "ubiquity" {
			slideshowOptions := []WizardOption{
				{"ubuntu", "Ubuntu", "Standard Ubuntu slideshow"},
				{"kubuntu", "Kubuntu", "KDE Plasma slideshow"},
				{"xubuntu", "Xubuntu", "Xfce slideshow"},
				{"lubuntu", "Lubuntu", "LXQt slideshow"},
				{"ubuntu-mate", "Ubuntu MATE", "MATE slideshow"},
			}
			slideshow = promptChoice(reader, "Select installer slideshow:", slideshowOptions)
		}
	}
	fmt.Println()

	fmt.Println("[ System Configuration ]")
	defaultHostname := distChoice
	if isMinimal {
		defaultHostname += "-minimal"
	} else if !isDebian && desktop != "none" {
		defaultHostname += "-" + desktop
	} else {
		defaultHostname += "-desktop"
	}
	hostname := promptString(reader, fmt.Sprintf("System hostname [%s]:", defaultHostname), defaultHostname)
	fmt.Println()

	archOptions := []WizardOption{
		{"amd64", "amd64", "64-bit x86 (standard)"},
		{"arm64", "arm64", "64-bit ARM"},
	}
	arch := promptChoice(reader, "Target architecture:", archOptions)
	fmt.Println()

	fmt.Println("[ Kernel Selection ]")
	var kernelOptions []WizardOption
	if isDebian {
		kernelOptions = []WizardOption{
			{"linux-image-amd64", "Standard", "Standard Debian kernel"},
			{"linux-image-rt-amd64", "Real-time", "Real-time kernel for latency-sensitive workloads"},
			{"linux-image-cloud-amd64", "Cloud", "Optimised for virtualised cloud environments"},
		}
	} else {
		kernelOptions = []WizardOption{
			{"linux-generic", "Generic", "Standard Ubuntu kernel (recommended)"},
			{"linux-lowlatency", "Low-latency", "Reduced-latency kernel for audio and professional use"},
			{"linux-oem-24.04", "OEM", "OEM stabilised kernel"},
		}
	}
	kernel := promptChoice(reader, "Select kernel variant:", kernelOptions)
	fmt.Println()

	fmt.Println("[ Additional Packages ]")
	fmt.Println("  Specify additional packages as a comma-separated list,")
	fmt.Println("  or press Enter to proceed with defaults.")
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

	defaultMirror := "http://archive.ubuntu.com/ubuntu/"
	if isDebian {
		defaultMirror = "http://deb.debian.org/debian/"
	}
	mirror := promptString(reader, fmt.Sprintf("APT repository mirror [%s]:", defaultMirror), defaultMirror)
	fmt.Println()

	var branding BrandingConfig
	if installerType == "calamares" {
		fmt.Println("[ Calamares Branding ]")
		fmt.Println("  Customise the installer product identity and support information.")
		fmt.Println("----------------------------------------------------------------")
		defaultName := "Debian GNU/Linux"
		defaultShortName := "Debian"
		defaultURL := "https://www.debian.org"
		if !isDebian {
			defaultName = "Ubuntu"
			defaultShortName = "Ubuntu"
			defaultURL = "https://ubuntu.com"
		}

		branding.ProductName = promptString(reader, fmt.Sprintf("Product name [%s]:", defaultName), defaultName)
		branding.ShortProductName = promptString(reader, fmt.Sprintf("Short product name [%s]:", defaultShortName), defaultShortName)
		branding.ProductUrl = promptString(reader, fmt.Sprintf("Product URL [%s]:", defaultURL), defaultURL)
		branding.SupportUrl = promptString(reader, fmt.Sprintf("Support URL [%s/support]:", defaultURL), defaultURL+"/support")
		branding.Version = promptString(reader, fmt.Sprintf("Version [%s]:", release), release)
	}
	fmt.Println()

	fmt.Println("[ Application Support ]")
	enableFlatpakStr := promptString(reader, "Enable Flatpak support? [Y/n]:", "Y")
	enableFlatpak := strings.ToLower(enableFlatpakStr) != "n"
	fmt.Println()

	fmt.Println("[ Security Configuration ]")
	blockSnapd := true
	if !isDebian {
		blockSnapdStr := promptString(reader, "Apply permanent snapd suppression? [Y/n]:", "Y")
		blockSnapd = strings.ToLower(blockSnapdStr) != "n"
	}
	fwStr := promptString(reader, "Enable UFW firewall? [y/N]:", "N")
	enableFirewall := strings.ToLower(fwStr) == "y"
	fmt.Println()

	cfg := buildWizardConfig(wizardParams{
		distro:         distChoice,
		release:        release,
		isMinimal:      isMinimal,
		desktop:        desktop,
		additionalPkgs: additionalPkgs,
		installerType:  installerType,
		slideshow:      slideshow,
		hostname:       hostname,
		arch:           arch,
		kernel:         kernel,
		mirror:         mirror,
		branding:       branding,
		blockSnapd:     blockSnapd,
		enableFirewall: enableFirewall,
		enableFlatpak:  enableFlatpak,
		wm:             wmChoice,
	})

	fmt.Println("[ Configuration Output ]")
	defaultOutputName := fmt.Sprintf("%s-%s-%s.json", distChoice, release, desktop)
	if isMinimal {
		defaultOutputName = fmt.Sprintf("%s-%s-minimal.json", distChoice, release)
	}

	configDir, _ := system.GetAppPaths()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("[WARNING] Could not create configuration directory %s: %v\n", configDir, err)
		configDir = "."
	}

	defaultOutputPath := filepath.Join(configDir, defaultOutputName)
	outputPath := promptString(reader, fmt.Sprintf("Output file path [%s]:", defaultOutputPath), defaultOutputPath)
	fmt.Println()

	prettyJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("configuration serialisation failed: %v", err)
	}

	fmt.Println("----------------------------------------------------------------")
	fmt.Println("                   Configuration Preview")
	fmt.Println("----------------------------------------------------------------")
	fmt.Println(string(prettyJSON))
	fmt.Println()

	confirm := promptString(reader, "Persist this configuration? [Y/n]:", "Y")
	if strings.ToLower(confirm) == "n" {
		fmt.Println("Configuration discarded.")
		return cfg, "", nil
	}

	if err := cfg.SaveToFile(outputPath); err != nil {
		return nil, "", fmt.Errorf("configuration persistence failed: %v", err)
	}

	absPath, _ := filepath.Abs(outputPath)
	fmt.Printf("\n[OK] Configuration persisted to: %s\n", absPath)
	fmt.Printf("[INFO] Initiate build with: sudo kagami --config %s\n\n", absPath)

	return cfg, outputPath, nil
}

type wizardParams struct {
	distro         string
	release        string
	isMinimal      bool
	desktop        string
	additionalPkgs []string
	installerType  string
	slideshow      string
	hostname       string
	arch           string
	mirror         string
	branding       BrandingConfig
	blockSnapd     bool
	enableFirewall bool
	enableFlatpak  bool
	kernel         string
	wm             string
}

func buildWizardConfig(p wizardParams) *Config {
	essential := getEssentialPackages(p.distro)
	defaultAdditional := []string{"vim", "curl", "wget", "git", "htop"}
	additional := append(defaultAdditional, p.additionalPkgs...)

	var removeList []string
	var disableServices []string
	if p.distro == "ubuntu" {
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

	return &Config{
		Distro:  p.distro,
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
			Kernel:        p.kernel,
			EnableFlatpak: p.enableFlatpak,
			WM:            p.wm,
		},
		Installer: InstallerConfig{
			Type:      p.installerType,
			Slideshow: p.slideshow,
			Branding:  p.branding,
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
}

func getEssentialPackages(distro string) []string {
	if distro == "debian" {
		return []string{
			"sudo",
			"live-boot",
			"live-boot-initramfs-tools",
			"live-config",
			"live-config-systemd",
			"discover",
			"laptop-detect",
			"os-prober",
			"network-manager",
			"net-tools",
			"wireless-tools",
			"wpasupplicant",
			"locales",
			"grub-common",
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

func getMinimalWMPackages(wm string) []string {
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
		base = append(base, "openbox", "obconf", "tint2", "feh", "dunst", "lxpolkit")
	case "dwm":
		base = append(base, "dwm", "dmenu", "stterm", "feh", "dunst", "lxpolkit")
	case "xfce4-minimal":
		base = append(base,
			"xfce4-panel", "xfce4-session", "xfce4-settings",
			"xfce4-terminal", "xfdesktop4", "xfwm4", "thunar",
		)
	}

	return base
}

func promptChoice(reader *bufio.Reader, prompt string, options []WizardOption) string {
	fmt.Println(prompt)
	for i, opt := range options {
		fmt.Printf("  %d) %-22s %s\n", i+1, opt.Label, opt.Description)
	}

	for {
		fmt.Print("\n  Selection [1]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			return options[0].Key
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(options) {
			fmt.Printf("  Invalid selection. Enter a value between 1 and %d.\n", len(options))
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
