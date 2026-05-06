package chroot

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"kagami/internal/build"
)

// memtestArchive is the upstream Memtest86+ release archive. Mirrors the
// download URL in references/build.sh.
const memtestArchive = "https://memtest.org/download/v7.00/mt86plus_7.00.binaries.zip"

// buildImage assembles the live image directory at /image: kernel + initrd,
// memtest86+, GRUB config (BIOS + UEFI), the EFI assets, manifests, and
// md5sum.txt. Mirrors build_image in references/build.sh.
func (p *Pipeline) buildImage(ctx context.Context) error {
	const root = "/image"

	if err := p.Runner.Run(ctx, "rm", "-rf", root); err != nil {
		return fmt.Errorf("clean %s: %w", root, err)
	}
	for _, d := range []string{
		"/image/casper",
		"/image/boot/grub",
		"/image/install",
		"/image/EFI/boot",
		"/image/EFI/ubuntu",
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	if err := p.copyKernelAndInitrd(); err != nil {
		return err
	}
	if err := installMemtest86(ctx, p.Runner); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(root, "ubuntu"), []byte{}, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "boot/grub/grub.cfg"), []byte(grubCfg(p.Config.LiveBootLabel)), 0o644); err != nil {
		return err
	}

	if err := p.writeManifests(ctx); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "README.diskdefines"), []byte(readmeDiskDefines(p.Config.LiveBootLabel)), 0o644); err != nil {
		return err
	}
	if err := installEFIAssets(); err != nil {
		return err
	}
	if err := buildEfibootImg(ctx, p.Runner); err != nil {
		return err
	}
	if err := buildBIOSImg(ctx, p.Runner); err != nil {
		return err
	}
	return writeMD5Sum(root)
}

func (p *Pipeline) copyKernelAndInitrd() error {
	vmlinuz, err := newestMatch("/boot", "vmlinuz-")
	if err != nil || vmlinuz == "" {
		return fmt.Errorf("no /boot/vmlinuz-* file found (kernel install failed?)")
	}
	initrd, err := newestMatch("/boot", "initrd.img-")
	if err != nil || initrd == "" {
		return fmt.Errorf("no /boot/initrd.img-* file found (kernel install failed?)")
	}
	if err := copyFile(vmlinuz, "/image/casper/vmlinuz"); err != nil {
		return err
	}
	return copyFile(initrd, "/image/casper/initrd")
}

// newestMatch returns the lexicographically-greatest entry in dir whose name
// starts with prefix (mirrors `ls -1 ... | sort -V | tail -1`).
func newestMatch(dir, prefix string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "", nil
	}
	sort.Strings(names)
	return filepath.Join(dir, names[len(names)-1]), nil
}

func installMemtest86(ctx context.Context, r interface {
	Run(ctx context.Context, name string, args ...string) error
}) error {
	zipPath := "/image/install/memtest86.zip"
	if err := r.Run(ctx, "wget", "--progress=dot", memtestArchive, "-O", zipPath); err != nil {
		return fmt.Errorf("download memtest archive: %w", err)
	}
	if err := unzipMember(zipPath, "memtest64.bin", "/image/install/memtest86+.bin"); err != nil {
		return err
	}
	if err := unzipMember(zipPath, "memtest64.efi", "/image/install/memtest86+.efi"); err != nil {
		return err
	}
	return os.Remove(zipPath)
}

// unzipMember extracts member from zipPath to dst using `unzip -p`.
func unzipMember(zipPath, member, dst string) error {
	cmd := exec.Command("unzip", "-p", zipPath, member)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// grubCfg returns the live image's grub.cfg, parameterized by the live-boot
// menu entry label. Mirrors the heredoc in references/build.sh build_image.
func grubCfg(liveBootLabel string) string {
	return fmt.Sprintf(`
search --set=root --file /ubuntu

insmod all_video

set default="0"
set timeout=30

menuentry "%s" {
    linux /casper/vmlinuz boot=casper nopersistent quiet splash ---
    initrd /casper/initrd
}

menuentry "Check the disc for defects" {
    linux /casper/vmlinuz boot=casper integrity-check quiet splash ---
    initrd /casper/initrd
}

grub_platform
if [ "$grub_platform" = "efi" ]; then
menuentry "UEFI firmware settings" {
    fwsetup
}

menuentry "Test memory with Memtest86+ (UEFI)" {
    linux /install/memtest86+.efi
}
else
menuentry "Test memory with Memtest86+ (BIOS)" {
    linux16 /install/memtest86+.bin
}
fi
`, liveBootLabel)
}

func readmeDiskDefines(liveBootLabel string) string {
	return fmt.Sprintf(`#define DISKNAME  %s
#define TYPE  binary
#define TYPEbinary  1
#define ARCH  amd64
#define ARCHamd64  1
#define DISKNUM  1
#define DISKNUM1  1
#define TOTALNUM  0
#define TOTALNUM0  1
`, liveBootLabel)
}

// writeManifests writes filesystem.manifest and filesystem.manifest-desktop.
// The -desktop variant strips packages listed in TARGET_PACKAGE_REMOVE.
func (p *Pipeline) writeManifests(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "dpkg-query", "-W", "--showformat=${Package} ${Version}\n").Output()
	if err != nil {
		return fmt.Errorf("dpkg-query manifest: %w", err)
	}
	manifest := "/image/casper/filesystem.manifest"
	if err := os.WriteFile(manifest, out, 0o644); err != nil {
		return err
	}

	desktopManifest := "/image/casper/filesystem.manifest-desktop"
	if err := copyFile(manifest, desktopManifest); err != nil {
		return err
	}

	remove := build.DefaultPackageRemove(p.Config.Installer)
	if len(remove) == 0 {
		return nil
	}
	raw, err := os.ReadFile(desktopManifest)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	filtered := make([]string, 0, len(lines))
nextLine:
	for _, line := range lines {
		for _, pkg := range remove {
			if strings.Contains(line, pkg) {
				continue nextLine
			}
		}
		filtered = append(filtered, line)
	}
	return os.WriteFile(desktopManifest, []byte(strings.Join(filtered, "\n")), 0o644)
}

// installEFIAssets copies the signed shim, mmx64, and grubx64 EFI binaries
// into /image/EFI/boot and copies grub.cfg to /image/EFI/ubuntu.
func installEFIAssets() error {
	pairs := [][2]string{
		{"/usr/lib/shim/shimx64.efi.signed.previous", "/image/EFI/boot/bootx64.efi"},
		{"/usr/lib/shim/mmx64.efi", "/image/EFI/boot/mmx64.efi"},
		{"/usr/lib/grub/x86_64-efi-signed/grubx64.efi.signed", "/image/EFI/boot/grubx64.efi"},
		{"/image/boot/grub/grub.cfg", "/image/EFI/ubuntu/grub.cfg"},
	}
	for _, p := range pairs {
		if err := copyFile(p[0], p[1]); err != nil {
			return fmt.Errorf("copy %s -> %s: %w", p[0], p[1], err)
		}
	}
	return nil
}

// buildEfibootImg creates a 10MB FAT-formatted image and populates it with
// the signed EFI binaries, ready for xorriso to append as GPT partition 2.
func buildEfibootImg(ctx context.Context, r interface {
	Run(ctx context.Context, name string, args ...string) error
}) error {
	const dir = "/image/boot/grub"
	img := filepath.Join(dir, "efiboot.img")

	if err := r.Run(ctx, "dd", "if=/dev/zero", "of="+img, "bs=1M", "count=10"); err != nil {
		return fmt.Errorf("dd zero efiboot.img: %w", err)
	}
	if err := r.Run(ctx, "mkfs.vfat", "-F", "16", img); err != nil {
		return fmt.Errorf("mkfs.vfat efiboot.img: %w", err)
	}

	envCmd := func(argv ...string) error {
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Env = append(os.Environ(), "LC_CTYPE=C")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	dirs := []string{"efi", "efi/ubuntu", "efi/boot"}
	for _, d := range dirs {
		if err := envCmd("mmd", "-i", img, d); err != nil {
			return fmt.Errorf("mmd %s: %w", d, err)
		}
	}
	copies := [][2]string{
		{"/image/EFI/boot/bootx64.efi", "::efi/boot/bootx64.efi"},
		{"/image/EFI/boot/mmx64.efi", "::efi/boot/mmx64.efi"},
		{"/image/EFI/boot/grubx64.efi", "::efi/boot/grubx64.efi"},
		{"/image/boot/grub/grub.cfg", "::efi/ubuntu/grub.cfg"},
	}
	for _, c := range copies {
		if err := envCmd("mcopy", "-i", img, c[0], c[1]); err != nil {
			return fmt.Errorf("mcopy %s: %w", c[0], err)
		}
	}
	return nil
}

// buildBIOSImg uses grub-mkstandalone + cdboot.img to produce bios.img used by
// xorriso for legacy BIOS El Torito booting.
func buildBIOSImg(ctx context.Context, r interface {
	Run(ctx context.Context, name string, args ...string) error
}) error {
	if err := r.Run(ctx,
		"grub-mkstandalone",
		"--format=i386-pc",
		"--output=/image/boot/grub/core.img",
		"--install-modules=linux16 linux normal iso9660 biosdisk memdisk search tar ls",
		"--modules=linux16 linux normal iso9660 biosdisk search",
		"--locales=",
		"--fonts=",
		"boot/grub/grub.cfg=/image/boot/grub/grub.cfg",
	); err != nil {
		return fmt.Errorf("grub-mkstandalone: %w", err)
	}
	return concatFiles("/image/boot/grub/bios.img",
		"/usr/lib/grub/i386-pc/cdboot.img",
		"/image/boot/grub/core.img")
}

func concatFiles(out string, parts ...string) error {
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()
	for _, p := range parts {
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		_ = r.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// writeMD5Sum walks the image tree and writes md5sum.txt at the root,
// excluding boot/grub/efiboot.img and boot/grub/bios.img (matching
// references/build.sh).
func writeMD5Sum(root string) error {
	out, err := os.Create(filepath.Join(root, "md5sum.txt"))
	if err != nil {
		return err
	}
	defer out.Close()
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "boot/grub/efiboot.img" || rel == "boot/grub/bios.img" || rel == "md5sum.txt" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := md5.New()
		_, err = io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "%x  ./%s\n", h.Sum(nil), rel)
		return err
	})
}
