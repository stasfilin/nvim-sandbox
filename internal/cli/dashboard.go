package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/stasfilin/nvim-sandbox/internal/app"
)

type dashboardModel struct {
	status         app.Status
	cfg            app.Config
	stateBase      string
	update         updateInfo
	cursor         int
	width          int
	mode           string
	wizard         wizardModel
	cancelled      bool
	action         string
	create         app.CreateOptions
	err            error
	darkBackground bool
}

type dashboardResult struct {
	action string
	create app.CreateOptions
}

func runDashboard(cfg app.Config, status app.Status, stateBase string) (dashboardResult, bool, error) {
	model := dashboardModel{cfg: cfg, status: status, stateBase: stateBase, darkBackground: true}
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return dashboardResult{}, false, err
	}
	final := finalModel.(dashboardModel)
	if final.err != nil {
		return dashboardResult{}, false, final.err
	}
	if final.cancelled || final.action == "" {
		return dashboardResult{}, false, nil
	}
	return dashboardResult{action: final.action, create: final.create}, true, nil
}

func (m dashboardModel) Init() tea.Cmd {
	version := currentVersion().Version
	return tea.Batch(
		tea.RequestBackgroundColor,
		func() tea.Msg {
			return updateCheckMsg{info: checkForUpdate(version, m.stateBase)}
		},
	)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if update, ok := msg.(updateCheckMsg); ok {
		m.update = update.info
		return m, nil
	}
	if background, ok := msg.(tea.BackgroundColorMsg); ok {
		m.darkBackground = background.IsDark()
		m.wizard.darkBackground = m.darkBackground
		return m, nil
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
		if m.mode == "wizard" {
			m.wizard.width = size.Width
			m.wizard.height = size.Height
		}
		return m, nil
	}
	if m.mode == "wizard" {
		next, cmd := m.wizard.Update(msg)
		wizard := next.(wizardModel)
		m.wizard = wizard
		if wizard.cancelled {
			m.cancelled = true
			return m, tea.Quit
		}
		if wizard.step == stepDone {
			m.action = "create"
			m.create = wizard.result
			return m, tea.Quit
		}
		return m, cmd
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	m.clampCursor()
	switch key.String() {
	case "ctrl+c", "esc", "q":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.choices())-1 {
			m.cursor++
		}
	case "enter":
		choices := m.choices()
		if m.cursor >= 0 && m.cursor < len(choices) {
			return m.applyChoice(choices[m.cursor].value)
		}
	}
	return m, nil
}

func (m dashboardModel) applyChoice(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "create", "new":
		wizard, err := newWizardModel(m.cfg, m.status.Context, nil)
		if err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.mode = "wizard"
		wizard.darkBackground = m.darkBackground
		m.wizard = wizard
		return m, nil
	default:
		m.action = action
		return m, tea.Quit
	}
}

func (m dashboardModel) View() tea.View {
	if m.cancelled || m.action != "" {
		return tea.NewView("")
	}
	if m.mode == "wizard" {
		return m.wizard.View()
	}
	styles := newWizardStyles(m.darkBackground)
	panelWidth := m.panelWidth()
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.title.Render("nvim-sandbox"),
		styles.muted.Render(" > "),
		styles.accent.Render("dashboard"),
	)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		styles.muted.Render(m.status.Context.ProjectRoot),
		"",
		m.renderBody(),
	)
	panel := styles.panel.Width(panelWidth).Render(content)
	if m.width > panelWidth+8 {
		panel = lipgloss.NewStyle().MarginLeft(2).Render(panel)
	}
	return tea.NewView(panel + "\n")
}

func (m dashboardModel) renderBody() string {
	styles := newWizardStyles(m.darkBackground)
	rows := []string{styles.title.Render(m.prompt()), ""}
	for _, detail := range m.details() {
		rows = append(rows, dashboardDetailLine(styles, detail.label, detail.value))
	}
	rows = append(rows, "", styles.title.Render("Actions"), "")
	for i, choice := range m.choices() {
		prefix := "  "
		label := styles.body.Render(choice.label)
		if i == m.cursor {
			prefix = styles.accent.Render("› ")
			label = styles.selected.Render(choice.label)
		}
		rows = append(rows, prefix+label)
	}
	rows = append(rows, "", styles.help.Render("j/k, up/down: select  •  enter: choose  •  q, esc: quit"))
	return strings.Join(rows, "\n")
}

type dashboardDetail struct {
	label string
	value string
}

func (m dashboardModel) details() []dashboardDetail {
	version := currentVersion()
	versionValue := version.Version
	if version.Commit != "" {
		versionValue += " (" + version.Commit
		if version.Dirty {
			versionValue += "-dirty"
		}
		versionValue += ")"
	}
	state := fallback(m.status.Status, "not created")
	if state == "-" {
		state = "not created"
	}
	details := []dashboardDetail{
		{label: "CLI", value: versionValue},
	}
	if m.update.LatestVersion != "" {
		details = append(details, dashboardDetail{
			label: "Update",
			value: "v" + m.update.LatestVersion + " available — brew upgrade nvim-sandbox",
		})
	}
	if m.status.Metadata != nil {
		details = append(details,
			dashboardDetail{label: "Runtime", value: fallback(runtimeDisplayName(m.status.Runtime), "unknown")},
			dashboardDetail{label: "Dockerfile", value: yesNo(app.HasDockerfile(m.status.Context.ProjectRoot, m.cfg))},
			dashboardDetail{label: "Decision", value: fallback(m.status.Decision, "unknown")},
			dashboardDetail{label: "State", value: state},
			dashboardDetail{label: "Container", value: fallback(m.status.ContainerName, "-")},
			dashboardDetail{label: "Image", value: fallback(m.status.Image, "-")},
			dashboardDetail{label: "Stop on exit", value: yesNo(m.status.StopOnExit)},
		)
	} else {
		available := []string{}
		for _, runtime := range availableRuntimes() {
			available = append(available, runtime.Label)
		}
		availableText := "none found"
		if len(available) > 0 {
			availableText = strings.Join(available, ", ")
		}
		details = append(details,
			dashboardDetail{label: "Runtime", value: "choose during creation"},
			dashboardDetail{label: "Available", value: availableText},
			dashboardDetail{label: "Dockerfile", value: yesNo(app.HasDockerfile(m.status.Context.ProjectRoot, m.cfg))},
			dashboardDetail{label: "Decision", value: fallback(m.status.Decision, "unknown")},
			dashboardDetail{label: "State", value: state},
		)
	}
	return details
}

func dashboardDetailLine(styles wizardStyles, label string, value string) string {
	return styles.muted.Render(fmt.Sprintf("%-14s", label+":")) + styles.body.Render(value)
}

func runtimeDisplayName(runtimeName string) string {
	switch runtimeName {
	case "apple-container":
		return "Apple Container"
	case "docker":
		return "Docker"
	case "podman":
		return "Podman"
	default:
		return runtimeName
	}
}

func (m dashboardModel) prompt() string {
	if m.status.Metadata == nil {
		return "No sandbox exists"
	}
	return "Sandbox exists"
}

func (m dashboardModel) choices() []choice {
	if m.status.Metadata == nil {
		return []choice{
			{label: "Create sandbox", value: "create"},
			{label: "Cancel", value: "cancel"},
		}
	}
	return []choice{
		{label: "Connect to sandbox", value: "connect"},
		{label: "Recreate container (same image)", value: "recreate"},
		{label: "Destroy and create new", value: "new"},
		{label: "Cancel", value: "cancel"},
	}
}

func (m dashboardModel) panelWidth() int {
	if m.width <= 0 {
		return 76
	}
	return minInt(maxInt(56, m.width-8), 104)
}

func (m *dashboardModel) clampCursor() {
	choices := m.choices()
	if len(choices) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(choices) {
		m.cursor = len(choices) - 1
	}
}
