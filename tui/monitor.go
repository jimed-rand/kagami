package tui

import (
	"fmt"
	"kagami/pkg/builder"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type progressMsg struct {
	step  int
	total int
	name  string
}

type logMsg string

type doneMsg struct {
	err error
}

type monitorModel struct {
	builder     *builder.Builder
	currentStep int
	totalSteps  int
	stepName    string
	logs        []string
	err         error
	done        bool
	width       int
	height      int
}

func (m monitorModel) Init() tea.Cmd {
	return func() tea.Msg {
		m.builder.OnProgress = func(step, total int, name string) {
			// This is not safe to call m from here directly if we were using m.
			// But we are sending a message.
		}
		// We need a way to send messages from callbacks.
		// Bubble Tea program.Send() can do that.
		return nil
	}
}

// I need to change how Init() and Run works to support the callbacks.
// Actually, I can pass the program pointer to the callbacks.

func ShowBuild(b *builder.Builder) error {
	m := monitorModel{
		builder: b,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())

	b.OnProgress = func(step, total int, name string) {
		p.Send(progressMsg{step, total, name})
	}
	b.OnLog = func(msg string) {
		p.Send(logMsg(msg))
	}

	go func() {
		err := b.Build()
		p.Send(doneMsg{err})
	}()

	_, err := p.Run()
	return err
}

func (m monitorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case progressMsg:
		m.currentStep = msg.step
		m.totalSteps = msg.total
		m.stepName = msg.name
	case logMsg:
		m.logs = append(m.logs, string(msg))
		if len(m.logs) > 100 {
			m.logs = m.logs[1:]
		}
	case doneMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		m.done = true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m monitorModel) View() string {
	w := m.width
	if w < 20 {
		w = 80
	}
	h := m.height
	if h < 10 {
		h = 24
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	borderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	sectionTitle := lipgloss.NewStyle().Bold(true)
	progressStyle := lipgloss.NewStyle().Bold(true).Reverse(true)

	header := titleStyle.Render("Kagami Build Monitor")

	progWidth := w - 12
	if progWidth < 20 {
		progWidth = 20
	}
	if progWidth > 60 {
		progWidth = 60
	}

	percent := 0.0
	if m.totalSteps > 0 {
		percent = float64(m.currentStep) / float64(m.totalSteps)
	}

	filled := int(float64(progWidth) * percent)
	if filled > progWidth {
		filled = progWidth
	}
	empty := progWidth - filled

	bar := "[" + progressStyle.Render(strings.Repeat("█", filled)) + strings.Repeat(" ", empty) + "]"
	progressLine := fmt.Sprintf("%s  %d/%d - %s", bar, m.currentStep, m.totalSteps, m.stepName)

	logBoxHeight := h - 12
	if logBoxHeight < 5 {
		logBoxHeight = 5
	}

	var logLines []string
	start := len(m.logs) - logBoxHeight
	if start < 0 {
		start = 0
	}
	for i := start; i < len(m.logs); i++ {
		line := m.logs[i]
		if len(line) > w-10 {
			line = line[:w-13] + "..."
		}
		logLines = append(logLines, "  "+line)
	}
	logContent := strings.Join(logLines, "\n")

	view := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		sectionTitle.Render("Build Progress:"),
		progressLine,
		"",
		sectionTitle.Render("Activity Log:"),
		logContent,
		"",
		"Press Ctrl+C to abort (not recommended during build)",
	)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, borderStyle.Render(view))
}
