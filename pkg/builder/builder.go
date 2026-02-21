package builder

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"kagami/pkg/config"
	"kagami/pkg/system"
)

type Builder struct {
	Config      *config.Config
	WorkDir     string
	OutputISO   string
	ChrootDir   string
	ImageDir    string
	DebianAlias string
	PrettyName  string
}

func NewBuilder(cfg *config.Config, workDir, outputISO string) *Builder {
	return &Builder{
		Config:    cfg,
		WorkDir:   workDir,
		OutputISO: outputISO,
		ChrootDir: filepath.Join(workDir, "chroot"),
		ImageDir:  filepath.Join(workDir, "image"),
	}
}

func (b *Builder) isDebian() bool {
	return b.Config.Distro == "debian"
}

func (b *Builder) liveDir() string {
	if b.isDebian() {
		return "live"
	}
	return "casper"
}

func (b *Builder) bootParam() string {
	if b.isDebian() {
		return "boot=live"
	}
	return "boot=casper"
}

func (b *Builder) getDistName() string {
	if b.PrettyName != "" {
		return b.PrettyName
	}
	if b.isDebian() {
		if b.DebianAlias != "" {
			return "Debian " + cases.Title(language.English).String(b.DebianAlias)
		}
		return "Debian (" + b.Config.Release + ")"
	}
	switch b.Config.Release {
	case "focal", "jammy", "noble", "resolute":
		return "Ubuntu LTS"
	case "devel":
		return "Ubuntu Rolling"
	default:
		return "Ubuntu"
	}
}

func (b *Builder) Build() error {
	b.resolveDebianRelease()

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Verifying prerequisites", b.checkPrerequisites},
		{"Initialising directory structure", b.createDirectories},
		{"Bootstrapping base system", b.bootstrapSystem},
		{"Mounting filesystems", b.mountFilesystems},
		{"Configuring base system", b.configureSystem},
		{"Applying snapd suppression", b.blockSnapd},
		{"Installing package manifest", b.installPackages},
		{"Installing desktop environment", b.installDesktop},
		{"Configuring Flatpak support", b.setupFlatpak},
		{"Configuring bootloader", b.configureBootloader},
		{"Cleaning chroot environment", b.cleanupChroot},
		{"Creating compressed filesystem", b.createFilesystem},
		{"Synthesising ISO image", b.createISO},
		{"Finalising build", b.cleanup},
	}

	for i, step := range steps {
		fmt.Printf("[%d/%d] %s...\n", i+1, len(steps), step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s: %v", step.name, err)
		}
	}

	return nil
}

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
			return fmt.Errorf("required tool '%s' not found; install with: sudo apt-get install %s", tool, tool)
		}
	}

	if os.Geteuid() != 0 {
		return fmt.Errorf("elevated privileges are required; re-execute with sudo")
	}

	if system.IsContainer() {
		fmt.Println("[INFO] Container environment detected (Docker/Podman/Distrobox).")
		fmt.Println("       Ensure the container has SYS_ADMIN capability for bind mounts.")
	}

	return nil
}

func (b *Builder) createDirectories() error {
	dirs := []string{
		b.WorkDir,
		b.ChrootDir,
		b.ImageDir,
		filepath.Join(b.ImageDir, b.liveDir()),
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

func (b *Builder) bootstrapSystem() error {
	if _, err := os.Stat(filepath.Join(b.ChrootDir, "etc")); err == nil {
		log.Println("Chroot already exists; skipping bootstrap phase")
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

	bootstrapRelease := b.Config.Release
	if bootstrapRelease == "devel" {
		bootstrapRelease = "noble"
		fmt.Println("[INFO] Bootstrapping with 'noble' as base for development target")
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
			return fmt.Errorf("debootstrap failed: %v\n[TIP] Container environments require '--privileged' or CAP_MKNOD for device node creation", err)
		}
		return err
	}
	return nil
}

func (b *Builder) mountFilesystems() error {
	mounts := []struct {
		source string
		target string
		flags  string
	}{
		{"/dev", filepath.Join(b.ChrootDir, "dev"), "bind"},
		{"/run", filepath.Join(b.ChrootDir, "run"), "bind"},
	}

	for _, m := range mounts {
		if isMounted(m.target) {
			continue
		}
		if err := os.MkdirAll(m.target, 0755); err != nil {
			return fmt.Errorf("failed to create mount target %s: %v", m.target, err)
		}

		cmd := exec.Command("mount", "--bind", m.source, m.target)
		if err := cmd.Run(); err != nil {
			errMsg := fmt.Errorf("failed to mount %s: %v", m.target, err)
			if system.IsContainer() {
				return fmt.Errorf("%v\n[TIP] Container environments require '--privileged' or CAP_SYS_ADMIN", errMsg)
			}
			return errMsg
		}
	}

	return nil
}

func (b *Builder) configureSystem() error {
	initScripts := []string{
		fmt.Sprintf("echo '%s' > /etc/hostname", b.Config.System.Hostname),
		b.generateSourcesList(),
		"mount none -t proc /proc 2>/dev/null || true",
		"mount none -t sysfs /sys 2>/dev/null || true",
		"mount none -t devpts /dev/pts 2>/dev/null || true",
		"export HOME=/root",
		"export LC_ALL=C",
	}

	for _, script := range initScripts {
		if err := b.chrootExec(script); err != nil {
			return err
		}
	}

	if err := b.configureAdditionalRepos(); err != nil {
		log.Printf("[WARNING] Additional repository configuration failed: %v", err)
	}

	basePackages := "systemd-sysv"
	if !b.isDebian() {
		basePackages = "libterm-readline-gnu-perl systemd-sysv"
	}

	postScripts := []string{
		"apt-get update",
		fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", basePackages),
		"dbus-uuidgen > /etc/machine-id",
		"ln -fs /etc/machine-id /var/lib/dbus/machine-id",
		"dpkg-divert --local --rename --add /sbin/initctl",
		"ln -s /bin/true /sbin/initctl",
	}

	for _, script := range postScripts {
		if err := b.chrootExec(script); err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) installPackages() error {
	if err := b.chrootExec("DEBIAN_FRONTEND=noninteractive apt-get -y dist-upgrade"); err != nil {
		return err
	}

	essentialPkgs := strings.Join(b.Config.Packages.Essential, " ")
	if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", essentialPkgs)); err != nil {
		return err
	}

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

	if len(b.Config.Packages.Additional) > 0 {
		additionalPkgs := strings.Join(b.Config.Packages.Additional, " ")
		if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", additionalPkgs)); err != nil {
			log.Printf("[WARNING] Some additional packages failed to install: %v", err)
		}
	}

	return nil
}

func (b *Builder) blockSnapd() error {
	if !b.Config.Security.BlockSnapdForever && !b.Config.System.BlockSnapd {
		return nil
	}

	fmt.Println("[INFO] Implementing multi-layer snapd suppression...")

	scripts := []string{
		"apt-get purge -y snapd snap-confine ubuntu-core-launcher snapd-xdg-open || true",
		"apt-get autoremove -y || true",
		"rm -rf /var/cache/snapd /var/lib/snapd /var/snap /snap",
		`cat > /etc/apt/preferences.d/nosnapd.pref <<'EOF'
Explanation: Snapd package installation is permanently prohibited on this system.
Package: snapd
Pin: release *
Pin-Priority: -1

Package: snapd:*
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
		"mkdir -p /etc/systemd/system/snapd.service.d",
		`cat > /etc/systemd/system/snapd.service.d/override.conf <<'EOF'
[Unit]
ConditionPathExists=!/etc/snapd-blocked

[Service]
ExecStart=
ExecStart=/bin/false
EOF`,
		"touch /etc/snapd-blocked",
		`echo "Snapd is permanently suppressed on this system." > /etc/snapd-blocked`,
		"mkdir -p /etc/systemd/system/snapd.socket.d",
		`cat > /etc/systemd/system/snapd.socket.d/override.conf <<'EOF'
[Unit]
ConditionPathExists=!/etc/snapd-blocked

[Socket]
ListenStream=
EOF`,
		"mkdir -p /etc/apt/apt.conf.d",
		`cat > /etc/apt/apt.conf.d/99-block-snapd <<'EOF'
DPkg::Pre-Install-Pkgs {
  "/usr/local/bin/block-snapd-hook";
};
EOF`,
		`cat > /usr/local/bin/block-snapd-hook <<'EOFSCRIPT'
#!/bin/sh
while read pkg; do
    case "$pkg" in
        *snapd*)
            echo "Installation of snapd is permanently prohibited on this system." >&2
            exit 1
            ;;
    esac
done
EOFSCRIPT`,
		"chmod +x /usr/local/bin/block-snapd-hook",
		"mkdir -p /etc/update-motd.d",
		`cat > /etc/update-motd.d/99-snapd-blocked <<'EOF'
#!/bin/sh
echo ""
echo "-----------------------------------------------------------"
echo "  NOTICE: Snapd is permanently suppressed on this system.  "
echo "  Snap package installation is not permitted.              "
echo "-----------------------------------------------------------"
echo ""
EOF`,
		"chmod +x /etc/update-motd.d/99-snapd-blocked",
		"dpkg-divert --local --rename --add /usr/bin/snap || true",
		"ln -sf /bin/false /usr/bin/snap || true",
		"rm -rf /snap /var/snap /var/lib/snapd ~/snap || true",
		`cat >> /etc/profile.d/block-snapd.sh <<'EOF'
export SNAPD_BLOCKED=1
snap() {
    echo "Snapd is permanently suppressed on this system." >&2
    return 1
}
EOF`,
		"chmod +x /etc/profile.d/block-snapd.sh",
	}

	for _, script := range scripts {
		if err := b.chrootExec(script); err != nil {
			log.Printf("[WARNING] Snapd suppression step encountered an error: %v", err)
		}
	}

	fmt.Println("[OK] Snapd suppression applied across all protection layers")
	return nil
}

func (b *Builder) installDesktop() error {
	if b.Config.Packages.Desktop == "none" {
		log.Println("Desktop mode is 'none'; packages must be specified in the additional list")

		if b.Config.Installer.Type == "ubiquity" {
			installerPkgs := []string{
				"ubiquity",
				"ubiquity-casper",
				"ubiquity-frontend-gtk",
				"ubiquity-ubuntu-artwork",
			}

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
				log.Printf("[WARNING] Some installer packages failed to install: %v", err)
			}
		}

		if b.Config.Installer.Type == "calamares" {
			if err := b.setupCalamares(); err != nil {
				log.Printf("[WARNING] Calamares configuration failed: %v", err)
			}
		}

		if len(b.Config.Packages.RemoveList) > 0 {
			removeList := strings.Join(b.Config.Packages.RemoveList, " ")
			b.chrootExec(fmt.Sprintf("apt-get purge -y %s || true", removeList))
		}

		b.chrootExec("apt-get autoremove -y")
		return nil
	}

	ubuntuDesktopPackages := map[string][]string{
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
		"kde":  {"kde-plasma-desktop"},
		"xfce": {"xfce4", "xfce4-goodies"},
		"lxde": {"lxde"},
		"lxqt": {"lxqt"},
		"mate": {"mate-desktop-environment"},
	}

	debianDesktopPackages := map[string][]string{
		"gnome": {"task-gnome-desktop"},
		"kde":   {"task-kde-desktop"},
		"xfce":  {"task-xfce-desktop"},
		"lxde":  {"task-lxde-desktop"},
		"lxqt":  {"task-lxqt-desktop"},
		"mate":  {"task-mate-desktop"},
	}

	var desktopPackages map[string][]string
	if b.isDebian() {
		desktopPackages = debianDesktopPackages
	} else {
		desktopPackages = ubuntuDesktopPackages
	}

	pkgs, ok := desktopPackages[b.Config.Packages.Desktop]
	if !ok {
		return fmt.Errorf("unsupported desktop environment identifier: %s", b.Config.Packages.Desktop)
	}

	pkgList := strings.Join(pkgs, " ")
	installCmd := "DEBIAN_FRONTEND=noninteractive apt-get install -y"
	if b.isDebian() {
		installCmd += " --no-install-recommends"
	}

	if err := b.chrootExec(fmt.Sprintf("%s %s", installCmd, pkgList)); err != nil {
		return err
	}

	if b.Config.Installer.Type == "ubiquity" && !b.isDebian() {
		installerPkgs := []string{
			"ubiquity",
			"ubiquity-casper",
			"ubiquity-frontend-gtk",
			"ubiquity-ubuntu-artwork",
		}

		installerList := strings.Join(installerPkgs, " ")
		if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends %s", installerList)); err != nil {
			log.Printf("[WARNING] Some installer packages failed to install: %v", err)
		}
	}

	if b.Config.Installer.Type == "calamares" {
		if err := b.setupCalamares(); err != nil {
			log.Printf("[WARNING] Calamares configuration failed: %v", err)
		}
	}

	if len(b.Config.Packages.RemoveList) > 0 {
		removeList := strings.Join(b.Config.Packages.RemoveList, " ")
		b.chrootExec(fmt.Sprintf("apt-get purge -y %s || true", removeList))
	}

	b.chrootExec("apt-get autoremove -y")

	if b.Config.Packages.Desktop == "gnome" && !b.isDebian() {
		b.refineVanillaGNOME()
	}

	return nil
}

func (b *Builder) refineVanillaGNOME() error {
	fmt.Println("[INFO] Refining vanilla GNOME configuration for Ubuntu...")

	scripts := []string{
		"apt-get purge -y ubuntu-session yaru-theme-gnome-shell yaru-theme-gtk yaru-theme-icon yaru-theme-sound || true",
		"update-alternatives --set gdm3-theme.desktop /usr/share/gnome-shell/theme/gnome-shell.css || true",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y qgnomeplatform-qt5 qgnomeplatform-qt6 || true",
	}

	for _, script := range scripts {
		if err := b.chrootExec(script); err != nil {
			log.Printf("[WARNING] GNOME refinement step failed: %v", err)
		}
	}
	return nil
}

func (b *Builder) setupFlatpak() error {
	if !b.Config.Packages.EnableFlatpak {
		return nil
	}
	return b.installFlatpak()
}

func (b *Builder) installFlatpak() error {
	fmt.Println("[INFO] Installing Flatpak and registering Flathub repository...")

	pkgs := []string{"flatpak"}

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

	return b.chrootExec("flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo")
}

func (b *Builder) configureBootloader() error {
	suffix := "generic"
	if b.isDebian() {
		suffix = "amd64"
	}

	if b.Config.Packages.Kernel != "" {
		switch {
		case strings.Contains(b.Config.Packages.Kernel, "lowlatency"):
			suffix = "lowlatency"
		case strings.Contains(b.Config.Packages.Kernel, "oem"):
			suffix = "oem"
		case strings.Contains(b.Config.Packages.Kernel, "amd64"):
			suffix = "amd64"
		case strings.Contains(b.Config.Packages.Kernel, "rt"):
			suffix = "rt"
		}
	}

	kernelPattern := filepath.Join(b.ChrootDir, "boot", "vmlinuz-*"+suffix+"*")
	initrdPattern := filepath.Join(b.ChrootDir, "boot", "initrd.img-*"+suffix+"*")

	kernels, _ := filepath.Glob(kernelPattern)
	if len(kernels) == 0 {
		kernelPattern = filepath.Join(b.ChrootDir, "boot", "vmlinuz-*")
		initrdPattern = filepath.Join(b.ChrootDir, "boot", "initrd.img-*")
		kernels, _ = filepath.Glob(kernelPattern)
	}
	initrds, _ := filepath.Glob(initrdPattern)

	liveDestDir := filepath.Join(b.ImageDir, b.liveDir())

	if len(kernels) > 0 {
		exec.Command("cp", kernels[0], filepath.Join(liveDestDir, "vmlinuz")).Run()
	}
	if len(initrds) > 0 {
		exec.Command("cp", initrds[0], filepath.Join(liveDestDir, "initrd")).Run()
	}

	memtestURL := "https://memtest.org/download/v7.00/mt86plus_7.00.binaries.zip"
	memtestZip := filepath.Join(b.ImageDir, "install", "memtest86.zip")

	exec.Command("wget", "--progress=dot", memtestURL, "-O", memtestZip).Run()
	exec.Command("unzip", "-p", memtestZip, "memtest64.bin").Output()
	exec.Command("unzip", "-p", memtestZip, "memtest64.efi").Output()
	exec.Command("rm", "-f", memtestZip).Run()

	markerFile := filepath.Join(b.ImageDir, "kagami-live")
	os.WriteFile(markerFile, []byte(""), 0644)

	grubCfg := filepath.Join(b.ImageDir, "isolinux", "grub.cfg")
	grubContent := b.generateGrubConfig()
	if err := os.WriteFile(grubCfg, []byte(grubContent), 0644); err != nil {
		return err
	}

	return nil
}

func (b *Builder) generateGrubConfig() string {
	distName := b.getDistName()
	liveDir := b.liveDir()
	bootParam := b.bootParam()

	var persistParam string
	if b.isDebian() {
		persistParam = "nopersistence"
	} else {
		persistParam = "nopersistent"
	}

	var installEntry string
	switch b.Config.Installer.Type {
	case "ubiquity":
		installEntry = b.generateUbiquityGrubEntry()
	case "calamares":
		installEntry = b.generateCalamaresGrubEntry()
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
   linux /%s/vmlinuz %s %s toram quiet splash ---
   initrd /%s/initrd
}
%s

menuentry "Check disc for defects" {
   linux /%s/vmlinuz %s integrity-check quiet splash ---
   initrd /%s/initrd
}

if [ "$grub_platform" = "efi" ]; then
menuentry "UEFI Firmware Settings" {
   fwsetup
}

menuentry "Test memory (Memtest86+ UEFI)" {
   linux /install/memtest86+.efi
}
else
menuentry "Test memory (Memtest86+ BIOS)" {
   linux16 /install/memtest86+.bin
}
fi

`, distName, liveDir, bootParam, persistParam, liveDir, installEntry,
		liveDir, bootParam, liveDir)
}

func (b *Builder) generateUbiquityGrubEntry() string {
	distName := b.getDistName()
	liveDir := b.liveDir()
	bootParam := b.bootParam()
	return fmt.Sprintf(`
menuentry "Install %s (Ubiquity)" {
   linux /%s/vmlinuz %s only-ubiquity quiet splash ---
   initrd /%s/initrd
}`, distName, liveDir, bootParam, liveDir)
}

func (b *Builder) generateCalamaresGrubEntry() string {
	distName := b.getDistName()
	liveDir := b.liveDir()
	bootParam := b.bootParam()
	return fmt.Sprintf(`
menuentry "Install %s (Calamares)" {
   linux /%s/vmlinuz %s quiet splash ---
   initrd /%s/initrd
}`, distName, liveDir, bootParam, liveDir)
}

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

func (b *Builder) applyCalamaresConfig() error {
	if b.Config.Installer.CalamaresConfig == "" {
		return nil
	}

	fmt.Printf("[INFO] Applying custom Calamares configuration from: %s\n", b.Config.Installer.CalamaresConfig)

	srcPath := b.Config.Installer.CalamaresConfig
	if !filepath.IsAbs(srcPath) {
		cwd, _ := os.Getwd()
		srcPath = filepath.Join(cwd, srcPath)
	}

	destPath := filepath.Join(b.ChrootDir, "etc", "calamares")
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create calamares configuration directory: %v", err)
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf("cp -rv %s/* %s/", srcPath, destPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (b *Builder) configureMinimalInstaller() error {
	fmt.Println("[INFO] Configuring minimal live installer environment...")

	wm := b.Config.Packages.WM
	if wm == "" {
		wm = "openbox" // fallback
	}

	liveUser := "live"
	if !b.isDebian() {
		liveUser = "ubuntu"
	}

	sessionName := wm
	if wm == "xfce4-minimal" {
		sessionName = "xfce"
	}

	scripts := []string{
		fmt.Sprintf("useradd -m -G sudo -s /bin/bash %s || true", liveUser),
		fmt.Sprintf("echo '%s:%s' | chpasswd", liveUser, liveUser),
		fmt.Sprintf("echo '%s ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/live", liveUser),
		"mkdir -p /etc/lightdm/lightdm.conf.d",
		fmt.Sprintf(`cat > /etc/lightdm/lightdm.conf.d/50-autologin.conf <<'EOF'
[Seat:*]
autologin-user=%s
autologin-user-timeout=0
autologin-session=%s
user-session=%s
EOF`, liveUser, sessionName, sessionName),
	}

	// WM specific autostart
	switch wm {
	case "openbox":
		scripts = append(scripts,
			"mkdir -p /etc/xdg/openbox",
			`cat > /etc/xdg/openbox/autostart <<'EOFSCRIPT'
#!/bin/sh
# Ensure calamares-launcher is in path
export PATH=$PATH:/usr/local/bin
tint2 &
feh --bg-fill /usr/share/backgrounds/default.png 2>/dev/null || xsetroot -solid "#2d2d2d" &
dunst &
lxpolkit &
nm-applet &
sleep 2
calamares-launcher &
EOFSCRIPT`,
			"chmod +x /etc/xdg/openbox/autostart",
		)
	case "dwm":
		scripts = append(scripts,
			`cat > /usr/local/bin/dwm-session <<'EOFSCRIPT'
#!/bin/sh
export PATH=$PATH:/usr/local/bin
feh --bg-fill /usr/share/backgrounds/default.png 2>/dev/null || xsetroot -solid "#2d2d2d" &
dunst &
lxpolkit &
nm-applet &
(sleep 2 && calamares-launcher) &
exec dwm
EOFSCRIPT`,
			"chmod +x /usr/local/bin/dwm-session",
			`cat > /usr/share/xsessions/dwm.desktop <<'EOF'
[Desktop Entry]
Name=dwm
Comment=ALCI style dynamic window manager
Exec=/usr/local/bin/dwm-session
Type=Application
EOF`,
		)
	case "xfce4-minimal":
		scripts = append(scripts,
			"mkdir -p /etc/xdg/autostart",
			`cat > /etc/xdg/autostart/calamares-autostart.desktop <<'EOF'
[Desktop Entry]
Type=Application
Name=Calamares Launcher
Exec=calamares-launcher
Icon=install-system
Terminal=false
X-GNOME-Autostart-enabled=true
EOF`,
		)
	}

	scripts = append(scripts,
		"chmod +x /usr/bin/calamares-launcher /usr/bin/add-calamares-desktop-icon || true",
		"mkdir -p /etc/polkit-1/localauthority/50-local.d",
		fmt.Sprintf(`cat > /etc/polkit-1/localauthority/50-local.d/allow-calamares.pkla <<'EOF'
[Allow Calamares]
Identity=unix-user:%s
Action=*
ResultAny=yes
ResultInactive=yes
ResultActive=yes
EOF`, liveUser),
		fmt.Sprintf("chown -R %s:%s /home/%s", liveUser, liveUser, liveUser),
		"systemctl enable lightdm || true",
	)

	for _, script := range scripts {
		if err := b.chrootExec(script); err != nil {
			log.Printf("[WARNING] Minimal installer configuration step failed: %v", err)
		}
	}

	fmt.Println("[OK] Minimal live installer environment configured")
	return nil
}

func (b *Builder) setupCalamares() error {
	fmt.Println("[INFO] Installing and configuring Calamares...")

	installerPkgs := []string{"calamares"}
	installerList := strings.Join(installerPkgs, " ")
	if err := b.chrootExec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", installerList)); err != nil {
		log.Printf("[WARNING] Calamares installation failed: %v", err)
	}

	if err := b.applyLocalCalamaresSettings(); err != nil {
		log.Printf("[WARNING] Local Calamares settings application failed: %v", err)
	}

	if err := b.applyBranding(); err != nil {
		log.Printf("[WARNING] Calamares branding application failed: %v", err)
	}

	if err := b.applyCalamaresConfig(); err != nil {
		log.Printf("[WARNING] Custom Calamares configuration failed: %v", err)
	}

	return b.configureMinimalInstaller()
}

func (b *Builder) applyLocalCalamaresSettings() error {
	cwd, _ := os.Getwd()
	localDataPath := filepath.Join(cwd, "calamares/data")

	if _, err := os.Stat(localDataPath); err != nil {
		return fmt.Errorf("local Calamares settings not found at %s", localDataPath)
	}

	fmt.Println("[INFO] Applying generic Calamares settings from project...")

	cmd := exec.Command("cp", "-rv", localDataPath+"/.", b.ChrootDir+"/")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Update settings.conf branding based on target OS
	brandingName := "ubuntu"
	if b.isDebian() {
		brandingName = "debian"
	}

	settingsPath := filepath.Join(b.ChrootDir, "etc", "calamares", "settings.conf")
	if content, err := os.ReadFile(settingsPath); err == nil {
		updated := strings.Replace(string(content), "branding: kagami", "branding: "+brandingName, 1)
		os.WriteFile(settingsPath, []byte(updated), 0644)
	}

	return nil
}

func (b *Builder) applyBranding() error {
	branding := b.Config.Installer.Branding
	if branding.ProductName == "" {
		return nil
	}

	fmt.Println("[INFO] Applying custom branding to Calamares configuration...")

	paths := []string{
		filepath.Join(b.ChrootDir, "etc", "calamares/branding/ubuntu/branding.desc"),
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
		return fmt.Errorf("branding.desc not found in expected locations")
	}

	content, err := os.ReadFile(brandingFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "productName:"):
			lines[i] = "    productName:         " + branding.ProductName
		case strings.HasPrefix(trimmed, "shortProductName:"):
			lines[i] = "    shortProductName:    " + branding.ShortProductName
		case strings.HasPrefix(trimmed, "productUrl:"):
			lines[i] = "    productUrl:          " + branding.ProductUrl
		case strings.HasPrefix(trimmed, "supportUrl:"):
			lines[i] = "    supportUrl:          " + branding.SupportUrl
		case strings.HasPrefix(trimmed, "version="):
			lines[i] = "version=" + branding.Version
		}
	}

	return os.WriteFile(brandingFile, []byte(strings.Join(lines, "\n")), 0644)
}

func (b *Builder) resolveDebianRelease() {
	if !b.isDebian() {
		return
	}

	alias := strings.ToLower(b.Config.Release)
	if alias != "stable" && alias != "testing" && alias != "unstable" {
		return
	}

	b.DebianAlias = alias
	fmt.Printf("[INFO] Resolving Debian '%s' alias to current codename...\n", alias)

	mirror := b.Config.Repository.Mirror
	if mirror == "" {
		mirror = "http://deb.debian.org/debian/"
	}

	url := fmt.Sprintf("%sdists/%s/Release", mirror, alias)
	cmd := exec.Command("wget", "-qO-", url)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[WARNING] Could not resolve Debian codename via network; proceeding with alias '%s'", alias)
		return
	}

	var label, version, codename string
	for _, line := range strings.Split(string(output), "\n") {
		switch {
		case strings.HasPrefix(line, "Codename:"):
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				codename = parts[1]
			}
		case strings.HasPrefix(line, "Label:"):
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				label = strings.TrimSpace(parts[1])
			}
		case strings.HasPrefix(line, "Version:"):
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				version = strings.TrimSpace(parts[1])
			}
		}
	}

	if codename != "" {
		fmt.Printf("[INFO] Debian '%s' resolved to codename: %s\n", alias, codename)
		b.Config.Release = codename
		if label != "" && version != "" {
			b.PrettyName = fmt.Sprintf("%s %s", label, version)
		} else if label != "" {
			b.PrettyName = fmt.Sprintf("%s %s", label, cases.Title(language.English).String(alias))
		}
	}
}
