package main

import (
	_ "embed"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

//go:embed words.txt
var wordsFile string

var words = strings.Fields(wordsFile)

func randomText(n int) string {
	picked := make([]string, n)
	for i := range picked {
		picked[i] = words[rand.IntN(len(words))]
	}
	return strings.Join(picked, " ")
}

// visibleLines is how many lines of the text are shown at once.
const visibleLines = 3

type testMode int

const (
	modeTime testMode = iota
	modeWords
)

var (
	timeOptions = []int{15, 30, 60}
	wordOptions = []int{10, 25, 50}
)

// settings is the test configuration. It survives restarts.
type settings struct {
	mode    testMode
	seconds int
	words   int
}

func (s settings) duration() time.Duration {
	return time.Duration(s.seconds) * time.Second
}

// cycle returns the option after (dir=1) or before (dir=-1) cur.
func cycle(opts []int, cur, dir int) int {
	i := slices.Index(opts, cur)
	if i < 0 {
		if dir > 0 {
			return opts[0]
		}
		return opts[len(opts)-1]
	}
	return opts[(i+dir+len(opts))%len(opts)]
}

type tickMsg struct{ id int }

func tick(id int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{id: id}
	})
}

type model struct {
	cfg    settings
	target string
	input  string
	start  time.Time
	end    time.Time
	width  int
	height int
	id     int
}

func (m model) Init() tea.Cmd {
	return nil
}

// restart returns a fresh test with the given settings, keeping the
// window size and invalidating any running tick chain.
func (m model) restart(cfg settings) model {
	nm := newModel(cfg)
	nm.width = m.width
	nm.height = m.height
	nm.id = m.id + 1
	return nm
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		if msg.id != m.id || m.finished() {
			return m, nil
		}
		if m.cfg.mode == modeTime && time.Since(m.start) >= m.cfg.duration() {
			m.end = m.start.Add(m.cfg.duration())
			return m, nil
		}
		return m, tick(m.id)
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyTab:
			return m.restart(m.cfg), nil
		}
		if m.finished() {
			return m, nil
		}
		wasIdle := m.start.IsZero()

		// Mode and length can only be changed before the test starts.
		if wasIdle {
			cfg := m.cfg
			switch msg.Type {
			case tea.KeyUp, tea.KeyDown:
				if cfg.mode == modeTime {
					cfg.mode = modeWords
				} else {
					cfg.mode = modeTime
				}
				return m.restart(cfg), nil
			case tea.KeyLeft, tea.KeyRight:
				dir := 1
				if msg.Type == tea.KeyLeft {
					dir = -1
				}
				if cfg.mode == modeTime {
					cfg.seconds = cycle(timeOptions, cfg.seconds, dir)
				} else {
					cfg.words = cycle(wordOptions, cfg.words, dir)
				}
				return m.restart(cfg), nil
			}
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
		if wasIdle && !m.start.IsZero() {
			return m, tick(m.id)
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
		// In time mode the text never runs out: add more words when
		// the cursor gets close to the end.
		if m.cfg.mode == modeTime && len(target)-len(in) < 100 {
			target = append(target, []rune(" "+randomText(25))...)
		}
		if len(in) >= len(target) {
			break
		}
		in = append(in, r)
	}
	m.target = string(target)
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

func (m model) typedChars() int {
	return len([]rune(m.input))
}

// elapsed is how long the test has been running (or ran, once finished).
func (m model) elapsed() time.Duration {
	switch {
	case m.start.IsZero():
		return 0
	case m.finished():
		return m.end.Sub(m.start)
	}
	return time.Since(m.start)
}

// wpm counts only correct characters (net WPM).
func (m model) wpm() float64 {
	minutes := m.elapsed().Minutes()
	if minutes == 0 {
		return 0
	}
	return float64(m.correctChars()) / 5 / minutes
}

// rawWpm counts every typed character, right or wrong.
func (m model) rawWpm() float64 {
	minutes := m.elapsed().Minutes()
	if minutes == 0 {
		return 0
	}
	return float64(m.typedChars()) / 5 / minutes
}

func (m model) accuracy() float64 {
	total := m.typedChars()
	if total == 0 {
		return 0
	}
	return float64(m.correctChars()) / float64(total) * 100
}

func (m model) timeLeft() int {
	if m.start.IsZero() {
		return m.cfg.seconds
	}
	remaining := m.cfg.duration() - time.Since(m.start)
	if remaining < 0 {
		return 0
	}
	return int((remaining + time.Second - 1) / time.Second)
}

// wordsTyped counts completed words in words mode.
func (m model) wordsTyped() int {
	if m.finished() {
		return m.cfg.words
	}
	return strings.Count(m.input, " ")
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
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)

// pick renders s highlighted when on, dimmed otherwise.
func pick(s string, on bool) string {
	if on {
		return statsStyle.Render(s)
	}
	return pendingStyle.Render(s)
}

// modeBar shows the mode and length choices before the test starts.
func (m model) modeBar() string {
	isTime := m.cfg.mode == modeTime
	parts := []string{pick("time", isTime), pick("words", !isTime), pendingStyle.Render("│")}
	opts, cur := timeOptions, m.cfg.seconds
	if !isTime {
		opts, cur = wordOptions, m.cfg.words
	}
	for _, o := range opts {
		parts = append(parts, pick(strconv.Itoa(o), o == cur))
	}
	if !slices.Contains(opts, cur) {
		// A custom length from a command-line flag.
		parts = append(parts, pick(strconv.Itoa(cur), true))
	}
	return strings.Join(parts, "  ")
}

// statusLine is shown above the text.
func (m model) statusLine() string {
	switch {
	case m.start.IsZero():
		return m.modeBar()
	case m.finished():
		if m.cfg.mode == modeTime {
			return pendingStyle.Render(fmt.Sprintf("time %d", m.cfg.seconds))
		}
		return pendingStyle.Render(fmt.Sprintf("words %d", m.cfg.words))
	}
	progress := fmt.Sprintf("%d", m.timeLeft())
	if m.cfg.mode == modeWords {
		progress = fmt.Sprintf("%d/%d", m.wordsTyped(), m.cfg.words)
	}
	// Too little time has passed in the first second for WPM to mean anything.
	liveWpm := "-"
	if m.elapsed() >= time.Second {
		liveWpm = fmt.Sprintf("%.0f", m.wpm())
	}
	live := pendingStyle.Render(liveWpm + " wpm")
	return statsStyle.Render(progress) + "   " + live
}

// results is shown below the text once the test is over.
func (m model) results() string {
	label := func(s string) string { return pendingStyle.Render(s) }
	wrong := m.typedChars() - m.correctChars()

	big := label("wpm ") + statsStyle.Render(fmt.Sprintf("%.0f", m.wpm())) +
		"    " + label("acc ") + statsStyle.Render(fmt.Sprintf("%.1f%%", m.accuracy()))
	small := label("raw ") + statsStyle.Render(fmt.Sprintf("%.0f", m.rawWpm())) +
		"    " + label("chars ") + correctStyle.Render(strconv.Itoa(m.correctChars())) +
		label("/") + errorStyle.Render(strconv.Itoa(wrong)) +
		"    " + label("time ") + statsStyle.Render(fmt.Sprintf("%.1fs", m.elapsed().Seconds()))
	return big + "\n" + small
}

func (m model) View() string {
	target := []rune(m.target)
	input := []rune(m.input)

	var lines []string
	var b strings.Builder
	maxWidth := m.wrapWidth()
	cursorLine := -1
	col := 0
	for i, t := range target {
		if i == 0 || target[i-1] == ' ' {
			wordLen := 0
			for j := i; j < len(target) && target[j] != ' '; j++ {
				wordLen++
			}
			if col > 0 && col+wordLen+1 > maxWidth {
				lines = append(lines, b.String())
				b.Reset()
				col = 0
			}
		}
		if i == len(input) {
			cursorLine = len(lines)
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
	lines = append(lines, b.String())
	if cursorLine < 0 {
		cursorLine = len(lines) - 1
	}

	// Show only a few lines: keep the cursor on the second line once
	// the first line is done, so upcoming text is always visible.
	start := max(0, cursorLine-1)
	start = min(start, max(0, len(lines)-visibleLines))
	end := min(len(lines), start+visibleLines)
	shown := lines[start:end]
	for len(shown) < visibleLines {
		shown = append(shown, "")
	}
	text := lipgloss.PlaceHorizontal(maxWidth, lipgloss.Left, strings.Join(shown, "\n"))

	var out strings.Builder
	out.WriteString(m.statusLine())
	out.WriteString("\n\n")
	out.WriteString(text)
	out.WriteString("\n\n")
	if m.finished() {
		out.WriteString(m.results())
		out.WriteString("\n\n")
	}
	if m.start.IsZero() {
		out.WriteString(pendingStyle.Render("←/→ length • ↑/↓ mode • tab restart • esc quit"))
	} else {
		out.WriteString(pendingStyle.Render("tab to restart • esc to quit"))
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, out.String())
}

func newModel(cfg settings) model {
	n := cfg.words
	if cfg.mode == modeTime {
		// Starting text; more is added while typing.
		n = 50
	}
	return model{cfg: cfg, target: randomText(n)}
}

func main() {
	seconds := flag.Int("t", 30, "test length in seconds (time mode)")
	words := flag.Int("w", 0, "number of words; switches to words mode")
	flag.Parse()

	if *seconds <= 0 || *words < 0 {
		fmt.Fprintln(os.Stderr, "termtype: -t must be > 0 and -w must be >= 0")
		os.Exit(2)
	}

	cfg := settings{mode: modeTime, seconds: *seconds, words: 25}
	if *words > 0 {
		cfg.mode = modeWords
		cfg.words = *words
	}

	p := tea.NewProgram(newModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
