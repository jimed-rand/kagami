package tui

import (
	"bufio"
	"fmt"
	"kagami/pkg/config"
	"os"
	"strconv"
	"strings"
)

type WizardOption struct {
	Key         string
	Label       string
	Description string
}

func RunCLI() (*config.Config, string, string, error) {
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

	var branding config.BrandingConfig
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

	fmt.Println("[ Configuration Output ]")
	defaultOutputName := fmt.Sprintf("%s-%s-%s.json", distChoice, release, desktop)
	if isMinimal {
		defaultOutputName = fmt.Sprintf("%s-%s-minimal.json", distChoice, release)
	}
	outputPath := promptString(reader, fmt.Sprintf("Output file path [%s]:", defaultOutputName), defaultOutputName)
	fmt.Println()

	cfg := &config.Config{
		Distro:  distChoice,
		Release: release,
		System: config.SystemConfig{
			Hostname:     hostname,
			BlockSnapd:   blockSnapd,
			Architecture: arch,
			Locale:       "en_US.UTF-8",
			Timezone:     "UTC",
		},
		Repository: config.RepositoryConfig{
			Mirror:          mirror,
			AdditionalRepos: []config.AdditionalRepo{},
		},
		Packages: config.PackageConfig{
			Desktop:       desktop,
			Additional:    additionalPkgs,
			Kernel:        kernel,
			EnableFlatpak: enableFlatpak,
			WM:            wmChoice,
		},
		Installer: config.InstallerConfig{
			Type:      installerType,
			Slideshow: slideshow,
			Branding:  branding,
		},
		Network: config.NetworkConfig{
			Manager: "network-manager",
		},
		Security: config.SecurityConfig{
			EnableFirewall:    enableFirewall,
			BlockSnapdForever: blockSnapd,
		},
	}

	if err := cfg.SaveToFile(outputPath); err != nil {
		return nil, "", "", err
	}

	fmt.Printf("\n[OK] Configuration persisted to: %s\n", outputPath)
	fmt.Printf("[INFO] Initiate build with: sudo kagami --config %s\n\n", outputPath)

	fmt.Println("[ Build Log Display ]")
	logOptions := []WizardOption{
		{"terminal", "Terminal", "Classic scrolling output"},
		{"tui", "TUI", "Visual progress interface"},
	}
	logMode := promptChoice(reader, "Choose log display mode:", logOptions)
	fmt.Println()

	confirm := promptString(reader, "Proceed with ISO synthesis now? [y/N]:", "N")
	if strings.ToLower(confirm) != "y" {
		return cfg, "", "", nil
	}

	return cfg, outputPath, logMode, nil
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
