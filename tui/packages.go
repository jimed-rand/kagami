package tui

func getMinimalWMPackages(wm string) []string {
	base := []string{
		"xorg", "xinit", "xterm", "lightdm", "lightdm-gtk-greeter",
		"network-manager-gnome", "dbus-x11", "fonts-dejavu-core",
		"lxappearance", "pcmanfm", "mousepad",
	}
	switch wm {
	case "openbox":
		base = append(base, "openbox", "obconf", "tint2", "feh", "dunst", "lxpolkit")
	case "dwm":
		base = append(base, "dwm", "dmenu", "stterm", "feh", "dunst", "lxpolkit")
	case "xfce4-minimal":
		base = append(base, "xfce4-panel", "xfce4-session", "xfce4-settings", "xfce4-terminal", "xfdesktop4", "xfwm4", "thunar")
	}
	return base
}
