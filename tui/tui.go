package tui

import (
	"fmt"
	"os"
	"strings"

	"kagami/pkg/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type step int

const (
	stepDistro step = iota
	stepRelease
	stepMode
	stepWM
	stepDesktop
	stepInstaller
	stepSlideshow
	stepHostname
	stepArch
	stepKernel
	stepExtraPkgs
	stepMirror
	stepBrandingName
	stepBrandingShort
	stepBrandingUrl
	stepBrandingSupport
	stepBrandingVersion
	stepFlatpak
	stepSnapd
	stepFirewall
	stepOutputPath
	stepConfirm
	stepLogChoice
	stepDone
)

type menuOption struct {
	key  string
	text string
	desc string
}

type model struct {
	step           step
	width          int
	height         int
	choices        map[string]string
	cursor         int
	options        []menuOption
	textInput      string
	cursorPos      int
	additionalPkgs []string
	outputPath     string
	quitting       bool
	confirmed      bool
}

func newModel() model {
	return model{
		step:    stepDistro,
		choices: make(map[string]string),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	if m.isMenuStep() {
		return m.updateMenu(msg)
	}
	return m.updateInput(msg)
}

func (m model) isMenuStep() bool {
	switch m.step {
	case stepHostname, stepExtraPkgs, stepMirror,
		stepBrandingName, stepBrandingShort, stepBrandingUrl,
		stepBrandingSupport, stepBrandingVersion, stepOutputPath:
		return false
	}
	return true
}

func (m model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	opts := m.getMenuOptions()
	m.options = opts

	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(opts)-1 {
				m.cursor++
			}
		case "enter":
			if len(opts) > 0 && m.cursor < len(opts) {
				m.selectOption(opts[m.cursor])
			}
		}
	}

	return m, nil
}

func (m model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "enter":
			m.commitInput()
			return m, nil
		case "backspace":
			if m.cursorPos > 0 {
				m.textInput = m.textInput[:m.cursorPos-1] + m.textInput[m.cursorPos:]
				m.cursorPos--
			}
		case "left":
			if m.cursorPos > 0 {
				m.cursorPos--
			}
		case "right":
			if m.cursorPos < len(m.textInput) {
				m.cursorPos++
			}
		default:
			if len(km.String()) == 1 || km.String() == " " {
				ch := km.String()
				m.textInput = m.textInput[:m.cursorPos] + ch + m.textInput[m.cursorPos:]
				m.cursorPos += len(ch)
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w < 20 {
		w = 80
	}
	h := m.height
	if h < 10 {
		h = 24
	}

	base := lipgloss.NewStyle()
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	title := lipgloss.NewStyle().Bold(true).Underline(true)
	highlight := lipgloss.NewStyle().Bold(true).Reverse(true).Padding(0, 1)
	dim := lipgloss.NewStyle()
	sectionTitle := lipgloss.NewStyle().Bold(true)

	stepLabel, stepDesc := m.getStepInfo()
	summaryText := m.buildSummary()

	header := title.Render("Kagami ISO Builder")
	divWidth := w - 8
	if divWidth < 10 {
		divWidth = 40
	}
	if divWidth > 50 {
		divWidth = 50
	}
	divider := strings.Repeat("─", divWidth)
	stepLine := sectionTitle.Render(stepLabel)

	var body string
	if m.isMenuStep() {
		opts := m.getMenuOptions()
		var lines []string
		for i, opt := range opts {
			label := fmt.Sprintf("  %s", opt.text)
			if opt.desc != "" {
				label += fmt.Sprintf("  (%s)", opt.desc)
			}
			if i == m.cursor {
				lines = append(lines, highlight.Render(label))
			} else {
				lines = append(lines, dim.Render(label))
			}
		}
		body = strings.Join(lines, "\n")
	} else {
		prompt := m.getInputPrompt()
		display := m.textInput
		if m.cursorPos < len(display) {
			display = display[:m.cursorPos] + "█" + display[m.cursorPos+1:]
		} else {
			display = display + "█"
		}
		body = fmt.Sprintf("%s\n\n  > %s", prompt, display)
	}

	var sidebar string
	if summaryText != "" {
		sideBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Width(30)
		sidebar = sideBox.Render(sectionTitle.Render("Current Config") + "\n" + summaryText)
	}

	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		divider,
		"",
		stepLine,
		stepDesc,
		"",
		body,
		"",
		divider,
		"Up/Down: Navigate  Enter: Select  Ctrl+C: Quit",
	)

	var page string
	if sidebar != "" {
		mainBox := base.Width(max(40, w-38)).Render(mainContent)
		page = lipgloss.JoinHorizontal(lipgloss.Top, mainBox, "  ", sidebar)
	} else {
		page = mainContent
	}

	framed := border.Render(page)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, framed)
}

func (m model) getStepInfo() (string, string) {
	switch m.step {
	case stepDistro:
		return "[ Distribution ]", "  Select the base distribution for the ISO"
	case stepRelease:
		return "[ Release ]", "  Select the release codename"
	case stepMode:
		return "[ Build Mode ]", "  Choose between full desktop or minimal installer"
	case stepWM:
		return "[ Window Manager ]", "  Select WM for the minimal installer"
	case stepDesktop:
		return "[ Desktop Environment ]", "  Select the desktop environment"
	case stepInstaller:
		return "[ Installer ]", "  Select the system installer"
	case stepSlideshow:
		return "[ Slideshow ]", "  Select installer slideshow theme"
	case stepHostname:
		return "[ Hostname ]", "  Enter system hostname"
	case stepArch:
		return "[ Architecture ]", "  Select target CPU architecture"
	case stepKernel:
		return "[ Kernel ]", "  Select the kernel variant"
	case stepExtraPkgs:
		return "[ Extra Packages ]", "  Comma-separated list (press Enter to skip)"
	case stepMirror:
		return "[ Mirror ]", "  APT repository mirror URL"
	case stepBrandingName:
		return "[ Branding: Product Name ]", "  Full product name for Calamares"
	case stepBrandingShort:
		return "[ Branding: Short Name ]", "  Abbreviated product name"
	case stepBrandingUrl:
		return "[ Branding: Product URL ]", "  Main website URL"
	case stepBrandingSupport:
		return "[ Branding: Support URL ]", "  User support page URL"
	case stepBrandingVersion:
		return "[ Branding: Version ]", "  Distribution version string"
	case stepFlatpak:
		return "[ Flatpak ]", "  Enable or skip Flatpak runtime"
	case stepSnapd:
		return "[ Snapd ]", "  Permanent snapd suppression"
	case stepFirewall:
		return "[ Firewall ]", "  Enable UFW firewall"
	case stepOutputPath:
		return "[ Output ]", "  Path for the configuration JSON file"
	case stepConfirm:
		return "[ Confirm ]", "  Finalize and proceed"
	case stepLogChoice:
		return "[ Build Logs ]", "  Choose how to display build progress"
	}
	return "", ""
}

func (m model) getMenuOptions() []menuOption {
	switch m.step {
	case stepDistro:
		return []menuOption{
			{"ubuntu", "Ubuntu", "Ubuntu-based ISO"},
			{"debian", "Debian", "Debian-based ISO"},
		}
	case stepRelease:
		if m.choices["distro"] == "debian" {
			return []menuOption{
				{"stable", "Stable", "Current"},
				{"testing", "Testing", "Rolling"},
				{"unstable", "Unstable", "Sid"},
				{"bookworm", "Bookworm", "Debian 12"},
				{"trixie", "Trixie", "Debian 13"},
			}
		}
		return []menuOption{
			{"noble", "Noble Numbat", "24.04 LTS"},
			{"resolute", "Resolute Rambutan", "26.04 LTS"},
			{"jammy", "Jammy Jellyfish", "22.04 LTS"},
			{"devel", "Development", "Rolling"},
		}
	case stepMode:
		return []menuOption{
			{"desktop", "Desktop ISO", "Full desktop"},
			{"minimal", "Minimal Installer", "Lightweight"},
		}
	case stepWM:
		return []menuOption{
			{"openbox", "Openbox", "Stacking WM"},
			{"dwm", "dwm", "Dynamic WM"},
			{"xfce4-minimal", "Xfce4 Minimal", "Xfce panel only"},
		}
	case stepDesktop:
		if m.choices["distro"] == "debian" {
			return []menuOption{
				{"xfce", "Xfce", "task-xfce-desktop"},
				{"gnome", "GNOME", "task-gnome-desktop"},
				{"kde", "KDE Plasma", "task-kde-desktop"},
				{"lxqt", "LXQt", "task-lxqt-desktop"},
				{"mate", "MATE", "task-mate-desktop"},
				{"lxde", "LXDE", "task-lxde-desktop"},
				{"none", "None", "Manual selection"},
			}
		}
		return []menuOption{
			{"gnome", "GNOME", "Ubuntu desktop"},
			{"kde", "KDE Plasma", "Kubuntu"},
			{"xfce", "Xfce", "Xubuntu"},
			{"mate", "MATE", "Ubuntu MATE"},
			{"lxqt", "LXQt", "Lubuntu"},
			{"lxde", "LXDE", "Lubuntu classic"},
			{"none", "None", "Manual selection"},
		}
	case stepInstaller:
		return []menuOption{
			{"ubiquity", "Ubiquity", "Legacy Ubuntu"},
			{"calamares", "Calamares", "Universal"},
		}
	case stepSlideshow:
		return []menuOption{
			{"ubuntu", "Ubuntu", "Standard"},
			{"kubuntu", "Kubuntu", "Plasma"},
			{"xubuntu", "Xubuntu", "Xfce"},
			{"lubuntu", "Lubuntu", "LXQt"},
			{"ubuntu-mate", "Ubuntu MATE", "MATE"},
		}
	case stepArch:
		return []menuOption{
			{"amd64", "amd64", "64-bit x86"},
			{"arm64", "arm64", "64-bit ARM"},
		}
	case stepKernel:
		if m.choices["distro"] == "debian" {
			return []menuOption{
				{"linux-image-amd64", "Standard", "Default kernel"},
				{"linux-image-rt-amd64", "Real-time", "RT kernel"},
				{"linux-image-cloud-amd64", "Cloud", "Cloud optimised"},
			}
		}
		return []menuOption{
			{"linux-generic", "Generic", "Default kernel"},
			{"linux-lowlatency", "Low-latency", "Pro audio"},
			{"linux-oem-24.04", "OEM", "Vendor kernel"},
		}
	case stepFlatpak:
		return []menuOption{
			{"y", "Yes", "Enable"},
			{"n", "No", "Skip"},
		}
	case stepSnapd:
		return []menuOption{
			{"y", "Yes", "Suppress"},
			{"n", "No", "Allow"},
		}
	case stepFirewall:
		return []menuOption{
			{"n", "No", "Skip UFW"},
			{"y", "Yes", "Enable UFW"},
		}
	case stepConfirm:
		return []menuOption{
			{"y", "Proceed", "Go to log selection"},
			{"n", "Cancel", "Discard"},
		}
	case stepLogChoice:
		return []menuOption{
			{"terminal", "Terminal (Classic)", "Traditional scrolling log"},
			{"tui", "TUI (Visual)", "Compact progress view"},
		}
	}
	return nil
}

func (m *model) selectOption(opt menuOption) {
	switch m.step {
	case stepDistro:
		m.choices["distro"] = opt.key
		m.cursor = 0
		m.step = stepRelease
	case stepRelease:
		m.choices["release"] = opt.key
		m.cursor = 0
		m.step = stepMode
	case stepMode:
		m.choices["mode"] = opt.key
		m.cursor = 0
		if opt.key == "minimal" {
			m.step = stepWM
		} else {
			m.step = stepDesktop
		}
	case stepWM:
		m.choices["wm"] = opt.key
		m.choices["desktop"] = "none"
		m.cursor = 0
		m.choices["installer"] = "calamares"
		m.step = stepHostname
		m.initInput()
	case stepDesktop:
		m.choices["desktop"] = opt.key
		m.cursor = 0
		if m.choices["distro"] == "debian" {
			m.choices["installer"] = "calamares"
			m.step = stepHostname
			m.initInput()
		} else {
			m.step = stepInstaller
		}
	case stepInstaller:
		m.choices["installer"] = opt.key
		m.cursor = 0
		if opt.key == "ubiquity" {
			m.step = stepSlideshow
		} else {
			m.step = stepHostname
			m.initInput()
		}
	case stepSlideshow:
		m.choices["slideshow"] = opt.key
		m.cursor = 0
		m.step = stepHostname
		m.initInput()
	case stepArch:
		m.choices["arch"] = opt.key
		m.cursor = 0
		m.step = stepKernel
	case stepKernel:
		m.choices["kernel"] = opt.key
		m.cursor = 0
		m.step = stepExtraPkgs
		m.initInput()
	case stepFlatpak:
		m.choices["flatpak"] = opt.key
		m.cursor = 0
		if m.choices["distro"] == "ubuntu" {
			m.step = stepSnapd
		} else {
			m.step = stepFirewall
		}
	case stepSnapd:
		m.choices["snapd"] = opt.key
		m.cursor = 0
		m.step = stepFirewall
	case stepFirewall:
		m.choices["firewall"] = opt.key
		m.cursor = 0
		m.step = stepOutputPath
		m.initInput()
	case stepConfirm:
		if opt.key == "y" {
			m.step = stepLogChoice
			m.cursor = 0
		} else {
			m.quitting = true
		}
	case stepLogChoice:
		m.choices["log_mode"] = opt.key
		m.confirmed = true
		m.quitting = true
	}
}

func (m *model) initInput() {
	m.textInput = m.getInputDefault()
	m.cursorPos = len(m.textInput)
}

func (m model) getInputDefault() string {
	switch m.step {
	case stepHostname:
		def := m.choices["distro"]
		if m.choices["mode"] == "minimal" {
			def += "-minimal"
		} else if m.choices["distro"] != "debian" && m.choices["desktop"] != "none" {
			def += "-" + m.choices["desktop"]
		} else {
			def += "-desktop"
		}
		return def
	case stepExtraPkgs:
		return ""
	case stepMirror:
		if m.choices["distro"] == "debian" {
			return "http://deb.debian.org/debian/"
		}
		return "http://archive.ubuntu.com/ubuntu/"
	case stepBrandingName:
		if m.choices["distro"] == "debian" {
			return "Debian GNU/Linux"
		}
		return "Ubuntu"
	case stepBrandingShort:
		if m.choices["distro"] == "debian" {
			return "Debian"
		}
		return "Ubuntu"
	case stepBrandingUrl:
		if m.choices["distro"] == "debian" {
			return "https://www.debian.org"
		}
		return "https://ubuntu.com"
	case stepBrandingSupport:
		base := m.choices["branding_url"]
		if base == "" {
			if m.choices["distro"] == "debian" {
				base = "https://www.debian.org"
			} else {
				base = "https://ubuntu.com"
			}
		}
		return base + "/support"
	case stepBrandingVersion:
		return m.choices["release"]
	case stepOutputPath:
		return "kagami.json"
	}
	return ""
}

func (m model) getInputPrompt() string {
	switch m.step {
	case stepHostname:
		return "Enter system hostname:"
	case stepExtraPkgs:
		return "Additional packages (comma-separated, Enter to skip):"
	case stepMirror:
		return "APT repository mirror URL:"
	case stepBrandingName:
		return "Calamares product name:"
	case stepBrandingShort:
		return "Short product name:"
	case stepBrandingUrl:
		return "Product URL:"
	case stepBrandingSupport:
		return "Support URL:"
	case stepBrandingVersion:
		return "Version:"
	case stepOutputPath:
		return "Configuration file path:"
	}
	return "Input:"
}

func (m *model) commitInput() {
	val := m.textInput
	switch m.step {
	case stepHostname:
		if val == "" {
			val = m.getInputDefault()
		}
		m.choices["hostname"] = val
		m.step = stepArch
		m.cursor = 0
	case stepExtraPkgs:
		if val != "" {
			for _, p := range strings.Split(val, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					m.additionalPkgs = append(m.additionalPkgs, p)
				}
			}
		}
		m.step = stepMirror
		m.initInput()
	case stepMirror:
		if val == "" {
			val = m.getInputDefault()
		}
		m.choices["mirror"] = val
		if m.choices["installer"] == "calamares" {
			m.step = stepBrandingName
		} else {
			m.step = stepFlatpak
			m.cursor = 0
		}
		m.initInput()
	case stepBrandingName:
		if val == "" {
			val = m.getInputDefault()
		}
		m.choices["branding_name"] = val
		m.step = stepBrandingShort
		m.initInput()
	case stepBrandingShort:
		if val == "" {
			val = m.getInputDefault()
		}
		m.choices["branding_short"] = val
		m.step = stepBrandingUrl
		m.initInput()
	case stepBrandingUrl:
		if val == "" {
			val = m.getInputDefault()
		}
		m.choices["branding_url"] = val
		m.step = stepBrandingSupport
		m.initInput()
	case stepBrandingSupport:
		if val == "" {
			val = m.getInputDefault()
		}
		m.choices["branding_support"] = val
		m.step = stepBrandingVersion
		m.initInput()
	case stepBrandingVersion:
		if val == "" {
			val = m.getInputDefault()
		}
		m.choices["branding_version"] = val
		m.step = stepFlatpak
		m.cursor = 0
	case stepOutputPath:
		if val == "" {
			val = "kagami.json"
		}
		m.outputPath = val
		m.step = stepConfirm
		m.cursor = 0
	}
	m.textInput = ""
	m.cursorPos = 0
	if !m.isMenuStep() {
		m.initInput()
	}
}

func (m model) buildSummary() string {
	var lines []string
	if v, ok := m.choices["distro"]; ok {
		lines = append(lines, fmt.Sprintf("Distro:    %s", v))
	}
	if v, ok := m.choices["release"]; ok {
		lines = append(lines, fmt.Sprintf("Release:   %s", v))
	}
	if v, ok := m.choices["mode"]; ok {
		lines = append(lines, fmt.Sprintf("Mode:      %s", v))
	}
	if v, ok := m.choices["desktop"]; ok && v != "" {
		lines = append(lines, fmt.Sprintf("Desktop:   %s", v))
	}
	if v, ok := m.choices["wm"]; ok && v != "" {
		lines = append(lines, fmt.Sprintf("WM:        %s", v))
	}
	if v, ok := m.choices["installer"]; ok {
		lines = append(lines, fmt.Sprintf("Installer: %s", v))
	}
	if v, ok := m.choices["hostname"]; ok {
		lines = append(lines, fmt.Sprintf("Hostname:  %s", v))
	}
	if v, ok := m.choices["arch"]; ok {
		lines = append(lines, fmt.Sprintf("Arch:      %s", v))
	}
	if v, ok := m.choices["kernel"]; ok {
		lines = append(lines, fmt.Sprintf("Kernel:    %s", v))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func Run() (*config.Config, string, string, error) {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return nil, "", "", err
	}

	final := m.(model)
	if final.quitting && !final.confirmed {
		os.Exit(0)
	}

	cfg := final.buildConfig()
	if final.outputPath != "" {
		if saveErr := cfg.SaveToFile(final.outputPath); saveErr != nil {
			return nil, "", "", saveErr
		}
	}

	return cfg, final.outputPath, final.choices["log_mode"], nil
}

func (m model) buildConfig() *config.Config {
	return &config.Config{
		Distro:  m.choices["distro"],
		Release: m.choices["release"],
		System: config.SystemConfig{
			Hostname:     m.choices["hostname"],
			BlockSnapd:   m.choices["snapd"] == "y",
			Architecture: m.choices["arch"],
			Locale:       "en_US.UTF-8",
			Timezone:     "UTC",
		},
		Repository: config.RepositoryConfig{
			Mirror: m.choices["mirror"],
		},
		Packages: config.PackageConfig{
			Desktop:       m.choices["desktop"],
			Additional:    m.additionalPkgs,
			Kernel:        m.choices["kernel"],
			EnableFlatpak: m.choices["flatpak"] == "y",
			WM:            m.choices["wm"],
		},
		Installer: config.InstallerConfig{
			Type:      m.choices["installer"],
			Slideshow: m.choices["slideshow"],
			Branding: config.BrandingConfig{
				ProductName:      m.choices["branding_name"],
				ShortProductName: m.choices["branding_short"],
				ProductUrl:       m.choices["branding_url"],
				SupportUrl:       m.choices["branding_support"],
				Version:          m.choices["branding_version"],
			},
		},
		Security: config.SecurityConfig{
			EnableFirewall: m.choices["firewall"] == "y",
		},
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
