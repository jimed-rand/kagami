// Package build holds shared build-time constants: HWE version map, kernel
// meta package resolution, and the curated package lists used by the chroot
// phase. These mirror the lists in references/build.sh.
package build

import "fmt"

// HWEVersion maps an Ubuntu release codename to its marketing version, used
// for the linux-*-hwe-* kernel meta packages.
func HWEVersion(release string) string {
	switch release {
	case "jammy":
		return "22.04"
	case "noble":
		return "24.04"
	case "resolute":
		return "26.04"
	}
	return ""
}

// KernelMetaPackage returns the linux meta package for the given release and
// flavor, or an error if either is unsupported.
func KernelMetaPackage(release, flavor string) (string, error) {
	hv := HWEVersion(release)
	if hv == "" {
		return "", fmt.Errorf("unsupported release %q for kernel meta package", release)
	}
	switch flavor {
	case "generic":
		return "linux-generic-hwe-" + hv, nil
	case "lowlatency":
		return "linux-lowlatency-hwe-" + hv, nil
	}
	return "", fmt.Errorf("unsupported kernel flavor %q (expected generic|lowlatency)", flavor)
}

// HostTools is the package set installed on the host before debootstrap.
var HostTools = []string{
	"debootstrap",
	"squashfs-tools",
	"xorriso",
}

// ChrootBase is the early base package set installed in the freshly bootstrapped
// chroot, before the main package install.
var ChrootBase = []string{
	"libterm-readline-gnu-perl",
	"systemd-sysv",
}

// CoreInstall is the curated "core" package list installed in the chroot.
// Mirrors install_pkg in references/build.sh.
var CoreInstall = []string{
	"sudo",
	"ubuntu-standard",
	"casper",
	"discover",
	"laptop-detect",
	"os-prober",
	"network-manager",
	"net-tools",
	"locales",
	"grub-common",
	"grub-gfxpayload-lists",
	"grub-pc",
	"grub-pc-bin",
	"grub2-common",
	"grub-efi-amd64-signed",
	"shim-signed",
	"mtools",
	"unzip",
	"binutils",
	"gparted",
	"dosfstools",
	"e2fsprogs",
	"btrfs-progs",
	"xfsprogs",
	"ntfs-3g",
	"parted",
}

// PlymouthSet is installed alongside the desktop to provide a boot splash.
var PlymouthSet = []string{
	"plymouth",
	"plymouth-label",
	"plymouth-theme-ubuntu-text",
}

// NetTools is the small toolkit needed for keyring downloads + repo setup
// (Brave, flathub).
var NetTools = []string{
	"curl",
	"apt-transport-https",
	"ca-certificates",
	"squashfs-tools",
}

// CommonCLI is the small CLI utility set added to every flavor.
var CommonCLI = []string{
	"git",
	"vim",
	"nano",
	"wget",
	"less",
}

// GnomeFull is the desktop package set when --gnome-recommends=1.
var GnomeFull = []string{
	"vanilla-gnome-desktop",
	"gnome-console",
}

// GnomeLite is the desktop package set when --gnome-recommends=0 (default).
var GnomeLite = GnomeFull // same names; install with --no-install-recommends.

// GnomeStoreSet is the GNOME software + flatpak plugin set.
var GnomeStoreSet = []string{
	"gnome-software",
	"gnome-software-plugin-flatpak",
}

// GnomePurge are the GNOME-specific games removed from the live image.
var GnomePurge = []string{
	"gnome-mahjongg",
	"gnome-mines",
	"gnome-sudoku",
}

// XFCEFull mirrors customize_image's XFCE branch in references/build.sh.
var XFCEFull = []string{
	"xfce4",
	"xfce4-goodies",
	"xfce4-terminal",
	"xfce4-notifyd",
	"xfce4-power-manager",
	"xfce4-pulseaudio-plugin",
	"xfce4-screensaver",
	"xfce4-taskmanager",
	"xfce4-indicator-plugin",
	"xfce4-whiskermenu-plugin",
	"thunar-archive-plugin",
	"thunar-media-tags-plugin",
	"thunar-volman",
	"tumbler",
	"gvfs",
	"gvfs-backends",
	"gvfs-fuse",
	"catfish",
	"menulibre",
	"mugshot",
	"gigolo",
	"galculator",
	"xarchiver",
	"blueman",
	"pulseaudio",
	"pavucontrol",
	"synaptic",
	"xdg-user-dirs",
	"xdg-user-dirs-gtk",
	"fonts-ubuntu",
	"fonts-noto-core",
	"hunspell-en-us",
	"onboard",
	"xorg",
	"lightdm",
	"slick-greeter",
	"labwc",
}

// CommonPurge are packages removed from the live image regardless of desktop.
var CommonPurge = []string{
	"transmission-gtk",
	"transmission-common",
	"aisleriot",
	"hitori",
}

// SlideshowPurge are slideshow packages we always strip (the build does not
// ship a calamares-/ubiquity-slideshow).
var SlideshowPurge = []string{
	"ubiquity-slideshow-ubuntu",
	"calamares-slideshow-ubuntu",
}

// DefaultPackageRemove returns the value of TARGET_PACKAGE_REMOVE used by
// build_image to derive filesystem.manifest-desktop. It depends on the
// installer in use because the live-only packages differ.
func DefaultPackageRemove(installer string) []string {
	switch installer {
	case "calamares":
		return []string{
			"calamares",
			"casper",
			"discover",
			"laptop-detect",
			"os-prober",
			"ubiquity-slideshow-ubuntu",
		}
	case "ubiquity":
		return []string{
			"ubiquity",
			"ubiquity-frontend-gtk",
			"ubiquity-ubuntu-artwork",
			"ubiquity-slideshow-ubuntu",
			"casper",
			"discover",
			"laptop-detect",
			"os-prober",
		}
	}
	return nil
}
