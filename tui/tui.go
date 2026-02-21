package tui

import (
	"kagami/pkg/config"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func Run() (*config.Config, string, error) {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	tview.Styles.ContrastBackgroundColor = tcell.ColorWhite
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorWhite
	tview.Styles.PrimaryTextColor = tcell.ColorDefault
	tview.Styles.SecondaryTextColor = tcell.ColorWhite
	tview.Styles.TertiaryTextColor = tcell.ColorBlack
	tview.Styles.InverseTextColor = tcell.ColorBlack
	tview.Styles.ContrastSecondaryTextColor = tcell.ColorBlack
	tview.Styles.BorderColor = tcell.ColorDefault
	tview.Styles.TitleColor = tcell.ColorDefault
	tview.Styles.GraphicsColor = tcell.ColorDefault

	app := tview.NewApplication()
	pages := tview.NewPages()

	var (
		distroChoice    string
		releaseChoice   string
		modeChoice      string
		wmChoice        string
		desktopChoice   string
		installerChoice string
		slideshowChoice string
		hostnameInput   string
		archChoice      string
		kernelChoice    string
		extraPkgsInput  string
		mirrorInput     string
		brandName       string
		brandShort      string
		brandUrl        string
		brandSupport    string
		brandVersion    string
		flatpakChoice   bool
		snapdChoice     bool
		firewallChoice  bool
		outputInput     string
	)

	frameBorder := true
	formTemplate := func(title string) *tview.Form {
		f := tview.NewForm()
		f.SetBorder(frameBorder).SetTitle(" Kagami ISO Builder - " + title + " ").SetTitleAlign(tview.AlignLeft)
		return f
	}

	showPage := func(name string) {
		pages.SwitchToPage(name)
	}

	var formOutput *tview.Form

	formConfirm := formTemplate("Confirm")
	formConfirm.AddTextView("Summary", "Are you sure you want to proceed and save this configuration?", 0, 2, true, false).
		AddButton("Proceed", func() { app.Stop() }).
		AddButton("Abort", func() { outputInput = ""; app.Stop() })

	formOutput = formTemplate("Output")
	formOutput.AddInputField("Configuration File Path", "kagami.json", 40, nil, func(text string) { outputInput = text }).
		AddButton("Next", func() {
			if outputInput == "" {
				outputInput = "kagami.json"
			}
			showPage("confirm")
		})

	formFirewall := formTemplate("Firewall")
	formFirewall.AddCheckbox("Enable UFW Firewall?", false, func(checked bool) { firewallChoice = checked }).
		AddButton("Next", func() { showPage("output") })

	formSnapd := formTemplate("Snapd")
	formSnapd.AddCheckbox("Apply permanent snapd suppression?", true, func(checked bool) { snapdChoice = checked }).
		AddButton("Next", func() { showPage("firewall") })

	formFlatpak := formTemplate("Flatpak")
	formFlatpak.AddCheckbox("Enable Flatpak Support?", true, func(checked bool) { flatpakChoice = checked }).
		AddButton("Next", func() {
			if distroChoice == "ubuntu" {
				showPage("snapd")
			} else {
				snapdChoice = false
				showPage("firewall")
			}
		})

	formBranding := formTemplate("Calamares Branding")
	formBranding.AddInputField("Product Name", "Debian GNU/Linux", 30, nil, func(text string) { brandName = text }).
		AddInputField("Short Product Name", "Debian", 20, nil, func(text string) { brandShort = text }).
		AddInputField("Product URL", "https://www.debian.org", 40, nil, func(text string) { brandUrl = text }).
		AddInputField("Support URL", "https://www.debian.org/support", 40, nil, func(text string) { brandSupport = text }).
		AddInputField("Version", "stable", 20, nil, func(text string) { brandVersion = text }).
		AddButton("Next", func() { showPage("flatpak") })

	formMirror := formTemplate("Mirror")
	formMirror.AddInputField("APT Repository Mirror", "http://archive.ubuntu.com/ubuntu/", 40, nil, func(text string) { mirrorInput = text }).
		AddButton("Next", func() {
			if installerChoice == "calamares" {
				showPage("branding")
			} else {
				showPage("flatpak")
			}
		})

	formExtraPkgs := formTemplate("Additional Packages")
	formExtraPkgs.AddInputField("Comma-separated packages", "", 50, nil, func(text string) { extraPkgsInput = text }).
		AddButton("Next", func() { showPage("mirror") })

	formKernel := formTemplate("Kernel")
	formKernel.AddDropDown("Kernel Variant", []string{"Standard", "Real-time", "Cloud", "Low-latency", "OEM"}, 0, func(option string, optionIndex int) {
		kernelChoice = option
	}).AddButton("Next", func() {
		showPage("extrapkgs")
	})

	formArch := formTemplate("Architecture")
	formArch.AddDropDown("Target Architecture", []string{"amd64", "arm64"}, 0, func(option string, optionIndex int) {
		archChoice = option
	}).AddButton("Next", func() {
		showPage("kernel")
	})

	formHostname := formTemplate("Hostname")
	formHostname.AddInputField("System Hostname", "linux-custom", 30, nil, func(text string) { hostnameInput = text }).
		AddButton("Next", func() {
			showPage("arch")
		})

	formSlideshow := formTemplate("Installer Slideshow")
	formSlideshow.AddDropDown("Slideshow Variant", []string{"ubuntu", "kubuntu", "xubuntu", "lubuntu", "ubuntu-mate"}, 0, func(option string, optionIndex int) {
		slideshowChoice = option
	}).AddButton("Next", func() {
		showPage("hostname")
	})

	formInstaller := formTemplate("Installer")
	formInstaller.AddDropDown("Select Installer", []string{"ubiquity", "calamares"}, 0, func(option string, optionIndex int) {
		installerChoice = option
	}).AddButton("Next", func() {
		if installerChoice == "ubiquity" {
			showPage("slideshow")
		} else {
			showPage("hostname")
		}
	})

	formDesktop := formTemplate("Desktop Environment")
	formDesktop.AddDropDown("Select Desktop", []string{"gnome", "kde", "xfce", "mate", "lxqt", "lxde", "none"}, 0, func(option string, optionIndex int) {
		desktopChoice = option
	}).AddButton("Next", func() {
		showPage("installer")
	})

	formWM := formTemplate("Window Manager")
	formWM.AddDropDown("Select Window Manager", []string{"openbox", "dwm", "xfce4-minimal"}, 0, func(option string, optionIndex int) {
		wmChoice = option
	}).AddButton("Next", func() {
		desktopChoice = "none"
		installerChoice = "calamares"
		showPage("hostname")
	})

	formMode := formTemplate("Build Mode")
	formMode.AddDropDown("Select Build Mode", []string{"desktop", "minimal"}, 0, func(option string, optionIndex int) {
		modeChoice = option
	}).AddButton("Next", func() {
		if modeChoice == "minimal" {
			showPage("wm")
		} else {
			showPage("desktop")
		}
	})

	formRelease := formTemplate("Release")
	formRelease.AddDropDown("Target Release", []string{"noble", "jammy", "resolute", "devel", "stable", "testing", "unstable", "bookworm", "trixie"}, 0, func(option string, optionIndex int) {
		releaseChoice = option
	}).AddButton("Next", func() {
		showPage("mode")
	})

	formDistro := formTemplate("Distribution")
	formDistro.AddDropDown("Base Distribution", []string{"ubuntu", "debian"}, 0, func(option string, optionIndex int) {
		distroChoice = option
	}).AddButton("Next", func() {
		showPage("release")
	})

	pages.AddPage("distro", formDistro, true, true)
	pages.AddPage("release", formRelease, true, false)
	pages.AddPage("mode", formMode, true, false)
	pages.AddPage("wm", formWM, true, false)
	pages.AddPage("desktop", formDesktop, true, false)
	pages.AddPage("installer", formInstaller, true, false)
	pages.AddPage("slideshow", formSlideshow, true, false)
	pages.AddPage("hostname", formHostname, true, false)
	pages.AddPage("arch", formArch, true, false)
	pages.AddPage("kernel", formKernel, true, false)
	pages.AddPage("extrapkgs", formExtraPkgs, true, false)
	pages.AddPage("mirror", formMirror, true, false)
	pages.AddPage("branding", formBranding, true, false)
	pages.AddPage("flatpak", formFlatpak, true, false)
	pages.AddPage("snapd", formSnapd, true, false)
	pages.AddPage("firewall", formFirewall, true, false)
	pages.AddPage("output", formOutput, true, false)
	pages.AddPage("confirm", formConfirm, true, false)

	if err := app.SetRoot(pages, true).EnableMouse(true).Run(); err != nil {
		return nil, "", err
	}

	if outputInput == "" {
		return nil, "", nil
	}

	if distroChoice == "" {
		distroChoice = "ubuntu"
	}
	if releaseChoice == "" {
		releaseChoice = "noble"
	}
	if desktopChoice == "" {
		desktopChoice = "none"
	}
	if installerChoice == "" {
		installerChoice = "calamares"
	}
	if archChoice == "" {
		archChoice = "amd64"
	}
	if kernelChoice == "" {
		kernelChoice = "linux-generic"
	}
	if mirrorInput == "" {
		if distroChoice == "debian" {
			mirrorInput = "http://deb.debian.org/debian/"
		} else {
			mirrorInput = "http://archive.ubuntu.com/ubuntu/"
		}
	}
	if hostnameInput == "" {
		hostnameInput = distroChoice + "-custom"
	}

	var addPkgs []string
	if extraPkgsInput != "" {
		for _, p := range strings.Split(extraPkgsInput, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				addPkgs = append(addPkgs, p)
			}
		}
	}

	if modeChoice == "minimal" && wmChoice != "" {
		addPkgs = append(addPkgs, getMinimalWMPackages(wmChoice)...)
	}

	kernelResolved := "linux-generic"
	if distroChoice == "debian" {
		switch kernelChoice {
		case "Standard":
			kernelResolved = "linux-image-amd64"
		case "Real-time":
			kernelResolved = "linux-image-rt-amd64"
		case "Cloud":
			kernelResolved = "linux-image-cloud-amd64"
		}
	} else {
		switch kernelChoice {
		case "Standard", "Generic":
			kernelResolved = "linux-generic"
		case "Low-latency":
			kernelResolved = "linux-lowlatency"
		case "OEM":
			kernelResolved = "linux-oem-24.04"
		}
	}

	cfg := &config.Config{
		Distro:  distroChoice,
		Release: releaseChoice,
		System: config.SystemConfig{
			Hostname:     hostnameInput,
			BlockSnapd:   snapdChoice,
			Architecture: archChoice,
			Locale:       "en_US.UTF-8",
			Timezone:     "UTC",
		},
		Repository: config.RepositoryConfig{
			Mirror: mirrorInput,
		},
		Packages: config.PackageConfig{
			Desktop:       desktopChoice,
			Additional:    addPkgs,
			Kernel:        kernelResolved,
			EnableFlatpak: flatpakChoice,
			WM:            wmChoice,
		},
		Installer: config.InstallerConfig{
			Type:      installerChoice,
			Slideshow: slideshowChoice,
			Branding: config.BrandingConfig{
				ProductName:      brandName,
				ShortProductName: brandShort,
				ProductUrl:       brandUrl,
				SupportUrl:       brandSupport,
				Version:          brandVersion,
			},
		},
		Security: config.SecurityConfig{
			EnableFirewall: firewallChoice,
		},
	}

	if err := cfg.SaveToFile(outputInput); err != nil {
		return nil, "", err
	}

	return cfg, outputInput, nil
}

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
