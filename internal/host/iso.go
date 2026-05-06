package host

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// EFI System Partition GUID (C12A7328-F81F-11D2-BA4B-00A0C93EC93B) in the
// on-disk mixed-endian byte order that xorriso's -append_partition expects.
const espTypeGUID = "28732ac11ff8d211ba4b00a0c93ec93b"

// Microsoft Basic Data Partition GUID (EBD0A0A2-B9E5-4433-87C0-68B6B72699C7)
// in mixed-endian, used as the ISO MBR partition type so the ISO9660 area is
// visible as a normal data partition when the stick is inspected.
const isoMBRTypeGUID = "a2a0d0ebe5b9334487c068b6b72699c7"

// buildISO produces the squashfs root, then a hybrid BIOS+UEFI ISO with
// matching SHA1/SHA256 sidecars. The xorriso invocation mirrors the layout
// in references/build.sh exactly.
func (p *Pipeline) buildISO(ctx context.Context) error {
	if err := p.Runner.RunPriv(ctx, "mkdir", "-p", p.Paths.Root); err != nil {
		return err
	}
	if err := p.Runner.RunPriv(ctx, "rm", "-rf", p.Paths.Image); err != nil {
		return err
	}
	if err := p.Runner.RunPriv(ctx, "mv", filepath.Join(p.Paths.Chroot, "image"), p.Paths.Image); err != nil {
		return fmt.Errorf("move chroot/image to %q: %w", p.Paths.Image, err)
	}

	if err := p.makeSquashfs(ctx); err != nil {
		return err
	}
	if err := p.writeFilesystemSize(ctx); err != nil {
		return err
	}
	if err := p.assertImageAssets(); err != nil {
		return err
	}
	if err := p.runXorriso(ctx); err != nil {
		return err
	}
	if err := p.writeISOHashes(ctx); err != nil {
		return err
	}

	// Cleanup workspace on success too, matching references/build.sh.
	if err := p.Runner.RunPriv(ctx, "rm", "-rf", p.Paths.Root); err != nil {
		p.UI.Warn("Failed to remove workspace " + p.Paths.Root + ": " + err.Error())
	}
	return nil
}

func (p *Pipeline) makeSquashfs(ctx context.Context) error {
	target := filepath.Join(p.Paths.Image, "casper", "filesystem.squashfs")
	args := []string{
		p.Paths.Chroot, target,
		"-noappend", "-no-duplicates", "-no-recovery",
		"-wildcards",
		"-comp", "xz", "-b", "1M", "-Xdict-size", "100%",
		"-e", "var/cache/apt/archives/*",
		"-e", "root/*",
		"-e", "root/.*",
		"-e", "tmp/*",
		"-e", "tmp/.*",
		"-e", "swapfile",
		"-e", "image",
	}
	return p.Runner.RunPriv(ctx, "mksquashfs", args...)
}

// writeFilesystemSize writes the byte size of the chroot tree (matching
// `du -sx --block-size=1`) to image/casper/filesystem.size.
func (p *Pipeline) writeFilesystemSize(ctx context.Context) error {
	out, err := p.Runner.CommandOutput(ctx, "sudo", "du", "-sx", "--block-size=1", p.Paths.Chroot)
	if err != nil {
		// On root, sudo may be missing; fall back to a direct call.
		out, err = p.Runner.CommandOutput(ctx, "du", "-sx", "--block-size=1", p.Paths.Chroot)
		if err != nil {
			return fmt.Errorf("du -sx %q: %w", p.Paths.Chroot, err)
		}
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return fmt.Errorf("du -sx returned no output")
	}
	if _, err := strconv.ParseInt(fields[0], 10, 64); err != nil {
		return fmt.Errorf("parse du output %q: %w", fields[0], err)
	}
	target := filepath.Join(p.Paths.Image, "casper", "filesystem.size")
	return p.writeFilePriv(ctx, target, []byte(fields[0]))
}

func (p *Pipeline) writeFilePriv(ctx context.Context, path string, data []byte) error {
	if err := p.Runner.RunPriv(ctx, "mkdir", "-p", filepath.Dir(path)); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "sudo", "tee", path)
	if os.Geteuid() == 0 {
		cmd = exec.CommandContext(ctx, "tee", path)
	}
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *Pipeline) assertImageAssets() error {
	bootHybrid := filepath.Join(p.Paths.Chroot, "usr/lib/grub/i386-pc/boot_hybrid.img")
	if _, err := os.Stat(bootHybrid); err != nil {
		return fmt.Errorf("missing %s (grub-pc-bin not installed in chroot?): %w", bootHybrid, err)
	}
	for _, rel := range []string{"boot/grub/bios.img", "boot/grub/efiboot.img"} {
		full := filepath.Join(p.Paths.Image, rel)
		if _, err := os.Stat(full); err != nil {
			return fmt.Errorf("missing %s (build_image step did not produce it): %w", full, err)
		}
	}
	return nil
}

func (p *Pipeline) runXorriso(ctx context.Context) error {
	bootHybrid := filepath.Join(p.Paths.Chroot, "usr/lib/grub/i386-pc/boot_hybrid.img")
	volid := isoVolumeID(p.Config.Name)
	isoOut := p.outputISOPath()

	args := []string{
		"-as", "mkisofs",
		"-r", "-V", volid,
		"-J", "-joliet-long",
		"-l",
		"-iso-level", "3",
		"-full-iso9660-filenames",
		"-o", isoOut,

		"--grub2-mbr", bootHybrid,
		"-partition_offset", "16",
		"--mbr-force-bootable",
		"-append_partition", "2", espTypeGUID, "boot/grub/efiboot.img",
		"-appended_part_as_gpt",
		"-iso_mbr_part_type", isoMBRTypeGUID,

		"-c", "boot.catalog",
		"-b", "boot/grub/bios.img",
		"-no-emul-boot",
		"-boot-load-size", "4",
		"-boot-info-table",
		"--grub2-boot-info",
		"-eltorito-alt-boot",
		"-e", "--interval:appended_partition_2:all::",
		"-no-emul-boot",

		".",
	}

	cmd := exec.CommandContext(ctx, "xorriso", args...)
	if os.Geteuid() != 0 {
		cmd = exec.CommandContext(ctx, "sudo", append([]string{"xorriso"}, args...)...)
	}
	cmd.Dir = p.Paths.Image
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isoVolumeID normalizes a name into the ISO 9660 vol id alphabet
// (A-Z 0-9 _, max 32 chars).
func isoVolumeID(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

func (p *Pipeline) outputISOPath() string {
	return filepath.Join(p.Config.RepoRoot, p.Config.Name+".iso")
}

func (p *Pipeline) writeISOHashes(ctx context.Context) error {
	iso := p.outputISOPath()
	if err := writeHashSidecar(iso, iso+".sha1", sha1.New); err != nil {
		return err
	}
	return writeHashSidecar(iso, iso+".sha256", sha256.New)
}

func writeHashSidecar(isoPath, sidecarPath string, ctor func() hash.Hash) error {
	in, err := os.Open(isoPath)
	if err != nil {
		return err
	}
	defer in.Close()
	h := ctor()
	if _, err := io.Copy(h, in); err != nil {
		return err
	}
	sum := fmt.Sprintf("%x  %s\n", h.Sum(nil), filepath.Base(isoPath))
	return os.WriteFile(sidecarPath, []byte(sum), 0o644)
}
