package tui

import (
	"os"
	"strings"

	"kagami/pkg/config"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Bold(true)
	successStyle      = lipgloss.NewStyle().Bold(true)
)

type item struct {
	key, label, desc string
}

func (i item) Title() string       { return i.label }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.label }

type state int

const (
	stateDistro state = iota
	stateRelease
	stateMode
	stateWM
	stateDesktop
	stateInstaller
	stateSlideshow
	stateHostname
	stateArch
	stateKernel
	stateExtraPkgs
	stateMirror
	stateBrandingName
	stateBrandingShort
	stateBrandingUrl
	stateBrandingSupport
	stateBrandingVersion
	stateFlatpak
	stateSnapd
	stateFirewall
	stateOutputPath
	stateConfirm
	stateBuilding
	stateFinished
)

type Model struct {
	state          state
	list           list.Model
	input          textinput.Model
	choices        map[string]string
	cfg            *config.Config
	outputPath     string
	err            error
	width          int
	height         int
	additionalPkgs []string
}

func NewModel() Model {
	ti := textinput.New()
	ti.Focus()

	return Model{
		state:   stateDistro,
		choices: make(map[string]string),
		input:   ti,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	var cmd tea.Cmd
	switch m.state {
	case stateDistro:
		m, cmd = m.updateDistro(msg)
	case stateRelease:
		m, cmd = m.updateRelease(msg)
	case stateMode:
		m, cmd = m.updateMode(msg)
	case stateWM:
		m, cmd = m.updateWM(msg)
	case stateDesktop:
		m, cmd = m.updateDesktop(msg)
	case stateInstaller:
		m, cmd = m.updateInstaller(msg)
	case stateSlideshow:
		m, cmd = m.updateSlideshow(msg)
	case stateHostname:
		m, cmd = m.updateHostname(msg)
	case stateArch:
		m, cmd = m.updateArch(msg)
	case stateKernel:
		m, cmd = m.updateKernel(msg)
	case stateExtraPkgs:
		m, cmd = m.updateExtraPkgs(msg)
	case stateMirror:
		m, cmd = m.updateMirror(msg)
	case stateBrandingName:
		m, cmd = m.updateBrandingName(msg)
	case stateBrandingShort:
		m, cmd = m.updateBrandingShort(msg)
	case stateBrandingUrl:
		m, cmd = m.updateBrandingUrl(msg)
	case stateBrandingSupport:
		m, cmd = m.updateBrandingSupport(msg)
	case stateBrandingVersion:
		m, cmd = m.updateBrandingVersion(msg)
	case stateFlatpak:
		m, cmd = m.updateFlatpak(msg)
	case stateSnapd:
		m, cmd = m.updateSnapd(msg)
	case stateFirewall:
		m, cmd = m.updateFirewall(msg)
	case stateOutputPath:
		m, cmd = m.updateOutputPath(msg)
	case stateConfirm:
		m, cmd = m.updateConfirm(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	s := ""
	s += titleStyle.Render("Kagami ISO Builder") + "\n\n"

	switch m.state {
	case stateDistro:
		s += "Select base distribution:\n\n"
		s += m.list.View()
	case stateRelease:
		s += "Select release:\n\n"
		s += m.list.View()
	case stateMode:
		s += "Select build mode:\n\n"
		s += m.list.View()
	case stateWM:
		s += "Select window manager:\n\n"
		s += m.list.View()
	case stateDesktop:
		s += "Select desktop environment:\n\n"
		s += m.list.View()
	case stateInstaller:
		s += "Select installer:\n\n"
		s += m.list.View()
	case stateSlideshow:
		s += "Select installer slideshow:\n\n"
		s += m.list.View()
	case stateHostname:
		s += "System hostname:\n\n"
		s += m.input.View()
	case stateArch:
		s += "Target architecture:\n\n"
		s += m.list.View()
	case stateKernel:
		s += "Select kernel variant:\n\n"
		s += m.list.View()
	case stateExtraPkgs:
		s += "Additional packages (comma separated):\n\n"
		s += m.input.View()
	case stateMirror:
		s += "APT repository mirror:\n\n"
		s += m.input.View()
	case stateBrandingName:
		s += "Product name:\n\n"
		s += m.input.View()
	case stateBrandingShort:
		s += "Short product name:\n\n"
		s += m.input.View()
	case stateBrandingUrl:
		s += "Product URL:\n\n"
		s += m.input.View()
	case stateBrandingSupport:
		s += "Support URL:\n\n"
		s += m.input.View()
	case stateBrandingVersion:
		s += "Version:\n\n"
		s += m.input.View()
	case stateFlatpak:
		s += "Enable Flatpak support? (y/n)\n\n"
		s += m.list.View()
	case stateSnapd:
		s += "Apply permanent snapd suppression? (y/n)\n\n"
		s += m.list.View()
	case stateFirewall:
		s += "Enable UFW firewall? (y/n)\n\n"
		s += m.list.View()
	case stateOutputPath:
		s += "Output file path:\n\n"
		s += m.input.View()
	case stateConfirm:
		s += "Persist this configuration and proceed? (y/n)\n\n"
		s += m.list.View()
	}

	return s
}

func (m Model) updateDistro(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"ubuntu", "Ubuntu", "Ubuntu-based ISO synthesis"},
			item{"debian", "Debian", "Debian-based ISO synthesis"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Distribution"
		m.list.SetShowStatusBar(false)
		m.list.SetFilteringEnabled(false)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["distro"] = i.key
		m.state = stateRelease
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateRelease(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		var items []list.Item
		if m.choices["distro"] == "debian" {
			items = []list.Item{
				item{"stable", "Stable", "Debian Stable (current)"},
				item{"testing", "Testing", "Debian Testing (current)"},
				item{"unstable", "Unstable", "Debian Unstable / Sid"},
				item{"bookworm", "Bookworm", "Debian 12"},
				item{"trixie", "Trixie", "Debian 13"},
			}
		} else {
			items = []list.Item{
				item{"noble", "Noble Numbat", "Ubuntu 24.04 LTS"},
				item{"resolute", "Resolute Rambutan", "Ubuntu 26.04 LTS"},
				item{"jammy", "Jammy Jellyfish", "Ubuntu 22.04 LTS"},
				item{"devel", "Development", "Ubuntu Rolling Development"},
			}
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Release"
		m.list.SetShowStatusBar(false)
		m.list.SetFilteringEnabled(false)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["release"] = i.key
		m.state = stateMode
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateMode(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"desktop", "Desktop ISO", "Full desktop environment"},
			item{"minimal", "Minimal Installer", "Minimal live environment"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Build Mode"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["mode"] = i.key
		if i.key == "minimal" {
			m.state = stateWM
		} else {
			m.state = stateDesktop
		}
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateWM(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"openbox", "Openbox", "Lightweight stacking WM"},
			item{"dwm", "dwm", "Dynamic window manager"},
			item{"xfce4-minimal", "Xfce4 (Minimal)", "Xfce4 panel and desktop"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Window Manager"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["wm"] = i.key
		m.choices["desktop"] = "none"
		m.state = stateInstaller
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateDesktop(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		var items []list.Item
		if m.choices["distro"] == "debian" {
			items = []list.Item{
				item{"xfce", "Xfce", "task-xfce-desktop"},
				item{"gnome", "GNOME", "task-gnome-desktop"},
				item{"kde", "KDE Plasma", "task-kde-desktop"},
				item{"lxqt", "LXQt", "task-lxqt-desktop"},
				item{"mate", "MATE", "task-mate-desktop"},
				item{"lxde", "LXDE", "task-lxde-desktop"},
				item{"none", "None (Manual)", "Specify packages later"},
			}
		} else {
			items = []list.Item{
				item{"gnome", "GNOME", "Ubuntu GNOME"},
				item{"kde", "KDE Plasma", "Kubuntu"},
				item{"xfce", "Xfce", "Xubuntu"},
				item{"mate", "MATE", "Ubuntu MATE"},
				item{"lxqt", "LXQt", "Lubuntu (LXQt)"},
				item{"lxde", "LXDE", "Lubuntu (LXDE)"},
				item{"none", "None (Manual)", "Specify packages later"},
			}
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Desktop Environment"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["desktop"] = i.key
		m.state = stateInstaller
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateInstaller(msg tea.Msg) (Model, tea.Cmd) {
	if m.choices["mode"] == "minimal" || m.choices["distro"] == "debian" {
		m.choices["installer"] = "calamares"
		m.state = stateHostname
		return m, nil
	}

	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"ubiquity", "Ubiquity", "Traditional Ubuntu installer"},
			item{"calamares", "Calamares", "Universal installer framework"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Installer"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["installer"] = i.key
		if i.key == "ubiquity" {
			m.state = stateSlideshow
		} else {
			m.state = stateHostname
		}
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateSlideshow(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"ubuntu", "Ubuntu", "Standard slideshow"},
			item{"kubuntu", "Kubuntu", "KDE Plasma slideshow"},
			item{"xubuntu", "Xubuntu", "Xfce slideshow"},
			item{"lubuntu", "Lubuntu", "LXQt slideshow"},
			item{"ubuntu-mate", "Ubuntu MATE", "MATE slideshow"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Slideshow"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["slideshow"] = i.key
		m.state = stateHostname
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateHostname(msg tea.Msg) (Model, tea.Cmd) {
	if m.input.Value() == "" {
		def := m.choices["distro"]
		if m.choices["mode"] == "minimal" {
			def += "-minimal"
		} else if m.choices["distro"] != "debian" && m.choices["desktop"] != "none" {
			def += "-" + m.choices["desktop"]
		} else {
			def += "-desktop"
		}
		m.input.SetValue(def)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		m.choices["hostname"] = m.input.Value()
		m.input.SetValue("")
		m.state = stateArch
	}

	return m, cmd
}

func (m Model) updateArch(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"amd64", "amd64", "64-bit x86"},
			item{"arm64", "arm64", "64-bit ARM"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Architecture"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["arch"] = i.key
		m.state = stateKernel
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateKernel(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		var items []list.Item
		if m.choices["distro"] == "debian" {
			items = []list.Item{
				item{"linux-image-amd64", "Standard", "Standard Debian kernel"},
				item{"linux-image-rt-amd64", "Real-time", "Real-time kernel"},
				item{"linux-image-cloud-amd64", "Cloud", "Optimised for cloud"},
			}
		} else {
			items = []list.Item{
				item{"linux-generic", "Generic", "Standard Ubuntu kernel"},
				item{"linux-lowlatency", "Low-latency", "Reduced-latency kernel"},
				item{"linux-oem-24.04", "OEM", "OEM stabilised kernel"},
			}
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Kernel"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["kernel"] = i.key
		m.state = stateExtraPkgs
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateExtraPkgs(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		input := m.input.Value()
		if input != "" {
			for _, pkg := range strings.Split(input, ",") {
				pkg = strings.TrimSpace(pkg)
				if pkg != "" {
					m.additionalPkgs = append(m.additionalPkgs, pkg)
				}
			}
		}
		m.input.SetValue("")
		m.state = stateMirror
	}

	return m, cmd
}

func (m Model) updateMirror(msg tea.Msg) (Model, tea.Cmd) {
	if m.input.Value() == "" {
		if m.choices["distro"] == "debian" {
			m.input.SetValue("http://deb.debian.org/debian/")
		} else {
			m.input.SetValue("http://archive.ubuntu.com/ubuntu/")
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		m.choices["mirror"] = m.input.Value()
		m.input.SetValue("")
		if m.choices["installer"] == "calamares" {
			m.state = stateBrandingName
		} else {
			m.state = stateFlatpak
		}
	}

	return m, cmd
}

func (m Model) updateBrandingName(msg tea.Msg) (Model, tea.Cmd) {
	if m.input.Value() == "" {
		if m.choices["distro"] == "debian" {
			m.input.SetValue("Debian GNU/Linux")
		} else {
			m.input.SetValue("Ubuntu")
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		m.choices["branding_name"] = m.input.Value()
		m.input.SetValue("")
		m.state = stateBrandingShort
	}

	return m, cmd
}

func (m Model) updateBrandingShort(msg tea.Msg) (Model, tea.Cmd) {
	if m.input.Value() == "" {
		if m.choices["distro"] == "debian" {
			m.input.SetValue("Debian")
		} else {
			m.input.SetValue("Ubuntu")
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		m.choices["branding_short"] = m.input.Value()
		m.input.SetValue("")
		m.state = stateBrandingUrl
	}

	return m, cmd
}

func (m Model) updateBrandingUrl(msg tea.Msg) (Model, tea.Cmd) {
	if m.input.Value() == "" {
		if m.choices["distro"] == "debian" {
			m.input.SetValue("https://www.debian.org")
		} else {
			m.input.SetValue("https://ubuntu.com")
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		m.choices["branding_url"] = m.input.Value()
		m.input.SetValue("")
		m.state = stateBrandingSupport
	}

	return m, cmd
}

func (m Model) updateBrandingSupport(msg tea.Msg) (Model, tea.Cmd) {
	if m.input.Value() == "" {
		m.input.SetValue(m.choices["branding_url"] + "/support")
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		m.choices["branding_support"] = m.input.Value()
		m.input.SetValue("")
		m.state = stateBrandingVersion
	}

	return m, cmd
}

func (m Model) updateBrandingVersion(msg tea.Msg) (Model, tea.Cmd) {
	if m.input.Value() == "" {
		m.input.SetValue(m.choices["release"])
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		m.choices["branding_version"] = m.input.Value()
		m.input.SetValue("")
		m.state = stateFlatpak
	}

	return m, cmd
}

func (m Model) updateFlatpak(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"y", "Yes", "Enable Flatpak support"},
			item{"n", "No", "Disable Flatpak support"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Flatpak Support"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["flatpak"] = i.key
		if m.choices["distro"] == "ubuntu" {
			m.state = stateSnapd
		} else {
			m.state = stateFirewall
		}
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateSnapd(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"y", "Yes", "Apply permanent snapd suppression"},
			item{"n", "No", "Allow snapd"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Snapd Suppression"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["snapd"] = i.key
		m.state = stateFirewall
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateFirewall(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"y", "Yes", "Enable UFW firewall"},
			item{"n", "No", "Disable UFW firewall"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Firewall"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		m.choices["firewall"] = i.key
		m.state = stateOutputPath
		m.list.SetItems(nil)
	}

	return m, cmd
}

func (m Model) updateOutputPath(msg tea.Msg) (Model, tea.Cmd) {
	if m.input.Value() == "" {
		m.input.SetValue("kagami.json")
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		m.outputPath = m.input.Value()
		m.input.SetValue("")
		m.state = stateConfirm
	}

	return m, cmd
}

func (m Model) updateConfirm(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.list.Items()) == 0 {
		items := []list.Item{
			item{"y", "Yes", "Start building ISO"},
			item{"n", "No", "Exit without building"},
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Confirm"
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
		i := m.list.SelectedItem().(item)
		if i.key == "y" {
			return m, tea.Quit
		}
		os.Exit(0)
	}

	return m, cmd
}

func Run() (*config.Config, string, error) {
	p := tea.NewProgram(NewModel())
	m, err := p.Run()
	if err != nil {
		return nil, "", err
	}

	finalModel := m.(Model)
	cfg := finalModel.buildConfig()
	return cfg, finalModel.outputPath, nil
}

func (m Model) buildConfig() *config.Config {
	branding := config.BrandingConfig{
		ProductName:      m.choices["branding_name"],
		ShortProductName: m.choices["branding_short"],
		ProductUrl:       m.choices["branding_url"],
		SupportUrl:       m.choices["branding_support"],
		Version:          m.choices["branding_version"],
	}

	cfg := &config.Config{
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
			Branding:  branding,
		},
		Security: config.SecurityConfig{
			EnableFirewall: m.choices["firewall"] == "y",
		},
	}

	return cfg
}
