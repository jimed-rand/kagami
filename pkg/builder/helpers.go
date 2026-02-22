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
)

func (b *Builder) createFilesystem() error {
	liveDestDir := filepath.Join(b.ImageDir, b.liveDir())

	manifestPath := filepath.Join(liveDestDir, "filesystem.manifest")
	manifestDesktopPath := filepath.Join(liveDestDir, "filesystem.manifest-desktop")

	dpkgCmd := "dpkg-query -W --showformat='${Package} ${Version}\\n'"
	output, err := b.chrootExecOutput(dpkgCmd)
	if err != nil {
		return err
	}

	if err := os.WriteFile(manifestPath, []byte(output), 0644); err != nil {
		return err
	}

	var liveRemovePackages []string
	if b.isDebian() {
		liveRemovePackages = []string{
			"calamares",
			"live-boot",
			"live-boot-initramfs-tools",
			"live-config",
			"live-config-systemd",
			"discover",
			"laptop-detect",
			"os-prober",
		}
	} else {
		liveRemovePackages = []string{
			"ubiquity",
			"calamares",
			"casper",
			"discover",
			"laptop-detect",
			"os-prober",
		}
	}

	manifestContent := output
	for _, pkg := range liveRemovePackages {
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

	if !b.isDebian() {
		diskdefinesPath := filepath.Join(b.ImageDir, "README.diskdefines")
		distName := b.getDistName()
		diskdefines := fmt.Sprintf("#define DISKNAME  %s %s\n#define TYPE  binary\n#define TYPEbinary  1\n#define ARCH  %s\n#define ARCHamd64  1\n#define DISKNUM  1\n#define DISKNUM1  1\n#define TOTALNUM  0\n#define TOTALNUM0  1\n",
			distName, b.Config.Release, b.Config.System.Architecture)

		if err := os.WriteFile(diskdefinesPath, []byte(diskdefines), 0644); err != nil {
			return err
		}
	}

	imageInChroot := filepath.Join(b.ChrootDir, "image")
	if _, err := os.Stat(imageInChroot); err == nil {
		if err := os.Rename(imageInChroot, b.ImageDir); err != nil {
			log.Printf("[WARNING] Could not relocate image directory: %v", err)
		}
	}

	fmt.Println("[INFO] Creating compressed squashfs filesystem image (this may take several minutes)...")
	squashfsPath := filepath.Join(liveDestDir, "filesystem.squashfs")

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

	args = append(args, "-no-xattrs")

	if err := b.runCommand("mksquashfs", args...); err != nil {
		return err
	}

	sizePath := filepath.Join(liveDestDir, "filesystem.size")
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

func (b *Builder) createISO() error {
	fmt.Println("[INFO] Preparing EFI and BIOS boot loader components...")

	isolinuxDir := filepath.Join(b.ImageDir, "isolinux")
	if err := os.MkdirAll(isolinuxDir, 0755); err != nil {
		return fmt.Errorf("failed to create isolinux directory: %v", err)
	}

	copyFile := func(srcPaths []string, dstName string) bool {
		dst := filepath.Join(isolinuxDir, dstName)
		for _, path := range srcPaths {
			if _, err := os.Stat(path); err == nil {
				if exec.Command("cp", path, dst).Run() == nil {
					return true
				}
			}
			chrootPath := filepath.Join(b.ChrootDir, path)
			if _, err := os.Stat(chrootPath); err == nil {
				if exec.Command("cp", chrootPath, dst).Run() == nil {
					return true
				}
			}
		}
		return false
	}

	fmt.Println("[INFO] Copying EFI loader components (Shim/GRUB)...")

	shimPaths := []string{
		"/usr/lib/shim/shimx64.efi.signed",
		"/usr/lib/shim/shimx64.efi.signed.previous",
		"/usr/lib/shim/shimx64.efi",
		"/boot/efi/EFI/ubuntu/shimx64.efi",
		"/usr/lib/shim/shim.efi",
	}
	if !copyFile(shimPaths, "bootx64.efi") {
		log.Printf("[WARNING] shimx64.efi not located; UEFI Secure Boot may be unavailable")
	}

	mmPaths := []string{
		"/usr/lib/shim/mmx64.efi",
		"/usr/lib/shim/mmx64.efi.signed",
		"/boot/efi/EFI/ubuntu/mmx64.efi",
	}
	copyFile(mmPaths, "mmx64.efi")

	grubEfiPaths := []string{
		"/usr/lib/grub/x86_64-efi-signed/grubx64.efi.signed",
		"/usr/lib/grub/x86_64-efi/monolithic/grubx64.efi",
		"/boot/efi/EFI/ubuntu/grubx64.efi",
		"/usr/lib/grub/x86_64-efi/grub.efi",
	}
	if !copyFile(grubEfiPaths, "grubx64.efi") {
		log.Printf("[WARNING] grubx64.efi not found in standard locations; initiating search...")
		findCmd := exec.Command("find", b.ChrootDir, "-name", "grubx64.efi", "-o", "-name", "grubx64.efi.signed")
		output, _ := findCmd.Output()
		foundPaths := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(foundPaths) > 0 && foundPaths[0] != "" {
			exec.Command("cp", foundPaths[0], filepath.Join(isolinuxDir, "grubx64.efi")).Run()
		} else {
			return fmt.Errorf("mandatory EFI loader grubx64.efi could not be located")
		}
	}

	fmt.Println("[INFO] Creating EFI boot image...")

	efibootImg := filepath.Join(isolinuxDir, "efiboot.img")
	grubCfg := filepath.Join(isolinuxDir, "grub.cfg")

	if err := exec.Command("dd", "if=/dev/zero", "of="+efibootImg, "bs=1M", "count=10").Run(); err != nil {
		return err
	}
	if err := exec.Command("mkfs.vfat", "-F", "16", efibootImg).Run(); err != nil {
		return err
	}

	exec.Command("mmd", "-i", efibootImg, "efi", "efi/ubuntu", "efi/debian", "efi/boot").Run()

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

	fmt.Println("[INFO] Creating GRUB BIOS image...")

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

	fmt.Println("[INFO] Computing MD5 checksums...")

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

	content, _ := os.ReadFile(md5Path)
	lines := strings.Split(string(content), "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.Contains(line, "isolinux") && line != "" {
			filtered = append(filtered, line)
		}
	}
	os.WriteFile(md5Path, []byte(strings.Join(filtered, "\n")), 0644)

	fmt.Println("[INFO] Synthesising final ISO image...")

	hybridImg := "/usr/lib/grub/i386-pc/boot_hybrid.img"
	volid := fmt.Sprintf("KAGAMI_%s_%s", strings.ToUpper(b.Config.Release), strings.ToUpper(b.Config.System.Architecture))

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

	if _, err := os.Stat(hybridImg); err == nil {
		xorrisoArgs = append(xorrisoArgs, "--grub2-mbr", hybridImg)
	} else {
		log.Printf("[WARNING] BIOS hybrid image not found at %s; --grub2-mbr omitted", hybridImg)
	}

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

	xorrisoArgs = append(xorrisoArgs,
		"/boot/grub/grub.cfg="+grubCfg,
		"/EFI/boot/grub.cfg="+grubCfg,
		"/EFI/ubuntu/grub.cfg="+grubCfg,
		"/EFI/debian/grub.cfg="+grubCfg,
		"/isolinux/bios.img="+biosImg,
		"/isolinux/efiboot.img="+efibootImg,
		b.ImageDir,
	)

	return b.runCommand("xorriso", xorrisoArgs...)
}

func (b *Builder) cleanup() error {
	fmt.Println("[INFO] Unmounting filesystems and releasing temporary resources...")

	mounts := []string{
		filepath.Join(b.ChrootDir, "dev/pts"),
		filepath.Join(b.ChrootDir, "dev"),
		filepath.Join(b.ChrootDir, "proc"),
		filepath.Join(b.ChrootDir, "sys"),
		filepath.Join(b.ChrootDir, "run"),
	}

	for _, mount := range mounts {
		if isMounted(mount) {
			for i := 0; i < 3; i++ {
				cmd := exec.Command("umount", "-l", mount)
				if err := cmd.Run(); err == nil {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	return nil
}

func (b *Builder) RemoveWorkspace() error {
	b.log(fmt.Sprintf("[INFO] Removing build workspace: %s\n", b.WorkDir))
	b.cleanup()
	cmd := exec.Command("rm", "-rf", b.WorkDir)
	return cmd.Run()
}

func (b *Builder) chrootExec(command string) error {
	cmd := exec.Command("chroot", b.ChrootDir, "/bin/bash", "-c", command)
	if b.OnLog != nil {
		cmd.Stdout = &logWriter{b}
		cmd.Stderr = &logWriter{b}
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func (b *Builder) chrootExecOutput(command string) (string, error) {
	cmd := exec.Command("chroot", b.ChrootDir, "/bin/bash", "-c", command)
	output, err := cmd.Output()
	return string(output), err
}

type logWriter struct {
	b *Builder
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.b.log(string(p))
	return len(p), nil
}

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
			sourcesContent = fmt.Sprintf(`cat > /etc/apt/sources.list <<'EOF'
deb %s %s main contrib non-free non-free-firmware
deb-src %s %s main contrib non-free non-free-firmware
`, mirror, b.Config.Release, mirror, b.Config.Release)
		} else {
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

func (b *Builder) configureAdditionalRepos() error {
	keyringsDir := filepath.Join(b.ChrootDir, "etc", "apt", "keyrings")
	if err := os.MkdirAll(keyringsDir, 0755); err != nil {
		return fmt.Errorf("failed to create keyrings directory: %v", err)
	}

	for _, repo := range b.Config.Repository.AdditionalRepos {
		log.Printf("[INFO] Registering repository: %s", repo.Name)

		keyName := fmt.Sprintf("%s.gpg", repo.Name)
		keyPath := filepath.Join(keyringsDir, keyName)
		chrootKeyPath := filepath.Join("/etc/apt/keyrings", keyName)
		signedByOpt := fmt.Sprintf("[signed-by=%s]", chrootKeyPath)

		if repo.Key != "" {
			if strings.HasPrefix(repo.Key, "http://") || strings.HasPrefix(repo.Key, "https://") {
				if strings.HasSuffix(repo.Key, ".gpg") {
					cmd := exec.Command("wget", "-qO", keyPath, repo.Key)
					if output, err := cmd.CombinedOutput(); err != nil {
						log.Printf("[WARNING] Key download failed for %s: %v\n%s", repo.Name, err, string(output))
					}
				} else {
					wgetCmd := exec.Command("wget", "-qO-", repo.Key)
					gpgCmd := exec.Command("gpg", "--dearmor", "-o", keyPath)

					wgetOut, err := wgetCmd.StdoutPipe()
					if err != nil {
						log.Printf("[WARNING] Pipe creation failed for key download: %v", err)
						continue
					}
					gpgCmd.Stdin = wgetOut

					if err := wgetCmd.Start(); err != nil {
						log.Printf("[WARNING] Key download initiation failed: %v", err)
						continue
					}

					if output, err := gpgCmd.CombinedOutput(); err != nil {
						log.Printf("[WARNING] Key dearmoring failed for %s: %v\n%s", repo.Name, err, string(output))
					}

					wgetCmd.Wait()
				}
			} else {
				cmd := exec.Command("gpg", "--dearmor", "-o", keyPath)
				cmd.Stdin = strings.NewReader(repo.Key)
				if output, err := cmd.CombinedOutput(); err != nil {
					log.Printf("[WARNING] Inline key processing failed for %s: %v\n%s", repo.Name, err, string(output))
				}
			}
		} else {
			signedByOpt = ""
		}

		components := strings.Join(repo.Components, " ")
		repoLine := fmt.Sprintf("deb %s %s %s %s", signedByOpt, repo.URI, repo.Suite, components)
		repoLine = strings.ReplaceAll(repoLine, "  ", " ")

		repoFilePath := filepath.Join(b.ChrootDir, "etc", "apt", "sources.list.d", fmt.Sprintf("%s.list", repo.Name))

		if err := os.MkdirAll(filepath.Dir(repoFilePath), 0755); err != nil {
			return fmt.Errorf("failed to create sources.list.d directory: %v", err)
		}

		if err := os.WriteFile(repoFilePath, []byte(repoLine+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to create repository file for %s: %v", repo.Name, err)
		}
	}

	return nil
}

func isMounted(path string) bool {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer file.Close()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == absPath {
			return true
		}
	}
	return false
}

func (b *Builder) InteractivePackageSelection() error {
	fmt.Println("\n--- Interactive Package Selection ---")
	fmt.Println("Specify additional packages to install (comma-separated), or press Enter to skip:")

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

	fmt.Println("\nPackage manifest summary:")
	fmt.Println("  Essential: ", strings.Join(b.Config.Packages.Essential, ", "))
	fmt.Println("  Additional:", strings.Join(b.Config.Packages.Additional, ", "))
	fmt.Println()

	return nil
}
