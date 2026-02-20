package builder

// NOTE for non-APT distribution users:
// If you are running Kagami on a non-APT system (e.g., Fedora, Arch, openSUSE),
// it is recommended to run it inside a Distrobox container (Ubuntu/Debian)
// with your home folder mapped to ensure proper file access and workspace management.

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"kagami/pkg/config"
	"kagami/pkg/system"
)

// Builder handles the ISO building process
type Builder struct {
	Config      *config.Config
	WorkDir     string
	OutputISO   string
	ChrootDir   string
	ImageDir    string
	DebianAlias string // stable, testing, or unstable
	PrettyName  string // e.g. "Debian GNU/Linux 12"
}

// NewBuilder creates a new Builder instance
func NewBuilder(cfg *config.Config, workDir, outputISO string) *Builder {
	return &Builder{
		Config:    cfg,
		WorkDir:   workDir,
		OutputISO: outputISO,
		ChrootDir: filepath.Join(workDir, "chroot"),
		ImageDir:  filepath.Join(workDir, "image"),
	}
}

// isDebian returns true if the release is a Debian release
func (b *Builder) isDebian() bool {
	return b.Config.Distro == "debian"
}

// getDistName returns the descriptive name of the distribution
func (b *Builder) getDistName() string {
	if b.PrettyName != "" {
		return b.PrettyName
	}

	if b.isDebian() {
		if b.DebianAlias != "" {
			return "Debian " + strings.Title(b.DebianAlias)
		}
		return "Debian (" + b.Config.Release + ")"
	}

	switch b.Config.Release {
	case "focal", "jammy", "noble", "resolute":
		return "Ubuntu LTS"
	case "devel":
		return "Ubuntu Rolling"
	default:
		return "Ubuntu Custom"
	}
}

// Build executes the complete build process
func (b *Builder) Build() error {
	// Resolve Debian aliases to codenames if needed
	b.resolveDebianRelease()

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Checking prerequisites", b.checkPrerequisites},
		{"Creating directories", b.createDirectories},
		{"Bootstrapping base system", b.bootstrapSystem},
		{"Mounting filesystems", b.mountFilesystems},
		{"Configuring system", b.configureSystem},
		{"Blocking snapd permanently", b.blockSnapd},
		{"Installing packages", b.installPackages},
		{"Installing desktop environment", b.installDesktop},

		{"Configuring Flatpak", b.setupFlatpak},
		{"Configuring bootloader", b.configureBootloader},
		{"Cleaning up chroot", b.cleanupChroot},
		{"Creating filesystem image", b.createFilesystem},
		{"Creating ISO", b.createISO},
		{"Cleaning up", b.cleanup},
	}

	for i, step := range steps {
		fmt.Printf("[%d/%d] %s...\n", i+1, len(steps), step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s failed: %v", step.name, err)
		}
	}

	return nil
}

// checkPrerequisites verifies required tools are installed
func (b *Builder) checkPrerequisites() error {
	required := []string{
		"debootstrap",
		"mksquashfs",
		"xorriso",
		"grub-mkstandalone",
		"gpg",
		"wget",
	}

	for _, tool := range required {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("required tool '%s' not found. Install with: sudo apt-get install %s", tool, tool)
		}
	}

	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("this program must be run as root (use sudo)")
	}

	// Environment-specific tips
	if system.IsContainer() {
		fmt.Println("[INFO] Container environment detected (Docker/Podman/Distrobox).")
		fmt.Println("       Ensure the container has SYS_ADMIN privileges for mounting.")
	}

	return nil
}

// createDirectories creates necessary directory structure
func (b *Builder) createDirectories() error {
	dirs := []string{
		b.WorkDir,
		b.ChrootDir,
		b.ImageDir,
		filepath.Join(b.ImageDir, "casper"),
		filepath.Join(b.ImageDir, "isolinux"),
		filepath.Join(b.ImageDir, "install"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// bootstrapSystem creates the base OS system using debootstrap
func (b *Builder) bootstrapSystem() error {
	// Check if chroot already exists
	if _, err := os.Stat(filepath.Join(b.ChrootDir, "etc")); err == nil {
		log.Println("Chroot already exists, skipping bootstrap")
		return nil
	}

	mirror := b.Config.Repository.Mirror
	if mirror == "" {
		if b.isDebian() {
			mirror = "http://deb.debian.org/debian/"
		} else {
			mirror = "http://archive.ubuntu.com/ubuntu/"
		}
	}

	// If running devel, we bootstrap the latest LTS first
	bootstrapRelease := b.Config.Release
	if bootstrapRelease == "devel" {
		bootstrapRelease = "noble" // Latest LTS
		fmt.Println("[INFO] Bootstrapping with 'noble' (LTS) for devel target")
	}

	cmd := exec.Command("debootstrap",
		"--arch="+b.Config.System.Architecture,
		"--variant=minbase",
		bootstrapRelease,
		b.ChrootDir,
		mirror,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if system.IsContainer() {
			return fmt.Errorf("debootstrap failed: %v\n[TIP] In a container, debootstrap requires '--privileged' or 'CAP_MKNOD' to create device nodes.", err)
		}
		return err
	}
	return nil
}

// mountFilesystems mounts necessary filesystems for chroot
func (b *Builder) mountFilesystems() error {
	// ... (code omitted for brevity, no changes needed here but context helps)
	mounts := []struct {
		source string
		target string
		fstype string
		flags  string
	}{
		{"/dev", filepath.Join(b.ChrootDir, "dev"), "", "bind"},
		{"/run", filepath.Join(b.ChrootDir, "run"), "", "bind"},
	}

	for _, m := range mounts {
		target := m.target

		// Check if already mounted
		if isMounted(target) {
			continue
		}

		// Ensure target directory exists
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("failed to create mount target %s: %v", target, err)
		}

		var cmd *exec.Cmd
		if m.flags == "bind" {
			cmd = exec.Command("mount", "--bind", m.source, target)
		} else {
			cmd = exec.Command("mount", "-t", m.fstype, m.source, target)
		}

		if err := cmd.Run(); err != nil {
			errMsg := fmt.Errorf("failed to mount %s: %v", target, err)
			if system.IsContainer() {
				return fmt.Errorf("%v\n[TIP] In a container, this usually requires '--privileged' or 'CAP_SYS_ADMIN'.", errMsg)
			}
			return errMsg
		}
	}

	return nil
}

// configureSystem performs basic system configuration
func (b *Builder) configureSystem() error {
	scripts := []string{
		// Set hostname
		fmt.Sprintf("echo '%s' > /etc/hostname", b.Config.System.Hostname),

		// Configure apt sources
		b.generateSourcesList(),

		// Mount internal filesystems
		"mount none -t proc /proc 2>/dev/null || true",
		"mount none -t sysfs /sys 2>/dev/null || true",
		"mount none -t devpts /dev/pts 2>/dev/null || true",

		// Set environment
		"export HOME=/root",
		"export LC_ALL=C",
	}

	for _, script := range scripts {
		if err := b.chrootExec(script); err != nil {
			return err
		}
	}

	// Add additional repositories with keys (Ubuntu specific)
	if err := b.configureAdditionalRepos(); err != nil {
		log.Printf("Warning: Failed to configure additional repos: %v", err)
	}

	// Continue with rest of configuration
	moreScripts := []string{
		// Update package lists
		"apt-get update",

		// Install systemd
		"DEBIAN_FRONTEND=noninteractive apt-get install -y libterm-readline-gnu-perl systemd-sysv",

		// Configure machine-id
		"dbus-uuidgen > /etc/machine-id",
		"ln -fs /etc/machine-id /var/lib/dbus/machine-id",

		// Setup diversion for initctl
		"dpkg-divert --local --rename --add /sbin/initctl",
		"ln -s /bin/true /sbin/initctl",
	}

	for _, script := range moreScripts {
		if err := b.chrootExec(script); err != nil {
			return err
		}
	}

	return nil
}

// installPackages installs all required packages
func (b *Builder) installPackages() error {
	// Upgrade existing packages
	if err := b.chrootExec("DEBIAN_FRONTEND=noninteractive apt-get -y dist-upgrade"); err != nil {
		return err
	}

	// Install essential packages
	essentialPkgs := strings.Join(b.Config.Packages.Essential, " ")
	if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", essentialPkgs)); err != nil {
		return err
	}

	// Install kernel and headers
	kernelPkg := b.Config.Packages.Kernel
	if kernelPkg == "" {
		kernelPkg = "linux-generic"
		if b.isDebian() {
			kernelPkg = "linux-image-amd64"
			if b.Config.System.Architecture == "arm64" {
				kernelPkg = "linux-image-arm64"
			}
		}
	}

	// Prepare headers package name
	headersPkg := ""
	if strings.HasPrefix(kernelPkg, "linux-image-") {
		headersPkg = strings.Replace(kernelPkg, "linux-image-", "linux-headers-", 1)
	} else if strings.HasPrefix(kernelPkg, "linux-") {
		headersPkg = "linux-headers-" + strings.TrimPrefix(kernelPkg, "linux-")
	}

	installKernelCmd := fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends %s", kernelPkg)
	if headersPkg != "" {
		installKernelCmd += " " + headersPkg
	}

	if err := b.chrootExec(installKernelCmd); err != nil {
		return err
	}

	// Install additional packages
	if len(b.Config.Packages.Additional) > 0 {
		additionalPkgs := strings.Join(b.Config.Packages.Additional, " ")
		if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", additionalPkgs)); err != nil {
			log.Printf("Warning: Some additional packages failed to install: %v", err)
		}
	}

	return nil
}

// blockSnapd implements comprehensive snapd blocking
func (b *Builder) blockSnapd() error {
	if !b.Config.Security.BlockSnapdForever && !b.Config.System.BlockSnapd {
		return nil
	}

	fmt.Println("[ACTION] Implementing comprehensive snapd blocking...")

	scripts := []string{
		// Remove snapd packages if present
		"apt-get purge -y snapd snap-confine ubuntu-core-launcher snapd-xdg-open || true",
		"apt-get autoremove -y || true",
		"rm -rf /var/cache/snapd /var/lib/snapd /var/snap /snap",

		// Create APT preference to block snapd (refined for Ubuntu/Debian)
		`cat > /etc/apt/preferences.d/nosnapd.pref <<'EOF'
# Snapd is permanently blocked on this system
Explanation: Snapd is permanently blocked on this system to prevent unwanted installation.
Package: snapd
Pin: release *
Pin-Priority: -1

Package: snapd:*
Pin: release *
Pin-Priority: -1

Package: snapd-unwrapped
Pin: release *
Pin-Priority: -1

Package: snap-confine
Pin: release *
Pin-Priority: -1

Package: ubuntu-core-launcher
Pin: release *
Pin-Priority: -1

Package: snapd-xdg-open
Pin: release *
Pin-Priority: -1
EOF`,

		// Create systemd drop-in to prevent snapd service activation
		"mkdir -p /etc/systemd/system/snapd.service.d",
		`cat > /etc/systemd/system/snapd.service.d/override.conf <<'EOF'
[Unit]
# Snapd is permanently disabled on this system
ConditionPathExists=!/etc/snapd-blocked

[Service]
ExecStart=
ExecStart=/bin/false
EOF`,

		// Create marker file
		"touch /etc/snapd-blocked",
		`echo "Snapd is permanently blocked on this system" > /etc/snapd-blocked`,

		// Block snapd socket
		"mkdir -p /etc/systemd/system/snapd.socket.d",
		`cat > /etc/systemd/system/snapd.socket.d/override.conf <<'EOF'
[Unit]
ConditionPathExists=!/etc/snapd-blocked

[Socket]
ListenStream=
EOF`,

		// Create hook to prevent snapd installation via apt
		"mkdir -p /etc/apt/apt.conf.d",
		`cat > /etc/apt/apt.conf.d/99-block-snapd <<'EOF'
// Block snapd package installation
DPkg::Pre-Install-Pkgs {
  "/usr/local/bin/block-snapd-hook";
};
EOF`,

		// Create the blocking hook script
		`cat > /usr/local/bin/block-snapd-hook <<'EOFSCRIPT'
#!/bin/bash
# Hook to prevent snapd installation

while read pkg; do
    if [[ "$pkg" == *"snapd"* ]]; then
        echo "==========================================================" >&2
        echo "ERROR: Installation of snapd is BLOCKED on this system!" >&2
        echo "This distribution is configured to never use snapd." >&2
        echo "==========================================================" >&2
        exit 1
    fi
done
EOFSCRIPT`,

		"chmod +x /usr/local/bin/block-snapd-hook",

		// Add warning to MOTD
		"mkdir -p /etc/update-motd.d",
		`cat > /etc/update-motd.d/99-snapd-blocked <<'EOF'
#!/bin/sh
echo ""
echo "-----------------------------------------------------------"
echo "  WARNING: Snapd is permanently blocked on this system     "
echo "  Snap packages cannot be installed or used                "
echo "-----------------------------------------------------------"
echo ""
EOF`,

		"chmod +x /etc/update-motd.d/99-snapd-blocked",

		// Create diversion for snapd package
		"dpkg-divert --local --rename --add /usr/bin/snap || true",
		"ln -sf /bin/false /usr/bin/snap || true",

		// Remove any snap directories
		"rm -rf /snap /var/snap /var/lib/snapd ~/snap || true",

		// Add snapd blocking to profile
		`cat >> /etc/profile.d/block-snapd.sh <<'EOF'
# Snapd is blocked on this system
export SNAPD_BLOCKED=1

# Override snap command
snap() {
    echo "ERROR: Snapd is permanently blocked on this system" >&2
    return 1
}
EOF`,

		"chmod +x /etc/profile.d/block-snapd.sh",
	}

	for _, script := range scripts {
		if err := b.chrootExec(script); err != nil {
			log.Printf("Warning during snapd blocking: %v", err)
		}
	}

	fmt.Println("[SUCCESS] Snapd blocked permanently with multiple layers of protection")
	return nil
}

// installDesktop installs the selected desktop environment
func (b *Builder) installDesktop() error {
	// If desktop is "none", user is manually specifying packages in "additional"
	// We just need to ensure ubiquity installer is installed
	if b.Config.Packages.Desktop == "none" {
		log.Println("Desktop set to 'none' - packages should be specified in 'additional' list")

		// Install ubiquity if requested
		if b.Config.Installer.Type == "ubiquity" {
			installerPkgs := []string{
				"ubiquity",
				"ubiquity-casper",
				"ubiquity-frontend-gtk",
				"ubiquity-ubuntu-artwork",
			}

			// Add slideshow package based on config
			slideshowMap := map[string]string{
				"ubuntu":      "ubiquity-slideshow-ubuntu",
				"kubuntu":     "ubiquity-slideshow-kubuntu",
				"xubuntu":     "ubiquity-slideshow-xubuntu",
				"lubuntu":     "ubiquity-slideshow-lubuntu",
				"ubuntu-mate": "ubiquity-slideshow-ubuntu-mate",
			}

			if slideshow, ok := slideshowMap[b.Config.Installer.Slideshow]; ok {
				installerPkgs = append(installerPkgs, slideshow)
			}

			installerList := strings.Join(installerPkgs, " ")
			if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", installerList)); err != nil {
				log.Printf("Warning: Some installer packages failed to install: %v", err)
			}
		}

		// Setup Calamares if requested
		if b.Config.Installer.Type == "calamares" {
			if err := b.setupCalamares(); err != nil {
				log.Printf("Warning: Failed to setup Calamares: %v", err)
			}
		}

		// Remove unwanted packages
		if len(b.Config.Packages.RemoveList) > 0 {
			removeList := strings.Join(b.Config.Packages.RemoveList, " ")
			b.chrootExec(fmt.Sprintf("apt-get purge -y %s || true", removeList))
		}

		b.chrootExec("apt-get autoremove -y")
		return nil
	}

	desktopPackages := map[string][]string{
		"gnome": {
			"vanilla-gnome-desktop",
			"vanilla-gnome-default-settings",
			"gnome-session",
			"gnome-tweaks",
			"gnome-shell-extension-manager",
			"gnome-backgrounds",
			"fonts-cantarell",
			"adwaita-icon-theme",
			"plymouth-themes",
		},
		"kde": {
			"kde-plasma-desktop",
		},
		"xfce": {
			"xfce4",
			"xfce4-goodies",
		},
		"lxde": {
			"lxde",
		},
		"lxqt": {
			"lxqt",
		},
		"mate": {
			"mate-desktop-environment",
		},
	}

	// Override for Debian
	if b.isDebian() {
		desktopPackages = map[string][]string{
			"gnome": {
				"task-gnome-desktop",
			},
			"kde": {
				"task-kde-desktop",
			},
			"xfce": {
				"task-xfce-desktop",
			},
			"lxde": {
				"task-lxde-desktop",
			},
			"lxqt": {
				"task-lxqt-desktop",
			},
			"mate": {
				"task-mate-desktop",
			},
		}
	}

	pkgs, ok := desktopPackages[b.Config.Packages.Desktop]
	if !ok {
		return fmt.Errorf("unknown desktop environment: %s", b.Config.Packages.Desktop)
	}

	pkgList := strings.Join(pkgs, " ")
	installCmd := "DEBIAN_FRONTEND=noninteractive apt-get install -y"
	if b.isDebian() {
		installCmd += " --no-install-recommends"
	}

	if err := b.chrootExec(fmt.Sprintf("%s %s", installCmd, pkgList)); err != nil {
		return err
	}

	// Ensure the correct installer is installed based on config
	if b.Config.Installer.Type == "ubiquity" {
		installerPkgs := []string{
			"ubiquity",
			"ubiquity-casper",
			"ubiquity-frontend-gtk",
			"ubiquity-ubuntu-artwork",
		}

		installerList := strings.Join(installerPkgs, " ")
		if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends %s", installerList)); err != nil {
			log.Printf("Warning: Some installer packages failed to install: %v", err)
		}

		// Remove unnecessary slideshows
		slideshowsToRemove := []string{
			"ubiquity-slideshow-kubuntu",
			"ubiquity-slideshow-xubuntu",
			"ubiquity-slideshow-lubuntu",
			"ubiquity-slideshow-ubuntu-mate",
			"ubiquity-slideshow-ubuntu-budgie",
			"ubiquity-slideshow-ubuntu",
		}

		purgeList := strings.Join(slideshowsToRemove, " ")
		b.chrootExec(fmt.Sprintf("apt-get purge -y %s || true", purgeList))
	}

	// Install calamares if requested (for non-'none' desktop)
	if b.Config.Installer.Type == "calamares" {
		if err := b.setupCalamares(); err != nil {
			log.Printf("Warning: Failed to setup Calamares: %v", err)
		}
	}

	// Remove unwanted packages
	if len(b.Config.Packages.RemoveList) > 0 {
		removeList := strings.Join(b.Config.Packages.RemoveList, " ")
		b.chrootExec(fmt.Sprintf("apt-get purge -y %s || true", removeList))
	}

	// Cleanup
	b.chrootExec("apt-get autoremove -y")

	// GNOME-specific vanilla improvements (inspired by ubuntu-debullshit)
	if b.Config.Packages.Desktop == "gnome" && !b.isDebian() {
		b.refineVanillaGNOME()
	}

	return nil
}

// refineVanillaGNOME performs additional steps to ensure a vanilla GNOME experience on Ubuntu
func (b *Builder) refineVanillaGNOME() error {
	fmt.Println("[ACTION] Refining vanilla GNOME experience (Ubuntu)...")

	scripts := []string{
		// Remove Ubuntu-specific branding and themes
		"apt-get purge -y ubuntu-session yaru-theme-gnome-shell yaru-theme-gtk yaru-theme-icon yaru-theme-sound || true",

		// Ensure GNOME session is default
		"update-alternatives --set gdm3-theme.desktop /usr/share/gnome-shell/theme/gnome-shell.css || true",

		// Additional integration packages
		"DEBIAN_FRONTEND=noninteractive apt-get install -y qgnomeplatform-qt5 qgnomeplatform-qt6 || true",
	}

	for _, script := range scripts {
		if err := b.chrootExec(script); err != nil {
			log.Printf("Warning during GNOME refinement: %v", err)
		}
	}
	return nil
}

// setupFlatpak checks if Flatpak is enabled and installs it
func (b *Builder) setupFlatpak() error {
	if !b.Config.Packages.EnableFlatpak {
		return nil
	}
	return b.installFlatpak()
}

// installFlatpak installs Flatpak and adds Flathub repository
func (b *Builder) installFlatpak() error {
	fmt.Println("[ACTION] Installing Flatpak and adding Flathub repository...")

	pkgs := []string{"flatpak"}

	// Add desktop-specific plugins only for GNOME and KDE
	switch b.Config.Packages.Desktop {
	case "gnome":
		pkgs = append(pkgs, "gnome-software-plugin-flatpak")
	case "kde":
		pkgs = append(pkgs, "plasma-discover-backend-flatpak")
	}

	pkgList := strings.Join(pkgs, " ")
	if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", pkgList)); err != nil {
		return err
	}

	// Add Flathub repository mandatorily
	return b.chrootExec("flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo")
}

// configureBootloader sets up GRUB and creates boot files
func (b *Builder) configureBootloader() error {
	// Determine kernel suffix for better matching
	suffix := "generic"
	if b.isDebian() {
		suffix = "amd64"
	}

	if b.Config.Packages.Kernel != "" {
		if strings.Contains(b.Config.Packages.Kernel, "lowlatency") {
			suffix = "lowlatency"
		} else if strings.Contains(b.Config.Packages.Kernel, "oem") {
			suffix = "oem"
		} else if strings.Contains(b.Config.Packages.Kernel, "amd64") {
			suffix = "amd64"
		} else if strings.Contains(b.Config.Packages.Kernel, "rt") {
			suffix = "rt"
		}
	}

	kernelPattern := filepath.Join(b.ChrootDir, "boot", "vmlinuz-*"+suffix+"*")
	initrdPattern := filepath.Join(b.ChrootDir, "boot", "initrd.img-*"+suffix+"*")

	// Fallback to broad pattern if specific one fails
	kernels, _ := filepath.Glob(kernelPattern)
	if len(kernels) == 0 {
		kernelPattern = filepath.Join(b.ChrootDir, "boot", "vmlinuz-*")
		initrdPattern = filepath.Join(b.ChrootDir, "boot", "initrd.img-*")
		kernels, _ = filepath.Glob(kernelPattern)
	}
	initrds, _ := filepath.Glob(initrdPattern)

	if len(kernels) > 0 {
		// Sort or pick the latest? For simplicity, pick the first match for now
		exec.Command("cp", kernels[0], filepath.Join(b.ImageDir, "casper", "vmlinuz")).Run()
	}
	if len(initrds) > 0 {
		exec.Command("cp", initrds[0], filepath.Join(b.ImageDir, "casper", "initrd")).Run()
	}

	// Download memtest86+
	memtestURL := "https://memtest.org/download/v7.00/mt86plus_7.00.binaries.zip"
	memtestZip := filepath.Join(b.ImageDir, "install", "memtest86.zip")

	exec.Command("wget", "--progress=dot", memtestURL, "-O", memtestZip).Run()
	exec.Command("unzip", "-p", memtestZip, "memtest64.bin").
		Output() // We'll handle output separately
	exec.Command("unzip", "-p", memtestZip, "memtest64.efi").
		Output()
	exec.Command("rm", "-f", memtestZip).Run()

	// Create distribution marker file
	markerFile := filepath.Join(b.ImageDir, "kagami-live")
	os.WriteFile(markerFile, []byte(""), 0644)

	// Create GRUB configuration
	grubCfg := filepath.Join(b.ImageDir, "isolinux", "grub.cfg")
	grubContent := b.generateGrubConfig()
	if err := os.WriteFile(grubCfg, []byte(grubContent), 0644); err != nil {
		return err
	}

	return nil
}

// generateGrubConfig creates GRUB menu configuration
func (b *Builder) generateGrubConfig() string {
	distName := b.getDistName()

	installEntry := ""
	switch b.Config.Installer.Type {
	case "ubiquity":
		installEntry = fmt.Sprintf(`
menuentry "Install %s" {
   linux /casper/vmlinuz boot=casper only-ubiquity quiet splash ---
   initrd /casper/initrd
}`, distName)
	case "calamares":
		installEntry = fmt.Sprintf(`
menuentry "Install %s" {
   linux /casper/vmlinuz boot=casper quiet splash ---
   initrd /casper/initrd
}`, distName)
	}

	return fmt.Sprintf(`
search --set=root --file /kagami-live

insmod all_video
insmod part_gpt
insmod part_msdos
insmod fat
insmod iso9660

set default="0"
set timeout=30

menuentry "Try %s without installing" {
   linux /casper/vmlinuz boot=casper nopersistent toram quiet splash ---
   initrd /casper/initrd
}
%s

menuentry "Check disc for defects" {
   linux /casper/vmlinuz boot=casper integrity-check quiet splash ---
   initrd /casper/initrd
}

if [ "$grub_platform" = "efi" ]; then
menuentry 'UEFI Firmware Settings' {
   fwsetup
}

menuentry "Test memory Memtest86+ (UEFI)" {
   linux /install/memtest86+.efi
}
else
menuentry "Test memory Memtest86+ (BIOS)" {
   linux16 /install/memtest86+.bin
}
fi

`, distName, installEntry)
}

// cleanupChroot performs cleanup before creating filesystem image
func (b *Builder) cleanupChroot() error {
	scripts := []string{
		"truncate -s 0 /etc/machine-id",
		"rm -f /sbin/initctl",
		"dpkg-divert --rename --remove /sbin/initctl",
		"apt-get clean",
		"rm -rf /tmp/* ~/.bash_history",
		"umount /proc || true",
		"umount /sys || true",
		"umount /dev/pts || true",
	}

	for _, script := range scripts {
		b.chrootExec(script)
	}

	return nil
}

// applyCalamaresConfig copies custom Calamares configuration to the chroot
func (b *Builder) applyCalamaresConfig() error {
	if b.Config.Installer.CalamaresConfig == "" {
		return nil
	}

	fmt.Printf("[ACTION] Applying custom Calamares configuration from %s...\n", b.Config.Installer.CalamaresConfig)

	// Source path on host
	srcPath := b.Config.Installer.CalamaresConfig
	if !filepath.IsAbs(srcPath) {
		cwd, _ := os.Getwd()
		srcPath = filepath.Join(cwd, srcPath)
	}

	// Target path in chroot (usually /etc/calamares)
	destPath := filepath.Join(b.ChrootDir, "etc", "calamares")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create calamares config directory: %v", err)
	}

	// Copy files
	// Note: We use shell to handle globbing if source is a directory content
	cmd := exec.Command("bash", "-c", fmt.Sprintf("cp -rv %s/* %s/", srcPath, destPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// configureMinimalInstaller sets up the ALCI-style minimal live environment
// where a lightweight WM auto-starts and Calamares is launched automatically.
// This mimics the approach used by ALCI (Arch Linux Calamares Installer):
//   - LightDM autologin to a 'live' user
//   - XDG autostart .desktop file to launch Calamares
//   - Openbox autostart (if openbox is the WM)
//   - Polkit rule so Calamares can run without manual auth prompt
func (b *Builder) configureMinimalInstaller() error {
	fmt.Println("[ACTION] Configuring minimal installer environment (ALCI-style)...")

	scripts := []string{
		// Create a live user for the live session
		"useradd -m -G sudo -s /bin/bash live || true",
		"echo 'live:live' | chpasswd",
		"echo 'live ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/live",

		// Configure LightDM autologin
		"mkdir -p /etc/lightdm/lightdm.conf.d",
		`cat > /etc/lightdm/lightdm.conf.d/50-autologin.conf <<'EOF'
[Seat:*]
autologin-user=live
autologin-user-timeout=0
autologin-session=openbox
user-session=openbox
EOF`,

		// Create XDG autostart entry for Calamares (works for any WM/DE)
		"mkdir -p /etc/xdg/autostart",
		`cat > /etc/xdg/autostart/calamares-installer.desktop <<'EOF'
[Desktop Entry]
Type=Application
Name=Install System
Comment=Launch Calamares Installer
Exec=sudo calamares
Icon=calamares
Terminal=false
Categories=System;
X-GNOME-Autostart-enabled=true
NoDisplay=false
EOF`,

		// Create Openbox autostart (for Openbox-based minimal ISOs)
		"mkdir -p /etc/xdg/openbox",
		`cat > /etc/xdg/openbox/autostart <<'EOFSCRIPT'
#!/bin/bash
# ALCI-style minimal installer autostart
# Start a panel for basic navigation
tint2 &

# Set wallpaper
feh --bg-fill /usr/share/backgrounds/default.png 2>/dev/null || xsetroot -solid "#2d2d2d" &

# Start notification daemon
dunst &

# Start polkit agent
lxpolkit &

# Start network manager applet
nm-applet &

# Wait for desktop to settle
sleep 2

# Launch Calamares installer
sudo calamares &
EOFSCRIPT`,

		"chmod +x /etc/xdg/openbox/autostart",

		// Create a polkit rule so Calamares can run without password prompt
		"mkdir -p /etc/polkit-1/localauthority/50-local.d",
		`cat > /etc/polkit-1/localauthority/50-local.d/allow-calamares.pkla <<'EOF'
[Allow Calamares]
Identity=unix-user:live
Action=*
ResultAny=yes
ResultInactive=yes
ResultActive=yes
EOF`,

		// Create desktop shortcut for Calamares on the desktop
		"mkdir -p /home/live/Desktop",
		`cat > /home/live/Desktop/install-system.desktop <<'EOF'
[Desktop Entry]
Type=Application
Name=Install System
Comment=Launch the system installer
Exec=sudo calamares
Icon=calamares
Terminal=false
Categories=System;
EOF`,

		"chmod +x /home/live/Desktop/install-system.desktop",
		"chown -R live:live /home/live",

		// Create a simple README on the desktop
		`cat > /home/live/Desktop/README.txt <<'EOF'
Welcome to the Kagami Minimal Installer!

This is a minimal live environment designed for system installation.
The Calamares installer should start automatically.

If it doesn't start, double-click "Install System" on the desktop
or run: sudo calamares

After installation, remove the USB/CD and reboot.
EOF`,

		"chown live:live /home/live/Desktop/README.txt",

		// Enable LightDM
		"systemctl enable lightdm || true",
	}

	for _, script := range scripts {
		if err := b.chrootExec(script); err != nil {
			log.Printf("Warning during minimal installer setup: %v", err)
		}
	}

	fmt.Println("[SUCCESS] Minimal installer environment configured (ALCI-style)")
	return nil
}

// setupCalamares performs comprehensive Calamares configuration
func (b *Builder) setupCalamares() error {
	fmt.Println("[ACTION] Setting up Calamares installer...")

	installerPkgs := []string{"calamares"}
	if b.isDebian() {
		installerPkgs = append(installerPkgs, "calamares-settings-debian")
	} else {
		// For Ubuntu, we use the package as base but will override with git repo if possible
		installerPkgs = append(installerPkgs, "calamares-settings-ubuntu")
	}

	installerList := strings.Join(installerPkgs, " ")
	if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", installerList)); err != nil {
		log.Printf("Warning: Calamares failed to install: %v", err)
	}

	// For Ubuntu, apply settings from the Lubuntu git repository if cloned
	if !b.isDebian() {
		if err := b.applyUbuntuCalamaresSettings(); err != nil {
			log.Printf("Warning: Could not apply Lubuntu git settings: %v", err)
		}
	}

	// Apply custom branding information
	if err := b.applyBranding(); err != nil {
		log.Printf("Warning: Failed to apply branding to Calamares: %v", err)
	}

	// Apply custom Calamares configuration directory (overwrites everything else)
	if err := b.applyCalamaresConfig(); err != nil {
		log.Printf("Warning: Failed to apply custom Calamares configuration: %v", err)
	}

	// Configure minimal installer autostart (ALCI-style)
	return b.configureMinimalInstaller()
}

// applyUbuntuCalamaresSettings uses the Lubuntu settings repository
func (b *Builder) applyUbuntuCalamaresSettings() error {
	cwd, _ := os.Getwd()
	repoPath := filepath.Join(cwd, "Git/calamares-settings-ubuntu/lubuntu")

	if _, err := os.Stat(repoPath); err != nil {
		return fmt.Errorf("ubuntu calamares settings repo not found at %s. Please clone it first", repoPath)
	}

	fmt.Println("[ACTION] Incorporating settings from Lubuntu Calamares repository...")

	destPath := filepath.Join(b.ChrootDir, "etc", "calamares")
	os.MkdirAll(destPath, 0755)

	// Copy modules, branding and settings
	// We use shell to handle the copy recursively
	cmds := []string{
		fmt.Sprintf("cp -rv %s/branding/* %s/branding/", repoPath, destPath),
		fmt.Sprintf("cp -v %s/settings.conf %s/", repoPath, destPath),
		fmt.Sprintf("cp -rv %s/modules/* %s/modules/", repoPath, destPath),
	}

	for _, cmdStr := range cmds {
		cmd := exec.Command("bash", "-c", cmdStr)
		cmd.Run()
	}

	return nil
}

// applyBranding modifies branding.desc in the chroot
func (b *Builder) applyBranding() error {
	branding := b.Config.Installer.Branding
	if branding.ProductName == "" {
		return nil
	}

	fmt.Println("[ACTION] Applying custom branding to Calamares settings...")

	// Possible paths for branding.desc
	paths := []string{
		filepath.Join(b.ChrootDir, "etc", "calamares/branding/lubuntu/branding.desc"),
		filepath.Join(b.ChrootDir, "etc", "calamares/branding/debian/branding.desc"),
		filepath.Join(b.ChrootDir, "etc", "calamares/branding/default/branding.desc"),
	}

	var brandingFile string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			brandingFile = p
			break
		}
	}

	if brandingFile == "" {
		return fmt.Errorf("could not find branding.desc to modify")
	}

	content, err := os.ReadFile(brandingFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "productName:") {
			lines[i] = "    productName:         " + branding.ProductName
		} else if strings.HasPrefix(trimmed, "shortProductName:") {
			lines[i] = "    shortProductName:    " + branding.ShortProductName
		} else if strings.HasPrefix(trimmed, "productUrl:") {
			lines[i] = "    productUrl:          " + branding.ProductUrl
		} else if strings.HasPrefix(trimmed, "supportUrl:") {
			lines[i] = "    supportUrl:          " + branding.SupportUrl
		} else if strings.HasPrefix(trimmed, "version=") {
			lines[i] = "version=" + branding.Version
		}
	}

	return os.WriteFile(brandingFile, []byte(strings.Join(lines, "\n")), 0644)
}

// resolveDebianRelease fetches the real codename for stable, testing, or unstable
func (b *Builder) resolveDebianRelease() {
	if !b.isDebian() {
		return
	}

	alias := strings.ToLower(b.Config.Release)
	if alias != "stable" && alias != "testing" && alias != "unstable" {
		return
	}

	b.DebianAlias = alias
	fmt.Printf("[ACTION] Resolving Debian %s codename...\n", alias)

	mirror := b.Config.Repository.Mirror
	if mirror == "" {
		mirror = "http://deb.debian.org/debian/"
	}

	// Fetch Release file from mirror
	url := fmt.Sprintf("%sdists/%s/Release", mirror, alias)
	cmd := exec.Command("wget", "-qO-", url)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Warning: Could not resolve Debian codename via net (offline?), using alias %s", alias)
		return
	}

	lines := strings.Split(string(output), "\n")
	var label, version, codename string
	for _, line := range lines {
		if strings.HasPrefix(line, "Codename:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				codename = parts[1]
			}
		} else if strings.HasPrefix(line, "Label:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				label = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "Version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				version = strings.TrimSpace(parts[1])
			}
		}
	}

	if codename != "" {
		fmt.Printf("[INFO] Debian %s resolved to codename: %s\n", alias, codename)
		b.Config.Release = codename
		if label != "" && version != "" {
			b.PrettyName = fmt.Sprintf("%s %s", label, version)
		} else if label != "" {
			b.PrettyName = fmt.Sprintf("%s %s", label, strings.Title(alias))
		}
	}
}
