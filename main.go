package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	target string
	input  string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyBackspace:
			r := []rune(m.input)
			if len(r) > 0 {
				m.input = string(r[:len(r)-1])
			}
		case tea.KeySpace:
			m.input = m.addChars(m.input, " ")
		case tea.KeyRunes:
			m.input = m.addChars(m.input, string(msg.Runes))
		}
	}
	return m, nil
}

func (m model) addChars(input, s string) string {
	target := []rune(m.target)
	in := []rune(input)
	for _, r := range s {
		if len(in) >= len(target) {
			break
		}
		in = append(in, r)
	}
	return string(in)
}

var (
	correcStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	wrongStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Underline(true)
	wrongSpace   = lipgloss.NewStyle().Background(lipgloss.Color("9"))
	cursorStyle  = lipgloss.NewStyle().Reverse(true)
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func (m model) View() string {
	target := []rune(m.target)
	input := []rune(m.input)

	var b strings.Builder
	for i, t := range target {
		ch := string(t)
		switch {
		case i < len(input) && input[i] == t:
			b.WriteString(correcStyle.Render(ch))
		case i < len(input):
			if t == ' ' {
				b.WriteString(wrongSpace.Render(ch))
			} else {
				b.WriteString(wrongStyle.Render(ch))
			}
		case i == len(input):
			b.WriteString(cursorStyle.Render(ch))
		default:
			b.WriteString(pendingStyle.Render(ch))
		}
	}
	b.WriteString("\n\n")
	b.WriteString(pendingStyle.Render("esc to quit"))
	return b.String()
}

func main() {
	m := model{target: "the quick brown fox"}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
