package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var words = []string{
	"the", "be", "of", "and", "a", "to", "in", "he", "have", "it",
	"that", "for", "they", "with", "as", "not", "on", "she", "at", "by",
	"this", "we", "you", "do", "but", "from", "or", "which", "one", "would",
	"all", "will", "there", "say", "who", "make", "when", "can", "more", "if",
	"no", "man", "out", "other", "so", "what", "time", "up", "go", "about",
	"than", "into", "could", "state", "only", "new", "year", "some", "take", "come",
	"these", "know", "see", "use", "get", "like", "then", "first", "any", "work",
	"now", "may", "such", "give", "over", "think", "most", "even", "find", "day",
	"also", "after", "way", "many", "must", "look", "before", "great", "back", "through",
	"long", "where", "much", "should", "well", "people", "down", "own", "just", "because",
	"good", "each", "those", "feel", "seem", "how", "high", "too", "place", "little",
	"world", "very", "still", "nation", "hand", "old", "life", "tell", "write", "become",
	"here", "show", "house", "both", "between", "need", "mean", "call", "develop", "under",
	"last", "right", "move", "thing", "general", "school", "never", "same", "another", "begin",
	"while", "number", "part", "turn", "real", "leave", "might", "want", "point", "form",
	"off", "child", "few", "small", "since", "against", "ask", "late", "home", "interest",
}

func randomText(n int) string {
	picked := make([]string, n)
	for i := range picked {
		picked[i] = words[rand.IntN(len(words))]
	}
	return strings.Join(picked, " ")
}

type model struct {
	target string
	input  string
	start  time.Time
	end    time.Time
	width  int
	height int
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
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyTab:
			nm := newModel()
			nm.width = m.width
			nm.height = m.height
			return nm, nil
		}
		if m.finished() {
			return m, nil
		}

		switch msg.Type {
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
	minutes := m.end.Sub(m.start).Minutes()
	if minutes == 0 {
		return 0
	}
	return float64(m.correctChars()) / 5 / minutes
}

func (m model) accuracy() float64 {
	total := len([]rune(m.target))
	if total == 0 {
		return 0
	}
	return float64(m.correctChars()) / float64(total) * 100
}

func (m model) wrapWidth() int {
	if m.width <= 0 {
		return 60
	}
	return min(m.width, 60)
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
	maxWidth := m.wrapWidth()
	col := 0
	for i, t := range target {
		if i == 0 || target[i-1] == ' ' {
			wordLen := 0
			for j := i; j < len(target) && target[j] != ' '; j++ {
				wordLen++
			}
			if col > 0 && col+wordLen+1 > maxWidth {
				b.WriteString("\n")
				col = 0
			}
		}
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
		col++
	}
	text := lipgloss.PlaceHorizontal(maxWidth, lipgloss.Left, b.String())
	var out strings.Builder
	out.WriteString(text)
	out.WriteString("\n\n")
	if m.finished() {
		out.WriteString(statsStyle.Render(fmt.Sprintf("wpm: %.0f acc: %.1f%%", m.wpm(), m.accuracy())))
		out.WriteString("\n\n")
	}
	out.WriteString(pendingStyle.Render("tab to restart • esc to quit"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, out.String())
}

func newModel() model {
	return model{target: randomText(25)}
}

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
