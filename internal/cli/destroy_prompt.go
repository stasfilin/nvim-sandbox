package cli

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/stasfilin/nvim-sandbox/internal/app"
)

type destroyPromptModel struct {
	status         app.Status
	pathDisplay    string
	cursor         int
	darkBackground bool
	done           bool
	confirmed      bool
}

func runDestroyPrompt(status app.Status, pathDisplay string) (bool, error) {
	model := destroyPromptModel{status: status, pathDisplay: pathDisplay, darkBackground: true}
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return false, err
	}
	return finalModel.(destroyPromptModel).confirmed, nil
}

func (m destroyPromptModel) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m destroyPromptModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.BackgroundColorMsg:
		m.darkBackground = message.IsDark()
	case tea.KeyPressMsg:
		switch message.String() {
		case "ctrl+c", "esc", "q", "n":
			m.done = true
			return m, tea.Quit
		case "y":
			m.done = true
			m.confirmed = true
			return m, tea.Quit
		case "up", "k":
			m.cursor = 0
		case "down", "j":
			m.cursor = 1
		case "enter":
			m.done = true
			m.confirmed = m.cursor == 1
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m destroyPromptModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	styles := newWizardStyles(m.darkBackground)
	rows := []string{
		styles.title.Render("Destroy this sandbox?"),
		styles.muted.Render(displayPath(m.status.Context.ProjectRoot, m.pathDisplay)),
	}
	if m.status.ContainerName != "" {
		rows = append(rows, styles.muted.Render("Container: "+m.status.ContainerName))
	}
	rows = append(rows, "")
	choices := []string{"No, keep sandbox", "Yes, destroy sandbox"}
	for index, choice := range choices {
		prefix := "  "
		label := styles.body.Render(choice)
		if index == m.cursor {
			prefix = styles.accent.Render("› ")
			label = styles.selected.Render(choice)
		}
		rows = append(rows, prefix+label)
	}
	rows = append(rows, "", styles.help.Render("enter: choose  •  y/n: choose  •  esc: cancel"))
	return tea.NewView(strings.Join(rows, "\n") + "\n")
}
