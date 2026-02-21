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
	"kagami/tui"
)

func main() {
	var (
		configFile    = flag.String("config", "", "Path to the JSON configuration file")
		release       = flag.String("release", "noble", "Target release codename (e.g. noble, jammy, bookworm, trixie, sid)")
		workDir       = flag.String("workdir", "", "Working directory for the build process")
		outputISO     = flag.String("output", "", "Absolute path for the synthesized ISO file")
		hostname      = flag.String("hostname", "ubuntu-custom", "System hostname for the target image")
		noSnapd       = flag.Bool("block-snapd", true, "Apply permanent snapd suppression (default: true)")
		interactive   = flag.Bool("interactive", false, "Enable interactive package selection during build")
		showVersion   = flag.Bool("version", false, "Display version and runtime information")
		checkDeps     = flag.Bool("check-deps", false, "Verify system build dependencies")
		installDeps   = flag.Bool("install-deps", false, "Install missing build dependencies (requires elevated privileges)")
		mirrorURL     = flag.String("mirror", "", "Override APT repository mirror URL")
		wizardMode    = flag.Bool("wizard", false, "Launch the interactive configuration wizard (TUI)")
		wizardCLIMode = flag.Bool("wizard-cli", false, "Launch the classic CLI configuration wizard")
	)

	flag.Parse()

	log.SetFlags(0)
	log.SetOutput(new(formalLogger))

	if *showVersion {
		fmt.Printf("%s %s\n", config.AppName, config.Version)
		fmt.Printf("Compiled with Go runtime %s for %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if flag.NFlag() == 0 {
		fmt.Printf("%s %s - Debian/Ubuntu ISO Builder\n", config.AppName, config.Version)
		fmt.Printf("Compiled with Go runtime %s for %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

		if !system.IsAPTBased() {
			fmt.Println()
			fmt.Println("----------------------------------------------------------------")
			fmt.Println("  [WARNING] NON-APT SYSTEM DETECTED")
			fmt.Println()
			fmt.Println("  This application requires a Debian or Ubuntu (APT-based)")
			fmt.Println("  host environment. Build and installation features are")
			fmt.Println("  unavailable on this system.")
			fmt.Println()
			fmt.Println("  On non-APT distributions, only the configuration wizard")
			fmt.Println("  (--wizard) is supported for generating JSON files.")
			fmt.Println("----------------------------------------------------------------")
			fmt.Println()
			fmt.Printf("  [RECOMMENDATION] Employ Distrobox (Docker/Podman) to create a\n")
			fmt.Printf("  Debian or Ubuntu container with your home directory mapped.\n")
			fmt.Println()
			fmt.Println("  Example:")
			fmt.Println("    distrobox create --name kagami-env --image ubuntu:noble")
			fmt.Println("    distrobox enter kagami-env")
			fmt.Printf("    sudo %s --wizard\n", os.Args[0])
			fmt.Println()
		}

		fmt.Printf("\nUsage:\n")
		fmt.Printf("  sudo %s [options]\n", os.Args[0])
		fmt.Printf("\nOptions:\n")
		flag.PrintDefaults()
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  sudo %s --wizard\n", os.Args[0])
		fmt.Printf("  sudo %s --wizard-cli\n", os.Args[0])
		fmt.Printf("  sudo %s --config examples/debian-bookworm-desktop.json\n\n", os.Args[0])
		os.Exit(0)
	}

	printBanner()

	if runtime.GOOS != "linux" {
		fatal("%s operates exclusively on Linux systems", config.AppName)
	}

	if !system.IsAPTBased() {
		fmt.Printf("\n[RECOMMENDATION] Employ Distrobox (Docker/Podman) to create a Debian or\n")
		fmt.Printf("  Ubuntu container. Map your home directory to the container so %s\n", config.AppName)
		fmt.Printf("  can access the build workspace.\n\n")
		fatal("%s requires an APT-based distribution (Debian or Ubuntu)", config.AppName)
	}

	if os.Geteuid() != 0 {
		fatal("%s must be executed with elevated privileges (sudo)", config.AppName)
	}

	if *checkDeps {
		fmt.Println("\n[INFO] Verifying build dependencies...")
		deps := system.CheckDependencies()
		if len(deps.Missing) == 0 {
			fmt.Println("[OK] All required dependencies are present")
			os.Exit(0)
		}
		fmt.Println("\n[INFO] Absent dependencies:")
		for _, dep := range deps.Missing {
			fmt.Printf("  - %s\n", dep)
		}
		fmt.Printf("\nInstall command: sudo apt-get install %s\n", system.GetInstallCommand(deps.Missing))
		fmt.Println("Alternatively: sudo kagami --install-deps")
		os.Exit(1)
	}

	if *installDeps {
		fmt.Println("\n[INFO] Installing build dependencies...")
		if err := system.InstallDependencies(); err != nil {
			fatal("Dependency installation failed: %v", err)
		}
		fmt.Println("[OK] All dependencies installed successfully")
		os.Exit(0)
	}

	if *wizardMode || *wizardCLIMode {
		var cfg *config.Config
		var outputPath string
		var logMode string
		var err error

		if *wizardCLIMode {
			cfg, outputPath, logMode, err = tui.RunCLI()
		} else {
			cfg, outputPath, logMode, err = tui.Run()
		}

		if err != nil {
			fatal("Wizard execution failed: %v", err)
		}
		if outputPath == "" {
			os.Exit(0)
		}

		if err := cfg.Validate(); err != nil {
			fatal("Configuration validation failed: %v", err)
		}

		_, wizardWorkDir := system.GetAppPaths()
		wizardIsoPath := filepath.Join(wizardWorkDir, fmt.Sprintf("kagami-%s.iso", cfg.Release))

		b := builder.NewBuilder(cfg, wizardWorkDir, wizardIsoPath)

		if logMode == "tui" {
			if err := tui.ShowBuild(b); err != nil {
				fmt.Printf("\n[ERROR] %v\n", err)
				offerCleanup(b, false)
				os.Exit(1)
			}
		} else {
			printBuildInfo(cfg, wizardWorkDir, wizardIsoPath)
			if err := b.Build(); err != nil {
				fmt.Printf("\n[ERROR] %v\n", err)
				offerCleanup(b, false)
				os.Exit(1)
			}
		}

		wizardIsoPath = relocateISO(wizardIsoPath, wizardWorkDir)
		printBuildSuccess(wizardIsoPath)
		offerCleanup(b, true)
		os.Exit(0)
	}

	deps := system.CheckDependencies()
	if len(deps.Missing) > 0 {
		fmt.Println("\n[ERROR] Absent build dependencies:")
		for _, dep := range deps.Missing {
			fmt.Printf("  - %s\n", dep)
		}
		fmt.Println("\nExecute: sudo kagami --install-deps")
		os.Exit(1)
	}

	var selectedRelease string
	releaseAliases := map[string]string{
		"lts":      "noble",
		"rolling":  "devel",
		"unstable": "devel",
	}

	releaseExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "release" {
			releaseExplicit = true
		}
	})

	inputRelease := *release
	if mapped, ok := releaseAliases[inputRelease]; ok {
		fmt.Printf("[INFO] Release alias '%s' resolved to '%s'\n", inputRelease, mapped)
		inputRelease = mapped
	}
	selectedRelease = inputRelease

	_, defaultWorkDir := system.GetAppPaths()

	var baseWorkDir string
	if *workDir != "" {
		baseWorkDir = *workDir
	} else if *configFile != "" {
		absPath, err := filepath.Abs(*configFile)
		if err != nil {
			fatal("Failed to resolve configuration file path: %v", err)
		}
		baseWorkDir = filepath.Join(filepath.Dir(absPath), "kagami-workspace")
	} else {
		baseWorkDir = defaultWorkDir
	}

	var isoPath string
	if *outputISO != "" {
		isoPath = *outputISO
	} else {
		isoPath = ""
	}

	var cfg *config.Config
	var err error

	if *configFile != "" {
		cfg, err = config.LoadFromFile(*configFile)
		if err != nil {
			fatal("Configuration loading failed: %v", err)
		}
		if releaseExplicit {
			cfg.Release = selectedRelease
		}
	} else {
		cfg = config.NewDefaultConfig(selectedRelease)
		cfg.System.Hostname = *hostname
		cfg.System.BlockSnapd = *noSnapd
	}

	if isoPath == "" {
		isoPath = filepath.Join(baseWorkDir, fmt.Sprintf("kagami-%s-%s.iso", cfg.Distro, cfg.Release))
	}

	if *mirrorURL != "" {
		cfg.Repository.Mirror = *mirrorURL
	}

	if err := cfg.Validate(); err != nil {
		fatal("Configuration validation failed: %v", err)
	}

	b := builder.NewBuilder(cfg, baseWorkDir, isoPath)
	printBuildInfo(cfg, baseWorkDir, isoPath)

	if *interactive {
		if err := b.InteractivePackageSelection(); err != nil {
			fatal("Interactive selection procedure failed: %v", err)
		}
	}

	if err := b.Build(); err != nil {
		fmt.Printf("\n[ERROR] %v\n", err)
		offerCleanup(b, false)
		os.Exit(1)
	}

	isoPath = relocateISO(isoPath, baseWorkDir)
	printBuildSuccess(isoPath)
	offerCleanup(b, true)
}

func printBuildInfo(cfg *config.Config, workDir, isoPath string) {
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("  %s %s - Debian/Ubuntu ISO Builder\n", config.AppName, config.Version)
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("\n[INFO] Build Parameters:\n")
	fmt.Printf("  Distribution: %s\n", cfg.Distro)
	fmt.Printf("  Release:      %s\n", cfg.Release)
	fmt.Printf("  Workspace:    %s\n", workDir)
	fmt.Printf("  Output ISO:   %s\n", isoPath)
	fmt.Printf("  Snapd Block:  %v\n", cfg.System.BlockSnapd)
	fmt.Printf("  Desktop:      %s\n", resolveDesktopLabel(cfg))
	fmt.Printf("  Installer:    %s\n", cfg.Installer.Type)
	fmt.Println()
}

func printBuildSuccess(isoPath string) {
	fmt.Println("\n---------------------------------------------------------------")
	fmt.Println("  [OK] Build process concluded successfully")
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("\n[OUTPUT] ISO path:  %s\n", isoPath)
	fmt.Printf("[OUTPUT] ISO size:  %s\n", computeFileSize(isoPath))
	fmt.Println("\n[INFO] Recommended next steps:")
	fmt.Println("  Virtualised validation: qemu-system-x86_64 -cdrom <iso> -m 2048")
	fmt.Println("  Physical media write:   sudo dd if=<iso> of=/dev/sdX bs=4M status=progress")
	fmt.Println()
}

func printBanner() {
	fmt.Printf("%s - Debian/Ubuntu ISO Builder %s\n", config.AppName, config.Version)
	fmt.Println("Vanilla Desktop Environment Synthesis")
	fmt.Println()
}

func resolveDesktopLabel(cfg *config.Config) string {
	if cfg.Packages.Desktop != "none" {
		return cfg.Packages.Desktop
	}
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

func computeFileSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "unknown"
	}
	size := float64(info.Size())
	units := []string{"B", "KB", "MB", "GB"}
	idx := 0
	for size >= 1024 && idx < len(units)-1 {
		size /= 1024
		idx++
	}
	return fmt.Sprintf("%.2f %s", size, units[idx])
}

func fatal(format string, v ...any) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] [FATAL] %s\n", timestamp, fmt.Sprintf(format, v...))
	os.Exit(1)
}

type formalLogger struct{}

func (l *formalLogger) Write(p []byte) (n int, err error) {
	timestamp := time.Now().Format("15:04:05")
	return fmt.Printf("[%s] [INFO] %s", timestamp, string(p))
}

func offerCleanup(b *builder.Builder, success bool) {
	reader := bufio.NewReader(os.Stdin)
	var input string

	if !success {
		fmt.Println("\n---------------------------------------------------------------")
		fmt.Println("  [WARNING] Build failed. Workspace cleanup is recommended.")
		fmt.Println("---------------------------------------------------------------")
		fmt.Println("  Unmounting filesystems prevents resource leakage.")
		fmt.Print("  Unmount and remove workspace? [Y/n]: ")
		input, _ = reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) != "n" {
			b.RemoveWorkspace()
		}
		return
	}

	fmt.Print("\nRemove build workspace and chroot? [y/N]: ")
	input, _ = reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(input)) == "y" {
		b.RemoveWorkspace()
	}
}

func relocateISO(isoPath, workDir string) string {
	absISO, _ := filepath.Abs(isoPath)
	absWork, _ := filepath.Abs(workDir)

	if !strings.HasPrefix(absISO, absWork) {
		return isoPath
	}

	execPath, err := os.Executable()
	if err != nil {
		return isoPath
	}
	destDir := filepath.Dir(execPath)

	if strings.Contains(destDir, "/usr/bin") || strings.Contains(destDir, "/bin") {
		destDir, _ = os.Getwd()
	}

	newPath := filepath.Join(destDir, filepath.Base(isoPath))
	if absISO == newPath {
		return isoPath
	}

	fmt.Printf("\n[INFO] Relocating ISO to: %s\n", newPath)
	if err := os.Rename(isoPath, newPath); err != nil {
		cmd := exec.Command("cp", isoPath, newPath)
		if err := cmd.Run(); err != nil {
			fmt.Printf("[WARNING] ISO relocation failed: %v\n", err)
			return isoPath
		}
		os.Remove(isoPath)
	}

	return newPath
}
