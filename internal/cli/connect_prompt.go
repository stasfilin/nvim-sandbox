package cli

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/stasfilin/nvim-sandbox/internal/app"
)

type connectPromptModel struct {
	ctx            app.Context
	cursor         int
	width          int
	darkBackground bool
	done           bool
	connect        bool
}

func runConnectPrompt(ctx app.Context) (bool, error) {
	model := connectPromptModel{ctx: ctx, darkBackground: true}
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return false, err
	}
	return finalModel.(connectPromptModel).connect, nil
}

func (m connectPromptModel) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m connectPromptModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.BackgroundColorMsg:
		m.darkBackground = message.IsDark()
		return m, nil
	case tea.WindowSizeMsg:
		m.width = message.Width
		return m, nil
	case tea.KeyPressMsg:
		switch message.String() {
		case "ctrl+c", "esc", "q", "n":
			m.done = true
			m.connect = false
			return m, tea.Quit
		case "y":
			m.done = true
			m.connect = true
			return m, tea.Quit
		case "up", "k":
			m.cursor = 0
		case "down", "j":
			m.cursor = 1
		case "enter":
			m.done = true
			m.connect = m.cursor == 0
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m connectPromptModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	styles := newWizardStyles(m.darkBackground)
	choices := []string{"Yes, connect to nvim", "No, return to shell"}
	rows := []string{
		styles.title.Render("Connect to nvim now?"),
		"",
	}
	for index, choice := range choices {
		prefix := "  "
		label := styles.body.Render(choice)
		if index == m.cursor {
			prefix = styles.accent.Render("› ")
			label = styles.selected.Render(choice)
		}
		rows = append(rows, prefix+label)
	}
	rows = append(rows, styles.help.Render("enter: choose  •  y/n: choose  •  esc: cancel"))
	return tea.NewView(strings.Join(rows, "\n") + "\n")
}
