package cli

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/stasfilin/nvim-sandbox/internal/app"
)

var availableRuntimes = app.AvailableRuntimes

const defaultPublishedPorts = "3000:3000, 8080:80"

type choice struct {
	label string
	value string
}

type wizardStep int

const (
	stepRuntime wizardStep = iota
	stepExisting
	stepSource
	stepImage
	stepNeovimInstall
	stepNeovim
	stepEditorToolsInstall
	stepEditorTools
	stepCustomEditorTools
	stepPackages
	stepCustomPackages
	stepInstallCommand
	stepNetwork
	stepPorts
	stepMount
	stepPluginInstall
	stepPluginManager
	stepPluginCustomCommand
	stepConnect
	stepStopOnExit
	stepReview
	stepDone
)

type wizardModel struct {
	cfg                           app.Config
	ctx                           app.Context
	existing                      *app.Metadata
	runtimes                      []app.RuntimeInfo
	hasDocker                     bool
	step                          wizardStep
	cursor                        int
	input                         string
	inputDefaultActive            bool
	err                           error
	cancelled                     bool
	result                        app.CreateOptions
	networkSet                    bool
	defaultInstallCmd             string
	defaultInstallArgs            string
	installField                  int
	installCommandInput           string
	installArgumentsInput         string
	installCommandDefaultActive   bool
	installArgumentsDefaultActive bool
	width                         int
	height                        int
	packageSelections             map[string]bool
	editorSelections              map[string]bool
	multiFilter                   string
	darkBackground                bool
	pathDisplay                   string
	inputError                    string
}

type wizardStyles struct {
	accent        lipgloss.Style
	title         lipgloss.Style
	muted         lipgloss.Style
	body          lipgloss.Style
	help          lipgloss.Style
	selected      lipgloss.Style
	placeholder   lipgloss.Style
	panel         lipgloss.Style
	input         lipgloss.Style
	inputInactive lipgloss.Style
	summary       lipgloss.Style
	errorText     lipgloss.Style
}

func newWizardStyles(dark bool) wizardStyles {
	lightDark := lipgloss.LightDark(dark)
	accent := lightDark(lipgloss.Color("#0369a1"), lipgloss.Color("#7dd3fc"))
	title := lightDark(lipgloss.Color("#111827"), lipgloss.Color("#f8fafc"))
	muted := lightDark(lipgloss.Color("#475569"), lipgloss.Color("#94a3b8"))
	body := lightDark(lipgloss.Color("#1f2937"), lipgloss.Color("#e2e8f0"))
	help := lightDark(lipgloss.Color("#334155"), lipgloss.Color("#cbd5e1"))
	selected := lightDark(lipgloss.Color("#86198f"), lipgloss.Color("#f0abfc"))
	placeholder := lightDark(lipgloss.Color("#5b21b6"), lipgloss.Color("#ddd6fe"))
	border := lightDark(lipgloss.Color("#64748b"), lipgloss.Color("#94a3b8"))
	inputBorder := lightDark(lipgloss.Color("#7e22ce"), lipgloss.Color("#c084fc"))
	errorColor := lightDark(lipgloss.Color("#b91c1c"), lipgloss.Color("#fca5a5"))

	return wizardStyles{
		accent:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		title:       lipgloss.NewStyle().Foreground(title).Bold(true),
		muted:       lipgloss.NewStyle().Foreground(muted),
		body:        lipgloss.NewStyle().Foreground(body),
		help:        lipgloss.NewStyle().Foreground(help),
		selected:    lipgloss.NewStyle().Foreground(selected).Bold(true),
		placeholder: lipgloss.NewStyle().Foreground(placeholder).Bold(true),
		panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1, 3),
		input: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(inputBorder).
			Padding(0, 1),
		inputInactive: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(border).
			Padding(0, 1),
		summary: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(border).
			PaddingLeft(2),
		errorText: lipgloss.NewStyle().Foreground(errorColor),
	}
}

func (m wizardModel) styles() wizardStyles {
	return newWizardStyles(m.darkBackground)
}

func runWizard(cfg app.Config, ctx app.Context, existing *app.Metadata, pathDisplay string) (app.CreateOptions, bool, error) {
	model, err := newWizardModelWithPath(cfg, ctx, existing, pathDisplay)
	if err != nil {
		return app.CreateOptions{}, false, err
	}
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return app.CreateOptions{}, false, err
	}
	final := finalModel.(wizardModel)
	if final.cancelled {
		return app.CreateOptions{}, false, nil
	}
	return final.result, true, final.err
}

func newWizardModel(cfg app.Config, ctx app.Context, existing *app.Metadata) (wizardModel, error) {
	return newWizardModelWithPath(cfg, ctx, existing, "short")
}

func newWizardModelWithPath(cfg app.Config, ctx app.Context, existing *app.Metadata, pathDisplay string) (wizardModel, error) {
	model := wizardModel{
		cfg:       cfg,
		ctx:       ctx,
		existing:  existing,
		runtimes:  availableRuntimes(),
		hasDocker: app.HasDockerfile(ctx.ProjectRoot, cfg),
		result: app.CreateOptions{
			Source: "default-image",
		},
		defaultInstallCmd: app.DetectInstallCommand(cfg.Image),
		darkBackground:    true,
		pathDisplay:       pathDisplay,
	}
	if model.defaultInstallCmd == "" {
		model.defaultInstallCmd = cfg.DefaultImage.InstallCommand
	}
	model.defaultInstallArgs = app.DefaultInstallArguments(model.defaultInstallCmd)
	if model.defaultInstallCmd == cfg.DefaultImage.InstallCommand && cfg.DefaultImage.InstallArguments != "" {
		model.defaultInstallArgs = cfg.DefaultImage.InstallArguments
	}
	if len(model.runtimes) == 0 {
		return wizardModel{}, &app.Error{Kind: "no-runtime", Message: "No container runtime found.\n\nInstall Apple Containers, Docker, or Podman.", Code: 1}
	}
	return model, nil
}

func (m wizardModel) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if background, ok := msg.(tea.BackgroundColorMsg); ok {
		m.darkBackground = background.IsDark()
		return m, nil
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
		m.height = size.Height
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if !m.isInputStep() {
		m.clampCursor()
	}
	switch key.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		if m.isMultiStep() && m.multiFilter != "" {
			m.multiFilter = ""
			m.clampCursor()
			return m, nil
		}
		m.cancelled = true
		return m, tea.Quit
	case "q":
		if m.isMultiStep() && m.multiFilter != "" {
			return m.updateMulti(key)
		}
		if !m.isInputStep() {
			m.cancelled = true
			return m, tea.Quit
		}
	case "enter":
		if m.step == stepReview {
			m.step = stepDone
			return m, tea.Quit
		}
	}
	if m.isInputStep() {
		if m.step == stepInstallCommand {
			return m.updateInstallInput(key)
		}
		return m.updateInput(key)
	}
	if m.isMultiStep() {
		return m.updateMulti(key)
	}
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.choices())-1 {
			m.cursor++
		}
	case "enter":
		m.applyChoice()
	}
	return m, nil
}

func (m wizardModel) View() tea.View {
	if m.cancelled || m.step == stepDone {
		return tea.NewView("")
	}
	styles := m.styles()
	body := m.renderBody()
	panelWidth := m.panelWidth()
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.title.Render("nvim-sandbox"),
		styles.muted.Render(" > "),
		styles.accent.Render("create"),
	)
	project := styles.muted.Render(displayProjectPath(m.ctx.ProjectRoot, m.pathDisplay))
	content := lipgloss.JoinVertical(lipgloss.Left, header, project, "", body)
	panel := styles.panel.Width(panelWidth).Render(content)
	if m.width > panelWidth+8 {
		panel = lipgloss.NewStyle().MarginLeft(2).Render(panel)
	}
	return tea.NewView(panel + "\n")
}

func (m wizardModel) renderBody() string {
	if m.step == stepInstallCommand {
		return m.renderInstallInput()
	}
	if m.isInputStep() {
		return m.renderInput()
	}
	if m.isMultiStep() {
		return m.renderMulti()
	}
	if m.step == stepReview {
		return m.renderReview()
	}
	return m.renderChoices()
}

func (m wizardModel) renderMulti() string {
	styles := m.styles()
	question := styles.title.Render(fmt.Sprintf("%s (%d selected)", m.choicePrompt(), m.selectedCount()))
	filter := m.multiFilter
	if filter == "" {
		filter = "type to filter"
		filter = styles.placeholder.Render(filter)
	}
	rows := []string{question, styles.muted.Render("Filter: ") + styles.body.Render(filter), ""}
	choices := m.filteredChoices()
	if len(choices) == 0 {
		rows = append(rows, styles.muted.Render("No matches."))
	}
	for i, choice := range choices {
		checked := m.isSelected(choice.value)
		box := "[ ]"
		if checked {
			box = "[x]"
		}
		prefix := "  "
		label := styles.body.Render(box + " " + choice.label)
		if i == m.cursor {
			prefix = styles.accent.Render("› ")
			label = styles.selected.Render(box + " " + choice.label)
		}
		rows = append(rows, prefix+label)
	}
	rows = append(rows, "", styles.help.Render("type: filter  •  backspace: edit  •  ctrl+u: clear  •  space: toggle  •  enter: continue  •  esc: clear/quit"))
	return m.withSummary(strings.Join(rows, "\n"), false)
}

func (m wizardModel) renderChoices() string {
	styles := m.styles()
	question := styles.title.Render(m.choicePrompt())
	rows := []string{question}
	if m.step == stepNetwork {
		rows = append(rows, styles.muted.Render("Published ports require network access."))
	}
	rows = append(rows, "")
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
	return m.withSummary(strings.Join(rows, "\n"), false)
}

func (m wizardModel) renderInput() string {
	styles := m.styles()
	value := m.input
	if m.step == stepPorts && value == "" {
		value = styles.placeholder.Render(defaultPublishedPorts)
	} else if value == "" {
		value = " "
	}
	if m.inputDefaultActive {
		value = styles.placeholder.Render(value + "  (default)")
	}
	fieldWidth := minInt(48, maxInt(18, m.panelWidth()-18))
	input := styles.input.Width(fieldWidth).Render(value)
	lines := []string{
		styles.title.Render(m.inputPrompt()),
	}
	if m.step == stepPorts {
		lines = append(lines, styles.muted.Render("Map HOST:CONTAINER. Separate mappings with commas; leave blank for none."))
	}
	lines = append(lines,
		"",
		input,
	)
	if m.inputError != "" {
		lines = append(lines, "", styles.errorText.Render(m.inputError))
	}
	help := "type to replace default  •  enter: accept  •  esc: quit"
	if m.step == stepPorts {
		help = "example: 3000:3000, 8080:80  •  enter: continue  •  esc: quit"
	}
	lines = append(lines, "", styles.help.Render(help))
	return m.withSummary(strings.Join(lines, "\n"), false)
}

func (m wizardModel) renderInstallInput() string {
	styles := m.styles()
	fieldWidth := minInt(56, maxInt(24, m.panelWidth()-18))
	command := m.installCommandInput
	if command == "" {
		command = " "
	}
	arguments := m.installArgumentsInput
	if arguments == "" {
		arguments = "(none)"
	}
	if m.installCommandDefaultActive {
		command = styles.placeholder.Render(command + "  (default)")
	}
	if m.installArgumentsDefaultActive {
		arguments = styles.placeholder.Render(arguments + "  (default)")
	}
	commandStyle := styles.inputInactive
	argumentsStyle := styles.inputInactive
	commandLabel := styles.body.Render("Install command")
	argumentsLabel := styles.body.Render("Install arguments")
	if m.installField == 0 {
		commandStyle = styles.input
		commandLabel = styles.title.Render("Install command")
	} else {
		argumentsStyle = styles.input
		argumentsLabel = styles.title.Render("Install arguments")
	}
	lines := []string{
		styles.title.Render("Package installation"),
		"",
		commandLabel,
		commandStyle.Width(fieldWidth).Render(command),
		"",
		argumentsLabel,
		argumentsStyle.Width(fieldWidth).Render(arguments),
		"",
		styles.help.Render("tab, up/down: switch field  •  type: edit  •  enter: continue  •  esc: quit"),
	}
	return m.withSummary(strings.Join(lines, "\n"), false)
}

func (m wizardModel) renderReview() string {
	styles := m.styles()
	lines := []string{styles.title.Render("Create plan"), ""}
	summary := m.reviewLines()
	if len(summary) == 0 {
		lines = append(lines, styles.muted.Render("No options selected. Existing sandbox will be used."))
	} else {
		for _, line := range summary {
			lines = append(lines, formatSummaryLines(styles, line, maxInt(28, m.panelWidth()-8))...)
		}
	}
	lines = append(lines, "", styles.help.Render("enter: create sandbox  •  esc: quit"))
	return strings.Join(lines, "\n")
}

func (m wizardModel) withSummary(main string, full bool) string {
	styles := m.styles()
	summary := m.summaryLines(full)
	if len(summary) == 0 || m.width < 112 {
		return main
	}
	lines := []string{styles.muted.Render("Selected")}
	for _, line := range summary {
		lines = append(lines, formatSummaryLine(styles, line))
	}
	side := styles.summary.Width(30).Render(strings.Join(lines, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, main, strings.Repeat(" ", 4), side)
}

func formatSummaryLine(styles wizardStyles, line string) string {
	label, value, ok := strings.Cut(line, ": ")
	if !ok {
		return styles.body.Render(line)
	}
	return styles.muted.Render(label+": ") + styles.body.Render(value)
}

func formatSummaryLines(styles wizardStyles, line string, width int) []string {
	label, value, ok := strings.Cut(line, ": ")
	if !ok || width <= 0 {
		return []string{formatSummaryLine(styles, line)}
	}
	prefix := label + ": "
	wrapped := wrapWords(value, maxInt(12, width-len(prefix)))
	lines := make([]string, 0, len(wrapped))
	for index, part := range wrapped {
		if index == 0 {
			lines = append(lines, styles.muted.Render(prefix)+styles.body.Render(part))
			continue
		}
		lines = append(lines, strings.Repeat(" ", len(prefix))+styles.body.Render(part))
	}
	return lines
}

func wrapWords(value string, width int) []string {
	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	return append(lines, current)
}

func (m wizardModel) panelWidth() int {
	if m.width <= 0 {
		return 76
	}
	return minInt(maxInt(56, m.width-8), 104)
}

func (m wizardModel) choices() []choice {
	switch m.step {
	case stepExisting:
		return []choice{{label: "Destroy and recreate", value: "recreate"}, {label: "Use existing sandbox", value: "use-existing"}}
	case stepRuntime:
		choices := make([]choice, 0, len(m.runtimes))
		for _, runtime := range m.runtimes {
			choices = append(choices, choice{label: runtime.Label, value: runtime.Name})
		}
		return choices
	case stepSource:
		choices := []choice{{label: "Default image (" + m.cfg.Image + ")", value: "default-image"}}
		if m.hasDocker {
			choices = append(choices, choice{label: "Dockerfile (detected)", value: "dockerfile"})
		}
		choices = append(choices, choice{label: "Custom image", value: "custom"})
		return choices
	case stepEditorTools:
		return []choice{
			{label: "gopls", value: "gopls"},
			{label: "lua_ls", value: "lua_ls"},
			{label: "rust_analyzer", value: "rust_analyzer"},
			{label: "terraformls", value: "terraformls"},
			{label: "pyright", value: "pyright"},
			{label: "bash-language-server", value: "bash-language-server"},
			{label: "typescript-language-server", value: "typescript-language-server"},
			{label: "vscode-langservers-extracted", value: "vscode-langservers-extracted"},
			{label: "yaml-language-server", value: "yaml-language-server"},
			{label: "Custom npm tools...", value: "__custom__"},
		}
	case stepPackages:
		return []choice{
			{label: "git", value: "git"},
			{label: "ripgrep", value: "ripgrep"},
			{label: "fd-find", value: "fd-find"},
			{label: "curl (HTTP requests)", value: "curl"},
			{label: "ping (network diagnostics)", value: "ping"},
			{label: "python3", value: "python3"},
			{label: "python3-pip", value: "python3-pip"},
			{label: "build-essential", value: "build-essential"},
			{label: "Custom packages...", value: "__custom__"},
		}
	case stepNetwork:
		return []choice{
			{label: "Enable network (no host port mappings)", value: "enabled"},
			{label: "Enable network + publish host ports...", value: "ports"},
			{label: "Disable network (isolated)", value: "disabled"},
		}
	case stepPluginManager:
		return pluginManagerChoices()
	case stepNeovimInstall, stepEditorToolsInstall, stepMount, stepPluginInstall, stepConnect, stepStopOnExit:
		return []choice{{label: "Yes", value: "yes"}, {label: "No", value: "no"}}
	default:
		return nil
	}
}

func (m wizardModel) choicePrompt() string {
	switch m.step {
	case stepExisting:
		return "Sandbox already exists"
	case stepRuntime:
		return "Runtime"
	case stepSource:
		return "Source"
	case stepNetwork:
		return "Network access"
	case stepEditorToolsInstall:
		return "Install LSP tools?"
	case stepNeovimInstall:
		return "Install Neovim?"
	case stepEditorTools:
		return "LSP tools"
	case stepPackages:
		if m.result.Source == "dockerfile" {
			return "Dockerfile packages"
		}
		return "Extra packages"
	case stepMount:
		return "Mount local Neovim config?"
	case stepPluginInstall:
		return "Install Neovim plugins?"
	case stepPluginManager:
		return "Plugin manager"
	case stepConnect:
		return "Connect to nvim after creating?"
	case stepStopOnExit:
		return "Stop container when editor exits?"
	default:
		return ""
	}
}

func (m wizardModel) isInputStep() bool {
	switch m.step {
	case stepImage, stepNeovim, stepCustomEditorTools, stepCustomPackages, stepInstallCommand, stepPorts:
		return true
	case stepPluginCustomCommand:
		return true
	default:
		return false
	}
}

func (m wizardModel) inputPrompt() string {
	switch m.step {
	case stepImage:
		return "Image name [" + m.cfg.Image + "]"
	case stepNeovim:
		return "Neovim version (stable / nightly / vX.Y.Z)"
	case stepCustomPackages:
		if m.result.Source == "dockerfile" {
			return "Dockerfile custom packages"
		}
		return "Custom packages (comma separated)"
	case stepCustomEditorTools:
		return "Custom LSP tools (comma separated; npm package names)"
	case stepInstallCommand:
		return "Install command"
	case stepPorts:
		return "Publish ports to the host"
	case stepPluginCustomCommand:
		return "Install command"
	default:
		return ""
	}
}

func (m wizardModel) isMultiStep() bool {
	return m.step == stepEditorTools || m.step == stepPackages
}

func (m wizardModel) isSelected(value string) bool {
	switch m.step {
	case stepEditorTools:
		return m.editorSelections[value]
	case stepPackages:
		return m.packageSelections[value]
	default:
		return false
	}
}

func (m wizardModel) updateMulti(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.filteredChoices())-1 {
			m.cursor++
		}
	case "backspace", "ctrl+h":
		if len(m.multiFilter) > 0 {
			m.multiFilter = m.multiFilter[:len(m.multiFilter)-1]
			m.clampCursor()
		}
	case "ctrl+u":
		m.multiFilter = ""
		m.clampCursor()
	case " ", "space":
		choices := m.filteredChoices()
		if m.cursor >= 0 && m.cursor < len(choices) {
			m.toggleSelection(choices[m.cursor].value)
		}
	case "enter":
		m.applyMulti()
	default:
		if text := key.Key().Text; text != "" {
			m.multiFilter += text
			m.clampCursor()
		}
	}
	return m, nil
}

func (m wizardModel) filteredChoices() []choice {
	choices := m.choices()
	filter := strings.TrimSpace(strings.ToLower(m.multiFilter))
	if filter == "" {
		return choices
	}
	filtered := []choice{}
	for _, choice := range choices {
		if strings.Contains(strings.ToLower(choice.label), filter) || strings.Contains(strings.ToLower(choice.value), filter) {
			filtered = append(filtered, choice)
		}
	}
	return filtered
}

func (m wizardModel) selectedCount() int {
	count := 0
	switch m.step {
	case stepEditorTools:
		for _, selected := range m.editorSelections {
			if selected {
				count++
			}
		}
	case stepPackages:
		for _, selected := range m.packageSelections {
			if selected {
				count++
			}
		}
	}
	return count
}

func (m *wizardModel) toggleSelection(value string) {
	switch m.step {
	case stepEditorTools:
		if m.editorSelections == nil {
			m.initEditorSelections()
		}
		m.editorSelections[value] = !m.editorSelections[value]
	case stepPackages:
		if m.packageSelections == nil {
			m.initPackageSelections()
		}
		m.packageSelections[value] = !m.packageSelections[value]
	}
}

func (m wizardModel) updateInput(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.String() != "enter" {
		m.inputError = ""
	}
	switch key.String() {
	case "enter":
		m.applyInput()
	case "backspace", "ctrl+h":
		if m.inputDefaultActive {
			m.input = ""
			m.inputDefaultActive = false
			return m, nil
		}
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	default:
		if text := key.Key().Text; text != "" {
			if m.inputDefaultActive {
				m.input = text
				m.inputDefaultActive = false
			} else {
				m.input += text
			}
		}
	}
	return m, nil
}

func (m wizardModel) updateInstallInput(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "tab", "down":
		m.installField = 1
		return m, nil
	case "shift+tab", "up":
		m.installField = 0
		return m, nil
	case "enter":
		m.applyInstallInput()
		return m, nil
	case "backspace", "ctrl+h":
		if m.installField == 0 {
			if m.installCommandDefaultActive {
				m.installCommandInput = ""
				m.installCommandDefaultActive = false
			} else if len(m.installCommandInput) > 0 {
				m.installCommandInput = m.installCommandInput[:len(m.installCommandInput)-1]
			}
			m.syncInstallArgumentDefault()
		} else if m.installArgumentsDefaultActive {
			m.installArgumentsInput = ""
			m.installArgumentsDefaultActive = false
		} else if len(m.installArgumentsInput) > 0 {
			m.installArgumentsInput = m.installArgumentsInput[:len(m.installArgumentsInput)-1]
		}
		return m, nil
	}
	text := key.Key().Text
	if text == "" {
		return m, nil
	}
	if m.installField == 0 {
		if m.installCommandDefaultActive {
			m.installCommandInput = text
			m.installCommandDefaultActive = false
		} else {
			m.installCommandInput += text
		}
		m.syncInstallArgumentDefault()
	} else if m.installArgumentsDefaultActive {
		m.installArgumentsInput = text
		m.installArgumentsDefaultActive = false
	} else {
		m.installArgumentsInput += text
	}
	return m, nil
}

func (m *wizardModel) beginInstallInput() {
	m.refreshDefaultInstallCommand()
	m.installField = 0
	m.installCommandInput = m.defaultInstallCmd
	m.installArgumentsInput = m.defaultInstallArgs
	m.installCommandDefaultActive = true
	m.installArgumentsDefaultActive = true
	m.step = stepInstallCommand
}

func (m *wizardModel) syncInstallArgumentDefault() {
	if !m.installArgumentsDefaultActive {
		return
	}
	m.defaultInstallArgs = app.DefaultInstallArguments(strings.TrimSpace(m.installCommandInput))
	m.installArgumentsInput = m.defaultInstallArgs
}

func (m *wizardModel) applyInstallInput() {
	command := strings.TrimSpace(m.installCommandInput)
	if command == "" {
		command = m.defaultInstallCmd
	}
	arguments := strings.TrimSpace(m.installArgumentsInput)
	if m.installArgumentsDefaultActive {
		arguments = app.DefaultInstallArguments(command)
	}
	m.result.InstallCommand = command
	m.result.InstallArguments = arguments
	m.step = stepNetwork
	m.clampCursor()
}

func (m *wizardModel) applyChoice() {
	choices := m.choices()
	if m.cursor < 0 || m.cursor >= len(choices) {
		return
	}
	selected := choices[m.cursor]
	m.cursor = 0
	switch m.step {
	case stepRuntime:
		m.result.Runtime = selected.value
		if m.existing != nil {
			m.step = stepExisting
		} else {
			m.step = stepSource
		}
	case stepExisting:
		if selected.value == "use-existing" {
			m.result.ForceRecreate = false
			m.step = stepReview
			return
		}
		m.result.ForceRecreate = true
		m.step = stepSource
	case stepSource:
		if selected.value == "custom" {
			m.result.Source = "default-image"
			m.step = stepImage
		} else {
			m.result.Source = selected.value
			m.step = stepNeovimInstall
		}
	case stepNeovimInstall:
		if selected.value == "yes" {
			m.setInput("stable", true)
			m.step = stepNeovim
		} else {
			m.result.NeovimVersion = ""
			m.result.InstallPackages = removeValue(m.result.InstallPackages, "neovim")
			m.step = stepEditorToolsInstall
		}
	case stepEditorTools:
		m.applyMulti()
	case stepEditorToolsInstall:
		if selected.value == "yes" {
			m.initEditorSelections()
			m.step = stepEditorTools
		} else {
			m.result.EditorTools = nil
			m.result.InstallEditorTools = false
			m.initPackageSelections()
			m.step = stepPackages
		}
	case stepNetwork:
		network := app.Network{Enabled: selected.value != "disabled", Ports: []string{}}
		m.result.Network = &network
		m.networkSet = true
		if selected.value == "ports" {
			m.input = defaultPublishedPorts
			m.inputDefaultActive = true
			m.inputError = ""
			m.step = stepPorts
		} else {
			m.step = stepMount
		}
	case stepMount:
		m.result.AttachLocalVimConfig = selected.value == "yes"
		if m.result.AttachLocalVimConfig {
			m.step = stepPluginInstall
		} else {
			m.step = stepConnect
		}
	case stepPluginInstall:
		if selected.value == "yes" {
			m.step = stepPluginManager
		} else {
			m.step = stepConnect
		}
	case stepPluginManager:
		m.applyPluginChoice(selected.value, selected.label)
	case stepConnect:
		m.result.Connect = selected.value == "yes"
		m.step = stepStopOnExit
	case stepStopOnExit:
		stopOnExit := selected.value == "yes"
		m.result.StopOnExit = &stopOnExit
		m.step = stepReview
	case stepReview:
		m.step = stepDone
	}
	if m.step == stepReview {
		return
	}
	m.clampCursor()
}

func (m *wizardModel) applyInput() {
	value := strings.TrimSpace(m.input)
	if m.step != stepPorts {
		m.input = ""
	}
	switch m.step {
	case stepImage:
		if value != "" {
			m.result.Image = value
		}
		m.refreshDefaultInstallCommand()
		m.step = stepNeovimInstall
	case stepNeovim:
		if value == "" {
			value = "stable"
		}
		m.result.NeovimVersion = value
		if !contains(m.result.InstallPackages, "neovim") {
			m.result.InstallPackages = append([]string{"neovim"}, m.result.InstallPackages...)
		}
		m.step = stepEditorToolsInstall
	case stepCustomEditorTools:
		for _, tool := range splitList(value) {
			m.result.EditorTools = append(m.result.EditorTools, tool)
		}
		m.result.InstallEditorTools = len(m.result.EditorTools) > 0
		m.initPackageSelections()
		m.step = stepPackages
	case stepPackages:
		m.applyMulti()
	case stepCustomPackages:
		for _, pkg := range splitList(value) {
			if pkg != "neovim" && pkg != "nvim" {
				m.result.InstallPackages = append(m.result.InstallPackages, pkg)
			}
		}
		if len(m.result.InstallPackages) > 0 {
			m.beginInstallInput()
		} else {
			m.step = stepNetwork
		}
	case stepPorts:
		ports, err := parsePortMappings(value)
		if err != nil {
			m.inputError = err.Error()
			return
		}
		if m.result.Network != nil {
			m.result.Network.Ports = ports
		}
		m.input = ""
		m.inputError = ""
		m.step = stepMount
	case stepPluginCustomCommand:
		m.result.PluginInstallCommand = value
		m.result.PluginLabel = "custom"
		m.step = stepConnect
	}
	m.clampCursor()
}

func parsePortMappings(value string) ([]string, error) {
	ports := splitList(value)
	if len(ports) == 0 {
		return []string{}, nil
	}
	for _, mapping := range ports {
		parts := strings.Split(mapping, ":")
		if len(parts) != 2 || !validPort(parts[0]) || !validPort(parts[1]) {
			return nil, fmt.Errorf("invalid mapping %q; expected HOST:CONTAINER", mapping)
		}
	}
	return ports, nil
}

func validPort(value string) bool {
	port, err := strconv.Atoi(value)
	return err == nil && port >= 1 && port <= 65535
}

func (m *wizardModel) applyMulti() {
	switch m.step {
	case stepEditorTools:
		m.result.EditorTools = m.selectedEditorTools()
		m.result.InstallEditorTools = len(m.result.EditorTools) > 0
		if m.editorSelections["__custom__"] {
			m.setInput("", false)
			m.multiFilter = ""
			m.step = stepCustomEditorTools
			return
		}
		m.initPackageSelections()
		m.multiFilter = ""
		m.step = stepPackages
	case stepPackages:
		packages := []string{}
		if contains(m.result.InstallPackages, "neovim") {
			packages = append(packages, "neovim")
		}
		for _, choice := range m.choices() {
			if choice.value != "__custom__" && m.packageSelections[choice.value] {
				packages = append(packages, choice.value)
			}
		}
		m.result.InstallPackages = packages
		if m.packageSelections["__custom__"] {
			m.setInput("", false)
			m.multiFilter = ""
			m.step = stepCustomPackages
			return
		}
		m.multiFilter = ""
		if len(m.result.InstallPackages) > 0 {
			m.beginInstallInput()
		} else {
			m.step = stepNetwork
		}
	}
	m.clampCursor()
}

func (m *wizardModel) clampCursor() {
	choices := m.choices()
	if m.isMultiStep() {
		choices = m.filteredChoices()
	}
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

func (m *wizardModel) initEditorSelections() {
	if m.editorSelections != nil {
		return
	}
	m.editorSelections = map[string]bool{
		"gopls":                        false,
		"lua_ls":                       false,
		"rust_analyzer":                false,
		"terraformls":                  false,
		"pyright":                      false,
		"bash-language-server":         false,
		"typescript-language-server":   false,
		"vscode-langservers-extracted": false,
		"yaml-language-server":         false,
		"__custom__":                   false,
	}
}

func (m *wizardModel) initPackageSelections() {
	if m.packageSelections != nil {
		return
	}
	m.packageSelections = map[string]bool{
		"git":             true,
		"ripgrep":         true,
		"fd-find":         true,
		"curl":            true,
		"ping":            true,
		"python3":         true,
		"python3-pip":     true,
		"build-essential": true,
	}
}

func (m wizardModel) selectedEditorTools() []string {
	order := []string{"gopls", "lua_ls", "rust_analyzer", "terraformls", "pyright", "bash-language-server", "typescript-language-server", "vscode-langservers-extracted", "yaml-language-server"}
	selected := []string{}
	for _, tool := range order {
		if m.editorSelections[tool] {
			selected = append(selected, tool)
		}
	}
	return selected
}

func (m *wizardModel) setInput(value string, defaultActive bool) {
	m.input = value
	m.inputDefaultActive = defaultActive
}

func (m *wizardModel) refreshDefaultInstallCommand() {
	image := m.result.Image
	if image == "" {
		image = m.cfg.Image
	}
	if detected := app.DetectInstallCommand(image); detected != "" {
		m.defaultInstallCmd = detected
		m.defaultInstallArgs = app.DefaultInstallArguments(detected)
	}
}

func (m *wizardModel) applyPluginChoice(value string, label string) {
	if value == "custom" {
		m.setInput("", false)
		m.step = stepPluginCustomCommand
		return
	}
	m.result.PluginInstallCommand = value
	m.result.PluginLabel = strings.TrimSpace(strings.Split(label, "(")[0])
	if strings.HasPrefix(label, "Native pack") {
		m.result.AttachLocalNvimSite = true
	}
	m.step = stepConnect
}

func (m wizardModel) summaryLines(full bool) []string {
	lines := []string{}
	if m.result.Runtime != "" {
		lines = append(lines, "Runtime: "+fallback(runtimeDisplayName(m.result.Runtime), m.result.Runtime))
	}
	if m.result.Source != "" {
		lines = append(lines, "Source: "+m.sourceSummary())
	}
	if full || m.result.Image != "" {
		if m.result.Image != "" {
			lines = append(lines, "Image: "+m.result.Image)
		}
	}
	if m.result.NeovimVersion != "" && contains(m.result.InstallPackages, "neovim") {
		lines = append(lines, "Neovim: "+m.result.NeovimVersion)
	}
	if m.result.InstallCommand != "" {
		lines = append(lines, "Install: "+m.result.InstallCommand)
	}
	if m.result.InstallArguments != "" {
		lines = append(lines, "Arguments: "+m.result.InstallArguments)
	}
	extra := []string{}
	packages := app.NormalizeInstallPackages(m.result.InstallCommand, m.result.InstallPackages)
	for _, pkg := range packages {
		if pkg != "neovim" {
			extra = append(extra, pkg)
		}
	}
	if len(extra) > 0 {
		lines = append(lines, "Packages: "+strings.Join(extra, ", "))
	}
	if m.result.InstallEditorTools {
		tools := m.result.EditorTools
		if len(tools) == 0 {
			tools = []string{"gopls", "lua_ls", "rust_analyzer", "terraformls"}
		}
		lines = append(lines, "LSP tools: "+strings.Join(tools, ", "))
	}
	if m.result.Network != nil {
		network := "enabled"
		if !m.result.Network.Enabled {
			network = "disabled"
		}
		lines = append(lines, "Network: "+network)
		ports := "none"
		if !m.result.Network.Enabled {
			ports = "unavailable (network disabled)"
		} else if len(m.result.Network.Ports) > 0 {
			ports = strings.Join(m.result.Network.Ports, ", ")
		}
		lines = append(lines, "Published ports: "+ports)
	}
	if m.result.AttachLocalVimConfig {
		lines = append(lines, "Config: ~/.config/nvim (read-only)")
	}
	if m.result.PluginLabel != "" {
		lines = append(lines, "Plugins: "+m.result.PluginLabel)
	}
	if full && m.result.Connect {
		lines = append(lines, "Connect: yes")
	}
	if m.result.StopOnExit != nil {
		value := "no"
		if *m.result.StopOnExit {
			value = "yes"
		}
		lines = append(lines, "Stop on exit: "+value)
	}
	return lines
}

func (m wizardModel) reviewLines() []string {
	lines := []string{
		"Project: " + displayProjectPath(m.ctx.ProjectRoot, m.pathDisplay),
		"Container: " + fallback(m.ctx.ContainerName, "sandbox will be named from project"),
	}
	if m.result.Runtime != "" {
		lines = append(lines, "Runtime: "+fallback(runtimeDisplayName(m.result.Runtime), m.result.Runtime))
	}
	if m.result.Source != "" {
		lines = append(lines, "Source: "+m.sourceSummary())
	}
	if m.result.Image != "" {
		lines = append(lines, "Image: "+m.result.Image)
	}
	if m.result.NeovimVersion != "" && contains(m.result.InstallPackages, "neovim") {
		lines = append(lines, "Neovim: "+m.result.NeovimVersion)
	}
	if m.result.InstallCommand != "" {
		install := m.result.InstallCommand
		if m.result.InstallArguments != "" {
			install += " " + m.result.InstallArguments
		}
		lines = append(lines, "Package manager: "+install)
	}
	extra := []string{}
	for _, pkg := range app.NormalizeInstallPackages(m.result.InstallCommand, m.result.InstallPackages) {
		if pkg != "neovim" {
			extra = append(extra, pkg)
		}
	}
	if len(extra) > 0 {
		lines = append(lines, "Packages: "+strings.Join(extra, ", "))
	}
	if m.result.InstallEditorTools {
		tools := m.result.EditorTools
		if len(tools) == 0 {
			tools = []string{"gopls", "lua_ls", "rust_analyzer", "terraformls"}
		}
		lines = append(lines, "LSP tools: "+strings.Join(tools, ", "))
	}
	if m.result.Network != nil {
		network := "enabled"
		if !m.result.Network.Enabled {
			network = "disabled"
		}
		lines = append(lines, "Network: "+network)
		ports := "none"
		if !m.result.Network.Enabled {
			ports = "unavailable"
		} else if len(m.result.Network.Ports) > 0 {
			ports = strings.Join(m.result.Network.Ports, ", ")
		}
		lines = append(lines, "Published ports: "+ports)
	}
	mounts := []string{}
	if m.result.AttachLocalVimConfig {
		mounts = append(mounts, "~/.config/nvim, ~/.vim, ~/.vimrc")
	}
	if m.result.AttachLocalNvimSite {
		mounts = append(mounts, "~/.local/share/nvim/site")
	}
	if len(mounts) > 0 {
		lines = append(lines, "Read-only mounts: "+strings.Join(mounts, "; "))
	}
	if m.result.PluginLabel != "" {
		lines = append(lines, "Plugin install: "+m.result.PluginLabel)
	}
	if m.result.Connect {
		lines = append(lines, "After create: connect to nvim")
	} else {
		lines = append(lines, "After create: return to shell")
	}
	if m.result.StopOnExit != nil {
		if *m.result.StopOnExit {
			lines = append(lines, "Lifecycle: stop container when editor exits")
		} else {
			lines = append(lines, "Lifecycle: keep container running")
		}
	}
	return lines
}

func (m wizardModel) sourceSummary() string {
	if m.result.Image != "" {
		return "custom image"
	}
	switch m.result.Source {
	case "default-image":
		return "managed default image"
	case "dockerfile":
		return "project Dockerfile"
	default:
		return m.result.Source
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func removeValue(values []string, needle string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != needle {
			result = append(result, value)
		}
	}
	return result
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
