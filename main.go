package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	target string
	input  string
	start  time.Time
	end    time.Time
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.finished() && msg.Type != tea.KeyEsc && msg.Type != tea.KeyCtrlC {
			return m, nil
		}
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyBackspace:
			r := []rune(m.input)
			if len(r) > 0 {
				m.input = string(r[:len(r)-1])
			}
		case tea.KeySpace:
			m = m.addChars(" ")
		case tea.KeyRunes:
			m = m.addChars(string(msg.Runes))
		}
	}
	return m, nil
}

func (m model) addChars(s string) model {
	if m.start.IsZero() {
		m.start = time.Now()
	}
	target := []rune(m.target)
	in := []rune(m.input)
	for _, r := range s {
		if len(in) >= len(target) {
			break
		}
		in = append(in, r)
	}
	m.input = string(in)
	if len(in) == len(target) && m.end.IsZero() {
		m.end = time.Now()
	}
	return m
}

func (m model) finished() bool {
	return !m.end.IsZero()
}

func (m model) correctChars() int {
	target := []rune(m.target)
	input := []rune(m.input)
	n := 0
	for i := range input {
		if i < len(target) && input[i] == target[i] {
			n++
		}
	}
	return n
}

func (m model) wpm() float64 {
	mintues := m.end.Sub(m.start).Minutes()
	if mintues == 0 {
		return 0
	}
	return float64(m.correctChars()) / 5 / mintues
}

func (m model) accuracy() float64 {
	total := len([]rune(m.target))
	if total == 0 {
		return 0
	}
	return float64(m.correctChars()) / float64(total) * 100
}

var (
	correctStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	wrongStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Underline(true)
	wrongSpace   = lipgloss.NewStyle().Background(lipgloss.Color("9"))
	cursorStyle  = lipgloss.NewStyle().Reverse(true)
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statsStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
)

func (m model) View() string {
	target := []rune(m.target)
	input := []rune(m.input)

	var b strings.Builder
	for i, t := range target {
		ch := string(t)
		switch {
		case i < len(input) && input[i] == t:
			b.WriteString(correctStyle.Render(ch))
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
	if m.finished() {
		b.WriteString(statsStyle.Render(fmt.Sprintf("wpm: %.0f acc: %.1f%%", m.wpm(), m.accuracy())))
		b.WriteString("\n\n")
	}
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
