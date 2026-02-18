package system

// NOTE for non-APT distribution users:
// If you are running Kagami on a non-APT system (e.g., Fedora, Arch, openSUSE),
// it is recommended to run it inside a Distrobox container (Ubuntu/Debian)
// with your home folder mapped to ensure proper file access and workspace management.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetAppPaths returns the base directory for configuration and workspace
// based on whether the application is running in portable mode or system-installed mode.
func GetAppPaths() (configDir, workDir string) {
	execPath, err := os.Executable()
	if err != nil {
		execPath = "."
	}

	absExecPath, err := filepath.Abs(execPath)
	if err != nil {
		absExecPath = execPath
	}

	execDir := filepath.Dir(absExecPath)
	// Check if running from system binaries
	isSystemPath := strings.HasPrefix(execDir, "/usr/bin") || strings.HasPrefix(execDir, "/bin") || strings.HasPrefix(execDir, "/usr/local/bin")

	if isSystemPath {
		homeDir, _ := os.UserHomeDir()
		base := filepath.Join(homeDir, "kagami")
		return filepath.Join(base, "config"), filepath.Join(base, "workspace")
	}

	// Portable mode: use executable directory
	return execDir, filepath.Join(execDir, "kagami-workspace")
}

// Dependencies represents system dependencies status
type Dependencies struct {
	Required  []string
	Missing   []string
	Installed []string
}

// Required packages for Kagami to function
var requiredPackages = []string{
	"debootstrap",
	"squashfs-tools",
	"xorriso",
	"grub-pc-bin",
	"grub-efi-amd64-bin",
	"mtools",
	"dosfstools",
	"isolinux",
	"syslinux",
	"syslinux-common",
}

// IsAPTBased checks if the system is APT-based (Ubuntu/Debian)
func IsAPTBased() bool {
	// Check for apt-get command
	if _, err := exec.LookPath("apt-get"); err != nil {
		return false
	}

	// Check for dpkg (Crucial for DEB-based APT)
	if _, err := exec.LookPath("dpkg"); err != nil {
		return false
	}

	// Check for rpm (If present alongside apt, it's likely an APT-RPM system like ALT Linux)
	if _, err := exec.LookPath("rpm"); err == nil {
		// Verify if it's explicitly ALT Linux
		if content, err := os.ReadFile("/etc/os-release"); err == nil {
			contentStr := strings.ToLower(string(content))
			if strings.Contains(contentStr, "altlinux") || strings.Contains(contentStr, "alt linux") {
				return false
			}
		}
	}

	// Check for sources.list
	if _, err := os.Stat("/etc/apt/sources.list"); err != nil {
		return false
	}

	// Check for Debian/Ubuntu release files
	hasDebianRelease := false
	releaseFiles := []string{
		"/etc/debian_version",
		"/etc/lsb-release",
		"/etc/os-release",
	}

	for _, file := range releaseFiles {
		if _, err := os.Stat(file); err == nil {
			// Read and check if it's Debian/Ubuntu based
			content, err := os.ReadFile(file)
			if err == nil {
				contentStr := strings.ToLower(string(content))
				// Ensure it's strictly Debian or Ubuntu family and NOT an RPM-based variant
				if (strings.Contains(contentStr, "ubuntu") || strings.Contains(contentStr, "debian")) &&
					!strings.Contains(contentStr, "altlinux") {
					hasDebianRelease = true
					break
				}
			}
		}
	}

	return hasDebianRelease
}

// IsContainer checks if the program is running inside a container (Docker, Podman, Distrobox)
func IsContainer() bool {
	// Check for .dockerenv (Standard Docker)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check for .containerenv (Podman/Distrobox)
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}

	// Check /proc/1/sched (If it contains 'init', it's likely a container or init system)
	if content, err := os.ReadFile("/proc/1/sched"); err == nil {
		if !strings.Contains(string(content), "init") && !strings.Contains(string(content), "systemd") {
			// This is not foolproof but a good hint
		}
	}

	// Distrobox specific environment variable
	if os.Getenv("DISTROBOX_ENTER_PATH") != "" {
		return true
	}

	return false
}

// CheckDependencies checks which required packages are installed
func CheckDependencies() Dependencies {
	deps := Dependencies{
		Required:  requiredPackages,
		Missing:   []string{},
		Installed: []string{},
	}

	for _, pkg := range requiredPackages {
		if isPackageInstalled(pkg) {
			deps.Installed = append(deps.Installed, pkg)
		} else {
			deps.Missing = append(deps.Missing, pkg)
		}
	}

	return deps
}

// isPackageInstalled checks if a package is installed using dpkg
func isPackageInstalled(packageName string) bool {
	cmd := exec.Command("dpkg", "-s", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	// Check if package is installed
	outputStr := string(output)
	return strings.Contains(outputStr, "Status: install ok installed")
}

// GetInstallCommand returns the apt-get install command for missing packages
func GetInstallCommand(packages []string) string {
	return strings.Join(packages, " ")
}

// InstallDependencies installs all missing dependencies
func InstallDependencies() error {
	deps := CheckDependencies()

	if len(deps.Missing) == 0 {
		fmt.Println("[SUCCESS] All dependencies already installed")
		return nil
	}

	fmt.Printf("[INFO] Installing %d packages: %s\n", len(deps.Missing), strings.Join(deps.Missing, ", "))

	// Update package lists
	fmt.Println("[ACTION] Updating package lists...")
	updateCmd := exec.Command("apt-get", "update")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("failed to update package lists: %v", err)
	}

	// Install packages
	fmt.Println("[ACTION] Installing packages...")
	installArgs := append([]string{"install", "-y"}, deps.Missing...)
	installCmd := exec.Command("apt-get", installArgs...)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	installCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install packages: %v", err)
	}

	// Verify installation
	fmt.Println("[ACTION] Verifying installation...")
	afterDeps := CheckDependencies()
	if len(afterDeps.Missing) > 0 {
		return fmt.Errorf("some packages failed to install: %s", strings.Join(afterDeps.Missing, ", "))
	}

	return nil
}

// GetSystemInfo returns information about the host system
func GetSystemInfo() (map[string]string, error) {
	info := make(map[string]string)

	// Get distribution info from lsb_release
	if output, err := exec.Command("lsb_release", "-a").Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				info[key] = value
			}
		}
	}

	// Get kernel version
	if output, err := exec.Command("uname", "-r").Output(); err == nil {
		info["Kernel"] = strings.TrimSpace(string(output))
	}

	// Get architecture
	if output, err := exec.Command("uname", "-m").Output(); err == nil {
		info["Architecture"] = strings.TrimSpace(string(output))
	}

	return info, nil
}

// CheckMinimumRequirements checks if the system meets minimum requirements
func CheckMinimumRequirements() error {
	// Check available disk space (need at least 10GB)
	// This is a simple check - in production you'd want more sophisticated checks

	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("must be run as root")
	}

	return nil
}
