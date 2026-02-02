package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	logoStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	itemStyle         = lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("12")).Bold(true)
)

const logo = `
                                                                                   
              ███████                   █████                   
            ███░░░░░███                ░░███                    
 ████████  ███     ░░███ ████████    ███████   ██████  ████████ 
░░███░░███░███      ░███░░███░░███  ███░░███  ███░░███░░███░░███
 ░███ ░███░███      ░███ ░███ ░███ ░███ ░███ ░███████  ░███ ░░░ 
 ░███ ░███░░███     ███  ░███ ░███ ░███ ░███ ░███░░░   ░███     
 ░███████  ░░░███████░   ████ █████░░████████░░██████  █████    
 ░███░░░     ░░░░░░░    ░░░░ ░░░░░  ░░░░░░░░  ░░░░░░  ░░░░░     
 ░███                                                           
 █████                                                          
░░░░░                                                           
`

type MenuModel struct {
	choices  []string
	cursor   int
	selected string
	quitting bool
}

func NewMenuModel() MenuModel {
	return MenuModel{
		choices: []string{"init", "web", "work", "list-features", "list-tasks", "status"},
	}
}

func (m MenuModel) Init() tea.Cmd {
	return nil
}

func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter":
			m.selected = m.choices[m.cursor]
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m MenuModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString(logoStyle.Render(logo))
	s.WriteString("\n\n")

	for i, choice := range m.choices {
		if m.cursor == i {
			s.WriteString(selectedItemStyle.Render(fmt.Sprintf("> %s", choice)))
		} else {
			s.WriteString(itemStyle.Render(fmt.Sprintf("  %s", choice)))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n(use arrow keys or j/k to navigate, enter to select, q to quit)\n")

	return s.String()
}

func (m MenuModel) Selected() string {
	return m.selected
}

func RunMenu() (string, error) {
	m := NewMenuModel()
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	return finalModel.(MenuModel).Selected(), nil
}
