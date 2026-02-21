package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	isSystemPath := strings.HasPrefix(execDir, "/usr/bin") ||
		strings.HasPrefix(execDir, "/bin") ||
		strings.HasPrefix(execDir, "/usr/local/bin")

	if isSystemPath {
		homeDir, _ := os.UserHomeDir()
		base := filepath.Join(homeDir, "kagami")
		return filepath.Join(base, "config"), filepath.Join(base, "workspace")
	}

	return execDir, filepath.Join(execDir, "kagami-workspace")
}

type Dependencies struct {
	Required  []string
	Missing   []string
	Installed []string
}

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

func IsAPTBased() bool {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return false
	}

	if _, err := exec.LookPath("dpkg"); err != nil {
		return false
	}

	if _, err := os.Stat("/etc/apt"); err != nil {
		return false
	}

	releaseFiles := []string{
		"/etc/os-release",
		"/etc/debian_version",
		"/etc/lsb-release",
	}

	for _, file := range releaseFiles {
		if content, err := os.ReadFile(file); err == nil {
			contentStr := strings.ToLower(string(content))
			if (strings.Contains(contentStr, "ubuntu") || strings.Contains(contentStr, "debian")) &&
				!strings.Contains(contentStr, "altlinux") {
				return true
			}
		}
	}

	if _, err := os.Stat("/etc/debian_version"); err == nil {
		return true
	}

	return false
}

func IsContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}

	if os.Getenv("DISTROBOX_ENTER_PATH") != "" {
		return true
	}

	return false
}

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

func isPackageInstalled(packageName string) bool {
	cmd := exec.Command("dpkg", "-s", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "Status: install ok installed")
}

func GetInstallCommand(packages []string) string {
	return strings.Join(packages, " ")
}

func InstallDependencies() error {
	deps := CheckDependencies()

	if len(deps.Missing) == 0 {
		fmt.Println("[OK] All build dependencies are already present")
		return nil
	}

	fmt.Printf("[INFO] Installing %d packages: %s\n", len(deps.Missing), strings.Join(deps.Missing, ", "))

	fmt.Println("[INFO] Updating package index...")
	updateCmd := exec.Command("apt-get", "update")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("package index update failed: %v", err)
	}

	fmt.Println("[INFO] Installing required packages...")
	installArgs := append([]string{"install", "-y"}, deps.Missing...)
	installCmd := exec.Command("apt-get", installArgs...)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	installCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("package installation failed: %v", err)
	}

	fmt.Println("[INFO] Verifying installation...")
	afterDeps := CheckDependencies()
	if len(afterDeps.Missing) > 0 {
		return fmt.Errorf("the following packages failed to install: %s", strings.Join(afterDeps.Missing, ", "))
	}

	return nil
}

func GetSystemInfo() (map[string]string, error) {
	info := make(map[string]string)

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

	if output, err := exec.Command("uname", "-r").Output(); err == nil {
		info["Kernel"] = strings.TrimSpace(string(output))
	}

	if output, err := exec.Command("uname", "-m").Output(); err == nil {
		info["Architecture"] = strings.TrimSpace(string(output))
	}

	return info, nil
}

func CheckMinimumRequirements() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("elevated privileges are required")
	}
	return nil
}
