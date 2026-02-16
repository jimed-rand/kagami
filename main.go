package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"kagami/pkg/builder"
	"kagami/pkg/config"
	"kagami/pkg/system"
)

func main() {
	var (
		configFile  = flag.String("config", "", "Path to configuration file (JSON)")
		release     = flag.String("release", "noble", "Ubuntu release (lts, devel, focal, jammy, noble, resolute)")
		workDir     = flag.String("workdir", "", "Working directory for build")
		outputISO   = flag.String("output", "", "Output ISO file path")
		hostname    = flag.String("hostname", "ubuntu-custom", "Custom hostname")
		noSnapd     = flag.Bool("block-snapd", true, "Block snapd installation (default: true)")
		interactive = flag.Bool("interactive", false, "Interactive mode for package selection")
		showVersion = flag.Bool("version", false, "Show version information")
		checkDeps   = flag.Bool("check-deps", false, "Check system dependencies")
		installDeps = flag.Bool("install-deps", false, "Install missing dependencies (requires sudo)")
		mirrorURL   = flag.String("mirror", "", "Custom Ubuntu mirror URL (e.g. http://archive.ubuntu.com/ubuntu/)")
	)

	flag.Parse()

	// Setup logging for a clean, professional output
	log.SetFlags(0)
	log.SetOutput(new(politeLogger))

	// Show version
	if *showVersion {
		fmt.Printf("%s %s\n", config.AppName, config.Version)
		fmt.Printf("Synthesized utilizing the Go runtime %s for %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// Display banner
	printBanner()

	// Check if running on Linux
	if runtime.GOOS != "linux" {
		fatal("%s only runs on Linux systems", config.AppName)
	}

	// Check for APT-based system
	if !system.IsAPTBased() {
		fatal("%s requires an APT-based distribution (Ubuntu/Debian)", config.AppName)
	}

	// Check for root privileges
	if os.Geteuid() != 0 {
		fatal("%s must be run as root (use sudo)", config.AppName)
	}

	// Check dependencies
	if *checkDeps {
		fmt.Println("\n Checking system dependencies...")
		deps := system.CheckDependencies()
		if len(deps.Missing) == 0 {
			fmt.Println(" All dependencies are installed")
			os.Exit(0)
		} else {
			fmt.Println("\n Missing dependencies:")
			for _, dep := range deps.Missing {
				fmt.Printf("  - %s\n", dep)
			}
			fmt.Printf("\nInstall with: sudo apt-get install %s\n", system.GetInstallCommand(deps.Missing))
			fmt.Println("Or run: sudo kagami --install-deps")
			os.Exit(1)
		}
	}

	// Install dependencies
	if *installDeps {
		fmt.Println("\n Installing dependencies...")
		if err := system.InstallDependencies(); err != nil {
			fatal("Failed to install dependencies: %v", err)
		}
		fmt.Println(" All dependencies installed successfully")
		os.Exit(0)
	}

	// Verify dependencies before proceeding
	deps := system.CheckDependencies()
	if len(deps.Missing) > 0 {
		fmt.Println("\n Missing required dependencies:")
		for _, dep := range deps.Missing {
			fmt.Printf("  - %s\n", dep)
		}
		fmt.Println("\nRun: sudo kagami --install-deps")
		os.Exit(1)
	}

	// Handle release selection logic
	var selectedRelease string
	releaseMap := map[string]string{
		"lts":      "noble",
		"rolling":  "devel",
		"unstable": "devel",
	}

	// Determine if release was explicitly set by user
	releaseSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "release" {
			releaseSet = true
		}
	})

	inputRelease := *release
	if mapped, ok := releaseMap[inputRelease]; ok {
		fmt.Printf(" Mapping '%s' to '%s'\n", inputRelease, mapped)
		inputRelease = mapped
	}
	selectedRelease = inputRelease

	// Determine working directory
	var baseWorkDir string
	if *workDir != "" {
		baseWorkDir = *workDir
	} else if *configFile != "" {
		// Default to the directory where the config file is located
		absPath, err := filepath.Abs(*configFile)
		if err != nil {
			fatal("Failed to get absolute path for config: %v", err)
		}
		baseWorkDir = filepath.Join(filepath.Dir(absPath), "kagami-workspace")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fatal("Failed to get user home directory: %v", err)
		}
		baseWorkDir = filepath.Join(homeDir, "kagami-workspace")
	}

	// Determine output ISO path
	var isoPath string
	if *outputISO != "" {
		isoPath = *outputISO
	} else {
		// We'll update this after config is loaded to ensure we have the final release
		isoPath = ""
	}

	// Load or create configuration
	var cfg *config.Config
	var err error

	if *configFile != "" {
		cfg, err = config.LoadFromFile(*configFile)
		if err != nil {
			fatal("Failed to load config: %v", err)
		}

		// Only override JSON release if CLI flag was explicitly provided
		if releaseSet {
			cfg.Release = selectedRelease
		}
	} else {
		cfg = config.NewDefaultConfig(selectedRelease)
		cfg.System.Hostname = *hostname
		cfg.System.BlockSnapd = *noSnapd
	}

	// Set final ISO path if not provided
	if isoPath == "" {
		isoPath = filepath.Join(baseWorkDir, fmt.Sprintf("kagami-ubuntu-%s.iso", cfg.Release))
	}

	// Apply CLI overrides
	if *mirrorURL != "" {
		cfg.Repository.Mirror = *mirrorURL
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fatal("Invalid configuration: %v", err)
	}

	// Create builder instance
	b := builder.NewBuilder(cfg, baseWorkDir, isoPath)

	// Print build information
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("               %s - Ubuntu ISO Builder %s               \n", config.AppName, config.Version)
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("\n[INFO] Build Configuration:\n")
	fmt.Printf("  Release:      %s\n", cfg.Release)
	fmt.Printf("  Workspace:    %s\n", baseWorkDir)
	fmt.Printf("  Output ISO:   %s\n", isoPath)
	fmt.Printf("  Block Snapd:  %v\n", cfg.System.BlockSnapd)
	fmt.Printf("  Desktop:      %s\n", getDesktopDescription(cfg))
	fmt.Printf("  Installer:    %s\n", cfg.Installer.Type)
	fmt.Println()

	if *interactive {
		if err := b.InteractivePackageSelection(); err != nil {
			fatal("Interactive selection failed: %v", err)
		}
	}

	// Run the build
	if err := b.Build(); err != nil {
		fatal("Build failed: %v", err)
	}

	fmt.Println("\n---------------------------------------------------------------")
	fmt.Println("            [SUCCESS] Build Completed Successfully!            ")
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("\n[OUTPUT] ISO Location: %s\n", isoPath)
	fmt.Printf("[OUTPUT] ISO Size: %s\n", getFileSize(isoPath))
	fmt.Println("\n[INFO] Next steps:")
	fmt.Println("  1. Test in VM: qemu-system-x86_64 -cdrom <iso> -m 2048")
	fmt.Println("  2. Write to USB: sudo dd if=<iso> of=/dev/sdX bs=4M status=progress")
	fmt.Println()
}

func printBanner() {
	fmt.Printf("%s - Ubuntu ISO Builder %s\n", config.AppName, config.Version)
	fmt.Println("Vanilla Desktop Environments")
	fmt.Println()
}

func getDesktopDescription(cfg *config.Config) string {
	if cfg.Packages.Desktop != "none" {
		return cfg.Packages.Desktop
	}
	// Try to detect from additional packages
	for _, pkg := range cfg.Packages.Additional {
		switch pkg {
		case "gnome-shell":
			return "GNOME (vanilla)"
		case "plasma-desktop":
			return "KDE Plasma (vanilla)"
		case "xfce4":
			return "Xfce (vanilla)"
		case "lxqt-core":
			return "LXQt (vanilla)"
		case "mate-desktop-environment-core":
			return "MATE (vanilla)"
		case "budgie-desktop":
			return "Budgie (vanilla)"
		case "cinnamon-desktop-environment":
			return "Cinnamon (vanilla)"
		}
	}
	return "custom"
}

func getFileSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "unknown"
	}
	size := float64(info.Size())
	units := []string{"B", "KB", "MB", "GB"}
	unitIndex := 0
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}
	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}

// fatal prints a formatted error message with a timestamp and terminates the application
func fatal(format string, v ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] [FATAL] %s\n", timestamp, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// politeLogger provides a clean logging format for user-facing output
type politeLogger struct{}

func (l *politeLogger) Write(p []byte) (n int, err error) {
	timestamp := time.Now().Format("15:04:05")
	return fmt.Printf("[%s] [INFO] %s", timestamp, string(p))
}
