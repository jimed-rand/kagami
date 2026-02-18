package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
		wizardMode  = flag.Bool("wizard", false, "Launch interactive configuration wizard")
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

	// Show usage and version if no flags are provided
	if flag.NFlag() == 0 {
		fmt.Printf("%s %s - Vanilla ISO Builder\n", config.AppName, config.Version)
		fmt.Printf("Synthesized utilizing the Go runtime %s for %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

		// Detect non-APT system and show appropriate warnings
		if !system.IsAPTBased() {
			fmt.Println()
			fmt.Println("----------------------------------------------------------------")
			fmt.Println("  [WARNING] NON-APT SYSTEM DETECTED")
			fmt.Println()
			fmt.Println("  Kagami is designed for Ubuntu/Debian (APT-based) only.")
			fmt.Println("  Build, dependency, and installation features are disabled.")
			fmt.Println()
			fmt.Println("  On this system, only generating JSON configuration files")
			fmt.Println("  via --wizard is supported.")
			fmt.Println("----------------------------------------------------------------")
			fmt.Println()
			fmt.Println("[TIP] If you are on a non-APT distribution (Fedora, Arch, etc.), use Distrobox")
			fmt.Println("      (Docker/Podman) to create an Ubuntu/Debian container. Ensure you map/use")
			fmt.Printf("      your home folder so %s can access the build workspace correctly.\n", config.AppName)
			fmt.Println()
			fmt.Println("      Example:")
			fmt.Println("        distrobox create --name kagami-box --image ubuntu:noble --home ~/distrobox-home")
			fmt.Println("        distrobox enter kagami-box")
			fmt.Printf("        sudo %s --wizard   # Generate config JSON inside the container\n", os.Args[0])
			fmt.Println()
		}

		fmt.Printf("\nUsage:\n")
		fmt.Printf("  sudo %s [options]\n", os.Args[0])
		fmt.Printf("\nOptions:\n")
		flag.PrintDefaults()
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  sudo %s --wizard\n", os.Args[0])
		fmt.Printf("  sudo %s --config examples/debian-bookworm-minimal-installer.json\n\n", os.Args[0])
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
		fmt.Printf("\n[TIP] If you are on a non-APT distribution (Fedora, Arch, etc.), use Distrobox (Docker/Podman)\n")
		fmt.Printf("      to create an Ubuntu/Debian container. Ensure you map/use your home folder\n")
		fmt.Printf("      so %s can access the build workspace correctly.\n\n", config.AppName)
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

	// Interactive wizard mode
	if *wizardMode {
		cfg, outputPath, err := config.RunWizard()
		if err != nil {
			fatal("Wizard failed: %v", err)
		}
		if outputPath == "" {
			os.Exit(0)
		}

		// Ask if user wants to build now
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("\nDo you want to build the ISO now? [y/N]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if strings.ToLower(input) != "y" {
			fmt.Println("\nYou can build later with:")
			fmt.Printf("  sudo kagami --config %s\n\n", outputPath)
			os.Exit(0)
		}

		// Proceed to build with the generated config
		if err := cfg.Validate(); err != nil {
			fatal("Invalid configuration: %v", err)
		}

		// Use environment-aware work directory
		_, wizardWorkDir := system.GetAppPaths()
		wizardIsoPath := filepath.Join(wizardWorkDir, fmt.Sprintf("kagami-%s.iso", cfg.Release))

		b := builder.NewBuilder(cfg, wizardWorkDir, wizardIsoPath)
		printBuildInfo(cfg, wizardWorkDir, wizardIsoPath)

		if err := b.Build(); err != nil {
			fmt.Printf("\n[ERROR] %v\n", err)
			offerCleanup(b, false)
			os.Exit(1)
		}

		// Move ISO to safety before optional cleanup
		wizardIsoPath = moveISOToSafety(wizardIsoPath, wizardWorkDir)

		printBuildSuccess(wizardIsoPath)
		offerCleanup(b, true)
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

	// Get default paths based on environment (portable vs installed)
	_, defaultWorkDir := system.GetAppPaths()

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
		baseWorkDir = defaultWorkDir
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
	printBuildInfo(cfg, baseWorkDir, isoPath)

	if *interactive {
		if err := b.InteractivePackageSelection(); err != nil {
			fatal("Interactive selection failed: %v", err)
		}
	}

	// Run the build
	if err := b.Build(); err != nil {
		fmt.Printf("\n[ERROR] %v\n", err)
		offerCleanup(b, false)
		os.Exit(1)
	}

	// Move ISO to safety before optional cleanup
	isoPath = moveISOToSafety(isoPath, baseWorkDir)

	printBuildSuccess(isoPath)
	offerCleanup(b, true)
}

func printBuildInfo(cfg *config.Config, workDir, isoPath string) {
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("               %s - Ubuntu ISO Builder %s               \n", config.AppName, config.Version)
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("\n[INFO] Build Configuration:\n")
	fmt.Printf("  Release:      %s\n", cfg.Release)
	fmt.Printf("  Workspace:    %s\n", workDir)
	fmt.Printf("  Output ISO:   %s\n", isoPath)
	fmt.Printf("  Block Snapd:  %v\n", cfg.System.BlockSnapd)
	fmt.Printf("  Desktop:      %s\n", getDesktopDescription(cfg))
	fmt.Printf("  Installer:    %s\n", cfg.Installer.Type)
	fmt.Println()
}

func printBuildSuccess(isoPath string) {
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

func offerCleanup(b *builder.Builder, success bool) {
	reader := bufio.NewReader(os.Stdin)
	var input string

	if !success {
		fmt.Println("\n---------------------------------------------------------------")
		fmt.Println("            [!] Build failed. Cleanup Recommended              ")
		fmt.Println("---------------------------------------------------------------")
		fmt.Println("It is highly recommended to unmount filesystems to avoid issues.")
		fmt.Print("Unmount and remove workspace? [Y/n]: ")
		input, _ = reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) != "n" {
			b.RemoveWorkspace()
		}
		return
	}

	fmt.Print("\nBuild successful. Clean up workspace/chroot now? [y/N]: ")
	input, _ = reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(input)) == "y" {
		b.RemoveWorkspace()
	}
}

// moveISOToSafety moves the generated ISO out of the workspace to the program directory
// to prevent it from being deleted during cleanup.
func moveISOToSafety(isoPath, workDir string) string {
	absISO, _ := filepath.Abs(isoPath)
	absWork, _ := filepath.Abs(workDir)

	// If ISO is not in workspace, it's already safe
	if !strings.HasPrefix(absISO, absWork) {
		return isoPath
	}

	// Get executable directory
	execPath, err := os.Executable()
	if err != nil {
		return isoPath
	}
	destDir := filepath.Dir(execPath)

	// If running from system bin, use CWD instead
	if strings.Contains(destDir, "/usr/bin") || strings.Contains(destDir, "/bin") {
		destDir, _ = os.Getwd()
	}

	newPath := filepath.Join(destDir, filepath.Base(isoPath))
	if absISO == newPath {
		return isoPath
	}

	fmt.Printf("\n[ACTION] Moving ISO to safe location: %s\n", newPath)
	// Try rename first
	if err := os.Rename(isoPath, newPath); err != nil {
		// Fallback to copy if rename fails (e.g. cross-device)
		cmd := exec.Command("cp", isoPath, newPath)
		if err := cmd.Run(); err != nil {
			fmt.Printf("[WARNING] Failed to move ISO: %v\n", err)
			return isoPath
		}
		os.Remove(isoPath)
	}

	return newPath
}
