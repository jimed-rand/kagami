package builder

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kagami/pkg/system"
)

// createFilesystem creates the squashfs filesystem and manifest
func (b *Builder) createFilesystem() error {
	// Create manifest
	manifestPath := filepath.Join(b.ImageDir, "casper", "filesystem.manifest")
	manifestDesktopPath := filepath.Join(b.ImageDir, "casper", "filesystem.manifest-desktop")

	dpkgCmd := "dpkg-query -W --showformat='${Package} ${Version}\\n'"
	output, err := b.chrootExecOutput(dpkgCmd)
	if err != nil {
		return err
	}

	if err := os.WriteFile(manifestPath, []byte(output), 0644); err != nil {
		return err
	}

	// Create desktop manifest (remove installer packages)
	manifestContent := output
	removePackages := []string{"ubiquity", "calamares", "casper", "discover", "laptop-detect", "os-prober"}
	for _, pkg := range removePackages {
		lines := strings.Split(manifestContent, "\n")
		var filteredLines []string
		for _, line := range lines {
			if !strings.Contains(line, pkg) {
				filteredLines = append(filteredLines, line)
			}
		}
		manifestContent = strings.Join(filteredLines, "\n")
	}

	if err := os.WriteFile(manifestDesktopPath, []byte(manifestContent), 0644); err != nil {
		return err
	}

	// Create diskdefines
	diskdefinesPath := filepath.Join(b.ImageDir, "README.diskdefines")
	distName := b.getDistName()
	diskdefines := fmt.Sprintf(`#define DISKNAME  %s %s
#define TYPE  binary
#define TYPEbinary  1
#define ARCH  %s
#define ARCHamd64  1
#define DISKNUM  1
#define DISKNUM1  1
#define TOTALNUM  0
#define TOTALNUM0  1
`, distName, b.Config.Release, b.Config.System.Architecture)

	if err := os.WriteFile(diskdefinesPath, []byte(diskdefines), 0644); err != nil {
		return err
	}

	// Move image directory out of chroot
	imageInChroot := filepath.Join(b.ChrootDir, "image")
	if _, err := os.Stat(imageInChroot); err == nil {
		if err := os.Rename(imageInChroot, b.ImageDir); err != nil {
			log.Printf("Warning: Could not move image directory: %v", err)
		}
	}

	// Create squashfs
	fmt.Println("[STEP] Creating compressed filesystem (this may take several minutes)...")
	squashfsPath := filepath.Join(b.ImageDir, "casper", "filesystem.squashfs")

	args := []string{
		b.ChrootDir,
		squashfsPath,
		"-noappend",
		"-no-duplicates",
		"-no-recovery",
		"-wildcards",
		"-comp", "xz",
		"-b", "1M",
		"-Xdict-size", "100%",
		"-e", "var/cache/apt/archives/*",
		"-e", "root/*",
		"-e", "root/.*",
		"-e", "tmp/*",
		"-e", "tmp/.*",
		"-e", "swapfile",
		"-e", "image",
	}

	// In container environments or certain filesystems, POSIX ACLs/xattrs can cause errors
	// (e.g., "Unrecognised xattr prefix system.posix_acl_access").
	// Using -no-xattrs is safer and more compatible for live ISOs.
	if system.IsContainer() {
		args = append(args, "-no-xattrs")
	}

	cmd := exec.Command("mksquashfs", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	// Write filesystem.size
	sizePath := filepath.Join(b.ImageDir, "casper", "filesystem.size")
	duCmd := exec.Command("du", "-sx", "--block-size=1", b.ChrootDir)
	outputBytes, err := duCmd.Output()
	if err != nil {
		return err
	}

	size := strings.Fields(string(outputBytes))[0]
	if err := os.WriteFile(sizePath, []byte(size), 0644); err != nil {
		return err
	}

	return nil
}

// createISO generates the final ISO image
func (b *Builder) createISO() error {
	fmt.Println("[STEP] Preparing boot loaders...")

	isolinuxDir := filepath.Join(b.ImageDir, "isolinux")
	if err := os.MkdirAll(isolinuxDir, 0755); err != nil {
		return fmt.Errorf("failed to create isolinux directory: %v", err)
	}

	// Dynamic detection of EFI loaders to support diverse environments (Host/Container/Chroot)
	copyFile := func(srcPaths []string, dstName string) bool {
		dst := filepath.Join(isolinuxDir, dstName)
		for _, path := range srcPaths {
			// Check absolute path
			if _, err := os.Stat(path); err == nil {
				if exec.Command("cp", path, dst).Run() == nil {
					return true
				}
			}
			// Check relative to chroot
			chrootPath := filepath.Join(b.ChrootDir, path)
			if _, err := os.Stat(chrootPath); err == nil {
				if exec.Command("cp", chrootPath, dst).Run() == nil {
					return true
				}
			}
		}
		return false
	}

	fmt.Println("[STEP] Copying EFI loaders (Shim/GRUB)...")

	// 1. Copy Shim (bootx64.efi)
	shimPaths := []string{
		"/usr/lib/shim/shimx64.efi.signed",
		"/usr/lib/shim/shimx64.efi.signed.previous",
		"/usr/lib/shim/shimx64.efi",
		"/boot/efi/EFI/ubuntu/shimx64.efi",
		"/usr/lib/shim/shim.efi",
	}
	if !copyFile(shimPaths, "bootx64.efi") {
		log.Printf("[WARNING] Could not find shimx64.efi, UEFI Secure Boot might not work")
	}

	// 2. Copy MokManager (mmx64.efi)
	mmPaths := []string{
		"/usr/lib/shim/mmx64.efi",
		"/usr/lib/shim/mmx64.efi.signed",
		"/boot/efi/EFI/ubuntu/mmx64.efi",
	}
	copyFile(mmPaths, "mmx64.efi")

	// 3. Copy GRUB EFI (grubx64.efi)
	grubEfiPaths := []string{
		"/usr/lib/grub/x86_64-efi-signed/grubx64.efi.signed",
		"/usr/lib/grub/x86_64-efi/monolithic/grubx64.efi",
		"/boot/efi/EFI/ubuntu/grubx64.efi",
		"/usr/lib/grub/x86_64-efi/grub.efi",
	}
	if !copyFile(grubEfiPaths, "grubx64.efi") {
		log.Printf("[WARNING] grubx64.efi not found in common locations, attempting fallback...")
		// Fallback to searching anywhere in chroot
		findCmd := exec.Command("find", b.ChrootDir, "-name", "grubx64.efi", "-o", "-name", "grubx64.efi.signed")
		output, _ := findCmd.Output()
		foundPaths := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(foundPaths) > 0 && foundPaths[0] != "" {
			exec.Command("cp", foundPaths[0], filepath.Join(isolinuxDir, "grubx64.efi")).Run()
		} else {
			return fmt.Errorf("mandatory file grubx64.efi not found")
		}
	}

	fmt.Println("[STEP] Creating EFI boot image...")

	// Create EFI boot image
	efibootImg := filepath.Join(isolinuxDir, "efiboot.img")
	grubCfg := filepath.Join(isolinuxDir, "grub.cfg")

	commands := [][]string{
		{"dd", "if=/dev/zero", "of=" + efibootImg, "bs=1M", "count=10"},
		{"mkfs.vfat", "-F", "16", efibootImg},
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return err
		}
	}

	// Use mtools to copy files to FAT image
	mmdCmd := exec.Command("mmd", "-i", efibootImg, "efi", "efi/ubuntu", "efi/debian", "efi/boot")
	mmdCmd.Run()

	mcopyCommands := [][]string{
		{"mcopy", "-i", efibootImg, filepath.Join(isolinuxDir, "bootx64.efi"), "::efi/boot/bootx64.efi"},
		{"mcopy", "-i", efibootImg, filepath.Join(isolinuxDir, "mmx64.efi"), "::efi/boot/mmx64.efi"},
		{"mcopy", "-i", efibootImg, filepath.Join(isolinuxDir, "grubx64.efi"), "::efi/boot/grubx64.efi"},
		{"mcopy", "-i", efibootImg, grubCfg, "::efi/boot/grub.cfg"},
		{"mcopy", "-i", efibootImg, grubCfg, "::efi/ubuntu/grub.cfg"},
		{"mcopy", "-i", efibootImg, grubCfg, "::efi/debian/grub.cfg"},
	}

	for _, cmd := range mcopyCommands {
		exec.Command(cmd[0], cmd[1:]...).Run()
	}

	fmt.Println("[STEP] Creating GRUB BIOS image...")

	// Create GRUB BIOS image
	coreImg := filepath.Join(isolinuxDir, "core.img")
	biosImg := filepath.Join(isolinuxDir, "bios.img")

	grubMkCmd := exec.Command("grub-mkstandalone",
		"--format=i386-pc",
		"--output="+coreImg,
		"--install-modules=linux16 linux normal iso9660 biosdisk memdisk search tar ls",
		"--modules=linux16 linux normal iso9660 biosdisk search",
		"--locales=",
		"--fonts=",
		"boot/grub/grub.cfg="+grubCfg,
	)
	if err := grubMkCmd.Run(); err != nil {
		return err
	}

	// Combine bootable GRUB cdboot.img
	cdbootImg := "/usr/lib/grub/i386-pc/cdboot.img"
	catCmd := exec.Command("cat", cdbootImg, coreImg)
	biosFile, err := os.Create(biosImg)
	if err != nil {
		return err
	}
	catCmd.Stdout = biosFile
	if err := catCmd.Run(); err != nil {
		biosFile.Close()
		return err
	}
	biosFile.Close()

	fmt.Println("[STEP] Generating MD5 checksums...")

	// Generate md5sum.txt
	md5Path := filepath.Join(b.ImageDir, "md5sum.txt")
	findCmd := exec.Command("find", ".", "-type", "f", "-print0")
	findCmd.Dir = b.ImageDir
	xargsCmd := exec.Command("xargs", "-0", "md5sum")

	findOutput, _ := findCmd.StdoutPipe()
	xargsCmd.Stdin = findOutput

	md5File, err := os.Create(md5Path)
	if err != nil {
		return err
	}
	defer md5File.Close()
	xargsCmd.Stdout = md5File

	findCmd.Start()
	xargsCmd.Run()
	findCmd.Wait()

	// Filter out isolinux from md5sum
	content, _ := os.ReadFile(md5Path)
	lines := strings.Split(string(content), "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.Contains(line, "isolinux") && line != "" {
			filtered = append(filtered, line)
		}
	}
	os.WriteFile(md5Path, []byte(strings.Join(filtered, "\n")), 0644)

	fmt.Println("[STEP] Building final ISO image...")

	// Create ISO
	hybridImg := "/usr/lib/grub/i386-pc/boot_hybrid.img"

	// Simplified Volume ID for better compatibility
	volid := fmt.Sprintf("KAGAMI_%s_%s", strings.ToUpper(b.Config.Release), strings.ToUpper(b.Config.System.Architecture))

	// Start building xorriso arguments
	xorrisoArgs := []string{
		"-as", "mkisofs",
		"-iso-level", "3",
		"-full-iso9660-filenames",
		"-J", "-J", "-joliet-long",
		"-volid", volid,
		"-output", b.OutputISO,
		"-eltorito-boot", "isolinux/bios.img",
		"-no-emul-boot",
		"-boot-load-size", "4",
		"-boot-info-table",
		"--eltorito-catalog", "boot.catalog",
		"--grub2-boot-info",
	}

	// Add BIOS hybrid MBR if available
	if _, err := os.Stat(hybridImg); err == nil {
		xorrisoArgs = append(xorrisoArgs, "--grub2-mbr", hybridImg)
	} else {
		log.Printf("[WARNING] BIOS hybrid image not found at %s, skipping --grub2-mbr", hybridImg)
	}

	// Continue with generic arguments
	xorrisoArgs = append(xorrisoArgs,
		"-partition_offset", "16",
		"--mbr-force-bootable",
		"-eltorito-alt-boot",
		"-no-emul-boot",
		"-e", "isolinux/efiboot.img",
		"-append_partition", "2", "28732ac11ff8d211ba4b00a0c93ec93b", filepath.Join(isolinuxDir, "efiboot.img"),
		"-appended_part_as_gpt",
		"-iso_mbr_part_type", "a2a0d0ebe5b9334487c068b6b72699c7",
		"-m", "isolinux/efiboot.img",
		"-m", "isolinux/bios.img",
		"-e", "--interval:appended_partition_2:::",
		"-exclude", "isolinux",
		"-graft-points",
	)

	// Add available EFI boot files
	efiMap := map[string]string{
		"/EFI/boot/bootx64.efi": filepath.Join(isolinuxDir, "bootx64.efi"),
		"/EFI/boot/mmx64.efi":   filepath.Join(isolinuxDir, "mmx64.efi"),
		"/EFI/boot/grubx64.efi": filepath.Join(isolinuxDir, "grubx64.efi"),
	}
	for isoPath, hostPath := range efiMap {
		if _, err := os.Stat(hostPath); err == nil {
			xorrisoArgs = append(xorrisoArgs, isoPath+"="+hostPath)
		}
	}

	// Add mandatory files
	xorrisoArgs = append(xorrisoArgs,
		"/boot/grub/grub.cfg="+grubCfg,
		"/EFI/boot/grub.cfg="+grubCfg,
		"/EFI/ubuntu/grub.cfg="+grubCfg,
		"/EFI/debian/grub.cfg="+grubCfg,
		"/isolinux/bios.img="+biosImg,
		"/isolinux/efiboot.img="+efibootImg,
		b.ImageDir,
	)

	cmd := exec.Command("xorriso", xorrisoArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// cleanup unmounts filesystems and clears temporary data
func (b *Builder) cleanup() error {
	fmt.Println("[ACTION] Cleaning up mounts and temporary files...")

	// Order matters for unmounting (innermost first)
	mounts := []string{
		filepath.Join(b.ChrootDir, "dev/pts"),
		filepath.Join(b.ChrootDir, "dev"),
		filepath.Join(b.ChrootDir, "proc"),
		filepath.Join(b.ChrootDir, "sys"),
		filepath.Join(b.ChrootDir, "run"),
	}

	for _, mount := range mounts {
		if isMounted(mount) {
			// Try unmounting several times if busy
			for i := 0; i < 3; i++ {
				cmd := exec.Command("umount", "-l", mount) // Use lazy unmount for robustness
				if err := cmd.Run(); err == nil {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	return nil
}

// RemoveWorkspace deletes the entire workspace directory including chroot and image
func (b *Builder) RemoveWorkspace() error {
	fmt.Printf("[ACTION] Removing workspace directory: %s\n", b.WorkDir)

	// Ensure everything is unmounted first
	b.cleanup()

	// Use rm -rf via exec to handle potential permission issues better than os.RemoveAll
	cmd := exec.Command("rm", "-rf", b.WorkDir)
	return cmd.Run()
}

// chrootExec executes a command inside the chroot environment
func (b *Builder) chrootExec(command string) error {
	cmd := exec.Command("chroot", b.ChrootDir, "/bin/bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// chrootExecOutput executes a command and returns its output
func (b *Builder) chrootExecOutput(command string) (string, error) {
	cmd := exec.Command("chroot", b.ChrootDir, "/bin/bash", "-c", command)
	output, err := cmd.Output()
	return string(output), err
}

// generateSourcesList creates apt sources.list content
func (b *Builder) generateSourcesList() string {
	mirror := b.Config.Repository.Mirror
	if mirror == "" {
		if b.isDebian() {
			mirror = "http://deb.debian.org/debian/"
		} else {
			mirror = "http://archive.ubuntu.com/ubuntu/"
		}
	}

	var sourcesContent string
	if b.isDebian() {
		if b.DebianAlias == "unstable" || b.Config.Release == "sid" {
			// Sid/Unstable: Single repository, no security or updates
			sourcesContent = fmt.Sprintf(`cat > /etc/apt/sources.list <<'EOF'
deb %s %s main contrib non-free non-free-firmware
deb-src %s %s main contrib non-free non-free-firmware
`, mirror, b.Config.Release, mirror, b.Config.Release)
		} else {

			// Stable/Testing
			sourcesContent = fmt.Sprintf(`cat > /etc/apt/sources.list <<'EOF'
deb %s %s main contrib non-free non-free-firmware
deb-src %s %s main contrib non-free non-free-firmware

deb %s %s-updates main contrib non-free non-free-firmware
deb-src %s %s-updates main contrib non-free non-free-firmware

deb http://security.debian.org/debian-security %s-security main contrib non-free non-free-firmware
deb-src http://security.debian.org/debian-security %s-security main contrib non-free non-free-firmware
`,
				mirror, b.Config.Release,
				mirror, b.Config.Release,
				mirror, b.Config.Release,
				mirror, b.Config.Release,
				b.Config.Release,
				b.Config.Release,
			)
		}
	} else {
		// Ubuntu
		sourcesContent = fmt.Sprintf(`cat > /etc/apt/sources.list <<'EOF'
deb %s %s main restricted universe multiverse
deb-src %s %s main restricted universe multiverse

deb %s %s-security main restricted universe multiverse
deb-src %s %s-security main restricted universe multiverse

deb %s %s-updates main restricted universe multiverse
deb-src %s %s-updates main restricted universe multiverse
`,
			mirror, b.Config.Release,
			mirror, b.Config.Release,
			mirror, b.Config.Release,
			mirror, b.Config.Release,
			mirror, b.Config.Release,
			mirror, b.Config.Release,
		)
	}

	// Add proposed if requested
	if b.Config.Repository.UseProposed && b.Config.Release != "sid" && b.Config.Release != "unstable" {
		sourcesContent += fmt.Sprintf(`
deb %s %s-proposed main restricted universe multiverse
deb-src %s %s-proposed main restricted universe multiverse
`,
			mirror, b.Config.Release,
			mirror, b.Config.Release,
		)
	}

	sourcesContent += "EOF"

	return sourcesContent
}

// configureAdditionalRepos adds additional repositories from config
func (b *Builder) configureAdditionalRepos() error {
	// ... (rest is same as before)
	keyringsDir := filepath.Join(b.ChrootDir, "etc", "apt", "keyrings")
	if err := os.MkdirAll(keyringsDir, 0755); err != nil {
		return fmt.Errorf("failed to create keyrings directory: %v", err)
	}

	for _, repo := range b.Config.Repository.AdditionalRepos {
		log.Printf("Adding repository: %s", repo.Name)

		keyName := fmt.Sprintf("%s.gpg", repo.Name)
		keyPath := filepath.Join(keyringsDir, keyName)

		// This path is relative to chroot for sources.list usage
		chrootKeyPath := filepath.Join("/etc/apt/keyrings", keyName)
		signedByOpt := fmt.Sprintf("[signed-by=%s]", chrootKeyPath)

		// Add GPG key if provided
		if repo.Key != "" {
			if strings.HasPrefix(repo.Key, "http://") || strings.HasPrefix(repo.Key, "https://") {
				// Download key from URL using host tools
				// Handle both binary (.gpg) and armored keys
				if strings.HasSuffix(repo.Key, ".gpg") {
					// Direct download for binary keys
					cmd := exec.Command("wget", "-qO", keyPath, repo.Key)
					if output, err := cmd.CombinedOutput(); err != nil {
						log.Printf("Warning: Failed to download binary key for %s: %v\nOutput: %s", repo.Name, err, string(output))
					}
				} else {
					// Download and dearmor for armored keys
					// We pipe wget output to gpg
					wgetCmd := exec.Command("wget", "-qO-", repo.Key)
					gpgCmd := exec.Command("gpg", "--dearmor", "-o", keyPath)

					wgetOut, err := wgetCmd.StdoutPipe()
					if err != nil {
						log.Printf("Warning: Failed to create pipe for key download: %v", err)
						continue
					}
					gpgCmd.Stdin = wgetOut

					if err := wgetCmd.Start(); err != nil {
						log.Printf("Warning: Failed to start wget for key: %v", err)
						continue
					}

					if output, err := gpgCmd.CombinedOutput(); err != nil {
						log.Printf("Warning: Failed to dearmor key for %s: %v\nOutput: %s", repo.Name, err, string(output))
					}

					wgetCmd.Wait()
				}
			} else {
				// Inline key - assume armored
				cmd := exec.Command("gpg", "--dearmor", "-o", keyPath)
				cmd.Stdin = strings.NewReader(repo.Key)
				if output, err := cmd.CombinedOutput(); err != nil {
					log.Printf("Warning: Failed to process inline key for %s: %v\nOutput: %s", repo.Name, err, string(output))
				}
			}
		} else {
			// No key provided
			signedByOpt = ""
		}

		// Create repository entry
		components := strings.Join(repo.Components, " ")
		repoLine := fmt.Sprintf("deb %s %s %s %s", signedByOpt, repo.URI, repo.Suite, components)
		// Clean up double spaces if signedByOpt is empty
		repoLine = strings.Replace(repoLine, "  ", " ", -1)

		repoFilePath := filepath.Join(b.ChrootDir, "etc", "apt", "sources.list.d", fmt.Sprintf("%s.list", repo.Name))

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(repoFilePath), 0755); err != nil {
			return fmt.Errorf("failed to create sources.list.d directory: %v", err)
		}

		if err := os.WriteFile(repoFilePath, []byte(repoLine+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to create repo file for %s: %v", repo.Name, err)
		}
	}

	return nil
}

// isMounted checks if a path is mounted
func isMounted(path string) bool {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer file.Close()

	// Ensure path is absolute and clean
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			// Check if the mount point (field 1) matches exactly
			if fields[1] == absPath {
				return true
			}
		}
	}
	return false
}

// InteractivePackageSelection allows user to select packages interactively
func (b *Builder) InteractivePackageSelection() error {
	fmt.Println("\n=== Interactive Package Selection ===")
	fmt.Println("Enter additional packages to install (comma-separated, or press Enter to skip):")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	input = strings.TrimSpace(input)
	if input != "" {
		packages := strings.Split(input, ",")
		for i, pkg := range packages {
			packages[i] = strings.TrimSpace(pkg)
		}
		b.Config.Packages.Additional = append(b.Config.Packages.Additional, packages...)
	}

	fmt.Println("\nPackages to be installed:")
	fmt.Println("Essential:", strings.Join(b.Config.Packages.Essential, ", "))
	fmt.Println("Additional:", strings.Join(b.Config.Packages.Additional, ", "))
	fmt.Println()

	return nil
}
