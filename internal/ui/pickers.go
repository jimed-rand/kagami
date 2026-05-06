package ui

import (
	"errors"
	"strings"
)

// ErrNoTTY is returned by pickers when interactive input is required but
// stdin is not a terminal.
var ErrNoTTY = errors.New("no terminal is available for interactive input")

// PickRelease prompts the user to choose an Ubuntu release. It returns one of
// "jammy", "noble", or "resolute".
func (u *UI) PickRelease() (string, error) {
	if !IsTTY() {
		u.Err("No terminal is available. Use --release=jammy|noble|resolute.")
		return "", ErrNoTTY
	}
	u.Heading("Ubuntu release")
	u.Println("    1) jammy     Ubuntu 22.04 LTS")
	u.Println("    2) noble     Ubuntu 24.04 LTS")
	u.Println("    3) resolute  Ubuntu 26.04 LTS")

	for {
		ans, err := u.ReadLine("Release [1/2/3]")
		if err != nil {
			return "", err
		}
		switch strings.ToLower(ans) {
		case "1", "jammy":
			u.OK("release=jammy")
			return "jammy", nil
		case "2", "noble":
			u.OK("release=noble")
			return "noble", nil
		case "3", "resolute":
			u.OK("release=resolute")
			return "resolute", nil
		case "":
			u.Warn("Please choose 1, 2, or 3.")
		default:
			u.Warn("Invalid selection: '" + ans + "'. Please choose 1, 2, or 3.")
		}
	}
}

// PickInstaller prompts the user to choose an installer. release is required
// because Ubiquity is only valid on jammy.
func (u *UI) PickInstaller(release string) (string, error) {
	if !IsTTY() {
		u.Err("No terminal is available. Use --installer=calamares|ubiquity.")
		return "", ErrNoTTY
	}
	u.Heading("Live installer")
	u.Println("    1) Calamares  Default. Project config in references/calamares (all releases)")
	u.Println("    2) Ubiquity   Classic Ubuntu installer (supported only on jammy / 22.04 LTS)")

	for {
		ans, err := u.ReadLine("Installer [1/2, Enter=1]")
		if err != nil {
			return "", err
		}
		switch strings.ToLower(ans) {
		case "", "1", "c", "calamares":
			u.OK("installer=calamares")
			return "calamares", nil
		case "2", "u", "ubiquity":
			if release != "jammy" {
				u.Warn("Ubiquity is supported only on Ubuntu 22.04 LTS (jammy).")
				u.Warn("Current release: '" + release + "'. Choose 1 (Calamares),")
				u.Warn("or restart with --release=jammy if you need Ubiquity.")
				continue
			}
			u.OK("installer=ubiquity")
			return "ubiquity", nil
		default:
			u.Warn("Invalid selection: '" + ans + "'.")
		}
	}
}

// PickKernel prompts the user to choose a kernel flavor. hweVersion (e.g.
// "24.04") is included in the option labels when non-empty.
func (u *UI) PickKernel(hweVersion string) (string, error) {
	if !IsTTY() {
		u.Err("No terminal is available. Use --kernel=generic|lowlatency.")
		return "", ErrNoTTY
	}
	hint := ""
	if hweVersion != "" {
		hint = "  (linux-generic-hwe-" + hweVersion + ")"
	}
	hintLow := ""
	if hweVersion != "" {
		hintLow = "  (linux-lowlatency-hwe-" + hweVersion + ")"
	}
	suffix := ""
	if hweVersion != "" {
		suffix = " (HWE stream for Ubuntu " + hweVersion + ")"
	}
	u.Heading("Kernel flavor" + suffix)
	u.Println("    1) generic     Recommended for most systems" + hint)
	u.Println("    2) lowlatency  Better for audio / low-latency workloads" + hintLow)

	for {
		ans, err := u.ReadLine("Kernel [1/2]")
		if err != nil {
			return "", err
		}
		switch strings.ToLower(ans) {
		case "1", "g", "generic":
			u.OK("kernel=generic")
			return "generic", nil
		case "2", "l", "lowlatency":
			u.OK("kernel=lowlatency")
			return "lowlatency", nil
		case "":
			u.Warn("Please choose 1 or 2.")
		default:
			u.Warn("Invalid selection: '" + ans + "'. Please choose 1 or 2.")
		}
	}
}

// PickDesktop prompts the user to choose a desktop flavor.
func (u *UI) PickDesktop() (string, error) {
	if !IsTTY() {
		u.Err("No terminal is available. Use --desktop=gnome|xfce.")
		return "", ErrNoTTY
	}
	u.Heading("Desktop environment")
	u.Println("    1) GNOME    Default. vanilla-gnome-desktop (Recommends asked next)")
	u.Println("    2) XFCE     Lighter. xfce4 + xfce4-goodies + lightdm + slick-greeter")

	for {
		ans, err := u.ReadLine("Desktop [1/2, Enter=1]")
		if err != nil {
			return "", err
		}
		switch strings.ToLower(ans) {
		case "", "1", "g", "gnome":
			u.OK("desktop=gnome")
			return "gnome", nil
		case "2", "x", "xfce":
			u.OK("desktop=xfce")
			return "xfce", nil
		default:
			u.Warn("Invalid selection: '" + ans + "'.")
		}
	}
}

// PickGnomeRecommends prompts the user about installing GNOME with Recommends.
// Default is "no" (lightweight install).
func (u *UI) PickGnomeRecommends() (bool, error) {
	if !IsTTY() {
		u.Err("No terminal is available. Use --gnome-recommends=0|1.")
		return false, ErrNoTTY
	}
	u.Heading("GNOME extra Recommends")
	u.Println("    y) apt install vanilla-gnome-desktop     (fuller GNOME experience)")
	u.Println("    n) apt install --no-install-recommends   (lighter; default)")
	yes := u.Confirm("Include recommended packages?", false)
	if yes {
		u.OK("gnome-recommends=1")
	} else {
		u.OK("gnome-recommends=0")
	}
	return yes, nil
}
