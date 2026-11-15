package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// visibleLines is how many lines of the text are shown at once.
const visibleLines = 3

type testMode int

const (
	modeTime testMode = iota
	modeWords
	modeQuote
	modeCount // number of modes, used for cycling
)

func (t testMode) String() string {
	return [...]string{"time", "words", "quote"}[t]
}

var (
	timeOptions = []int{15, 30, 60}
	wordOptions = []int{10, 25, 50}
)

// settings is the test configuration. It survives restarts.
type settings struct {
	mode    testMode
	seconds int
	words   int
	punct   bool
	nums    bool
}

func (s settings) duration() time.Duration {
	return time.Duration(s.seconds) * time.Second
}

// text returns the text for a new test, or n more words in time mode.
func (s settings) text(n int) string {
	if s.mode == modeQuote {
		return randomQuote()
	}
	return generateText(n, s.punct, s.nums)
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

// savedMsg reports the outcome of writing history.json.
type savedMsg struct{ err error }

func saveCmd(history []result) tea.Cmd {
	return func() tea.Msg {
		return savedMsg{err: saveHistory(history)}
	}
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

	// samples holds the WPM typed during each second of the test.
	samples   []float64
	lastTyped int // typedChars at the previous sample

	history []result // all saved results, shared across restarts
	result  result   // this test's result, once finished
	summary summary  // result compared with history, once finished
	saveErr error
}

func (m model) Init() tea.Cmd {
	return nil
}

// restart returns a fresh test with the given settings, keeping the
// window size and history and invalidating any running tick chain.
func (m model) restart(cfg settings) model {
	nm := newModel(cfg, m.history)
	nm.width = m.width
	nm.height = m.height
	nm.id = m.id + 1
	nm.saveErr = m.saveErr
	return nm
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case savedMsg:
		m.saveErr = msg.err
		return m, nil
	case tickMsg:
		if msg.id != m.id || m.finished() {
			return m, nil
		}
		m = m.takeSample()
		if m.cfg.mode == modeTime && time.Since(m.start) >= m.cfg.duration() {
			return m.finish(m.start.Add(m.cfg.duration()))
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

		// Settings can only be changed before the test starts.
		if wasIdle {
			if cfg, ok := m.changeSettings(msg); ok {
				return m.restart(cfg), nil
			}
		}

		switch {
		case msg.Type == tea.KeyCtrlH || msg.Type == tea.KeyCtrlW ||
			(msg.Type == tea.KeyBackspace && msg.Alt):
			// Ctrl+Backspace (sent as ctrl+h by most terminals),
			// Ctrl+W or Alt+Backspace delete a whole word.
			m.input = deleteWord(m.input)
		case msg.Type == tea.KeyBackspace:
			r := []rune(m.input)
			if len(r) > 0 {
				m.input = string(r[:len(r)-1])
			}
		case msg.Type == tea.KeySpace:
			m = m.addChars(" ")
		case msg.Type == tea.KeyRunes:
			m = m.addChars(string(msg.Runes))
		}

		var cmds []tea.Cmd
		if wasIdle && !m.start.IsZero() {
			cmds = append(cmds, tick(m.id))
		}
		if !m.finished() && !m.start.IsZero() && m.typedChars() == len([]rune(m.target)) {
			var cmd tea.Cmd
			m, cmd = m.finish(time.Now())
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// changeSettings handles the keys that change the test before it starts.
// It reports false if msg is not one of them.
func (m model) changeSettings(msg tea.KeyMsg) (settings, bool) {
	cfg := m.cfg
	switch msg.Type {
	case tea.KeyDown:
		cfg.mode = (cfg.mode + 1) % modeCount
	case tea.KeyUp:
		cfg.mode = (cfg.mode + modeCount - 1) % modeCount
	case tea.KeyLeft, tea.KeyRight:
		dir := 1
		if msg.Type == tea.KeyLeft {
			dir = -1
		}
		switch cfg.mode {
		case modeTime:
			cfg.seconds = cycle(timeOptions, cfg.seconds, dir)
		case modeWords:
			cfg.words = cycle(wordOptions, cfg.words, dir)
		case modeQuote:
			// Quotes have no length to choose; show a different one.
		}
	case tea.KeyCtrlP:
		cfg.punct = !cfg.punct
	case tea.KeyCtrlN:
		cfg.nums = !cfg.nums
	default:
		return cfg, false
	}
	return cfg, true
}

// deleteWord removes the word before the cursor. If the cursor is right
// after a space, the space and the word before it are removed.
func deleteWord(s string) string {
	r := []rune(s)
	i := len(r)
	for i > 0 && r[i-1] == ' ' {
		i--
	}
	for i > 0 && r[i-1] != ' ' {
		i--
	}
	return string(r[:i])
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
			target = append(target, []rune(" "+m.cfg.text(25))...)
		}
		if len(in) >= len(target) {
			break
		}
		in = append(in, r)
	}
	m.target = string(target)
	m.input = string(in)
	return m
}

// takeSample records the WPM typed since the previous sample (one second).
func (m model) takeSample() model {
	typed := m.typedChars()
	perSecond := max(0, typed-m.lastTyped) // backspacing can make it negative
	m.samples = append(slices.Clip(m.samples), float64(perSecond)*60/5)
	m.lastTyped = typed
	return m
}

// finish ends the test, records the result and saves the history.
func (m model) finish(end time.Time) (model, tea.Cmd) {
	m.end = end
	m.result = result{
		Date:     end,
		Mode:     m.cfg.mode.String(),
		WPM:      m.wpm(),
		Raw:      m.rawWpm(),
		Accuracy: m.accuracy(),
	}
	switch m.cfg.mode {
	case modeTime:
		m.result.Length = m.cfg.seconds
	case modeWords:
		m.result.Length = m.cfg.words
	}
	if m.cfg.mode != modeQuote {
		m.result.Punct = m.cfg.punct
		m.result.Numbers = m.cfg.nums
	}
	m.summary = summarize(m.history, m.result)
	// Clip so the append copies instead of writing into a slice that an
	// older model (or the save in progress) might still be using.
	m.history = append(slices.Clip(m.history), m.result)
	return m, saveCmd(m.history)
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

// totalWords is the number of words in the text (words and quote mode).
func (m model) totalWords() int {
	return len(strings.Fields(m.target))
}

// wordsTyped counts completed words.
func (m model) wordsTyped() int {
	if m.finished() {
		return m.totalWords()
	}
	return strings.Count(m.input, " ")
}

func (m model) wrapWidth() int {
	if m.width <= 0 {
		return 60
	}
	return min(m.width, 60)
}

// pick renders s highlighted when on, dimmed otherwise.
func pick(s string, on bool) string {
	if on {
		return statsStyle.Render(s)
	}
	return pendingStyle.Render(s)
}

// modeBar shows the test settings before the test starts.
func (m model) modeBar() string {
	sep := pendingStyle.Render("│")
	var parts []string
	for mode := range modeCount {
		parts = append(parts, pick(mode.String(), mode == m.cfg.mode))
	}
	if m.cfg.mode == modeQuote {
		return strings.Join(parts, "  ")
	}

	parts = append(parts, sep)
	opts, cur := timeOptions, m.cfg.seconds
	if m.cfg.mode == modeWords {
		opts, cur = wordOptions, m.cfg.words
	}
	for _, o := range opts {
		parts = append(parts, pick(strconv.Itoa(o), o == cur))
	}
	if !slices.Contains(opts, cur) {
		// A custom length from a command-line flag.
		parts = append(parts, pick(strconv.Itoa(cur), true))
	}
	parts = append(parts, sep, pick("punctuation", m.cfg.punct), pick("numbers", m.cfg.nums))
	return strings.Join(parts, "  ")
}

// modeLabel describes the finished test, e.g. "time 30 · punctuation".
func (m model) modeLabel() string {
	label := m.cfg.mode.String()
	switch m.cfg.mode {
	case modeTime:
		label += " " + strconv.Itoa(m.cfg.seconds)
	case modeWords:
		label += " " + strconv.Itoa(m.cfg.words)
	}
	if m.cfg.mode != modeQuote {
		if m.cfg.punct {
			label += " · punctuation"
		}
		if m.cfg.nums {
			label += " · numbers"
		}
	}
	return pendingStyle.Render(label)
}

// statusLine is shown above the text.
func (m model) statusLine() string {
	switch {
	case m.start.IsZero():
		return m.modeBar()
	case m.finished():
		return m.modeLabel()
	}
	progress := strconv.Itoa(m.timeLeft())
	if m.cfg.mode != modeTime {
		progress = fmt.Sprintf("%d/%d", m.wordsTyped(), m.totalWords())
	}
	// Too little time has passed in the first second for WPM to mean anything.
	liveWpm := "-"
	if m.elapsed() >= time.Second {
		liveWpm = fmt.Sprintf("%.0f", m.wpm())
	}
	live := pendingStyle.Render(liveWpm + " wpm")
	return statsStyle.Render(progress) + "   " + live
}

// sparkline draws vals as a row of block characters, at most width wide.
func sparkline(vals []float64, width int) string {
	if len(vals) == 0 || width <= 0 {
		return ""
	}
	// Too many samples: average them into width buckets.
	if len(vals) > width {
		buckets := make([]float64, width)
		for i := range buckets {
			from := i * len(vals) / width
			to := (i + 1) * len(vals) / width
			sum := 0.0
			for _, v := range vals[from:to] {
				sum += v
			}
			buckets[i] = sum / float64(to-from)
		}
		vals = buckets
	}
	levels := []rune("▁▂▃▄▅▆▇█")
	top := slices.Max(vals)
	var b strings.Builder
	for _, v := range vals {
		i := 0
		if top > 0 {
			i = int(v/top*float64(len(levels)-1) + 0.5)
		}
		b.WriteRune(levels[i])
	}
	return b.String()
}

// results is shown below the text once the test is over.
func (m model) results() string {
	label := func(s string) string { return pendingStyle.Render(s) }
	num := func(f float64) string { return statsStyle.Render(fmt.Sprintf("%.0f", f)) }
	r := m.result
	wrong := m.typedChars() - m.correctChars()

	lines := []string{
		label("wpm ") + num(r.WPM) +
			"    " + label("acc ") + statsStyle.Render(fmt.Sprintf("%.1f%%", r.Accuracy)),
		label("raw ") + num(r.Raw) +
			"    " + label("chars ") + correctStyle.Render(strconv.Itoa(m.correctChars())) +
			label("/") + errorStyle.Render(strconv.Itoa(wrong)) +
			"    " + label("time ") + statsStyle.Render(fmt.Sprintf("%.1fs", m.elapsed().Seconds())),
	}

	s := m.summary
	switch {
	case s.newBest(r):
		lines = append(lines, correctStyle.Bold(true).Render("new best!")+
			label(fmt.Sprintf("  (previous %.0f)", s.prevBest)))
	case !s.hasPrev:
		lines = append(lines, label("first result for this mode"))
	default:
		lines = append(lines, label("best ")+num(s.prevBest)+
			"    "+label(fmt.Sprintf("avg of last %d ", s.avgCount))+num(s.avg))
	}
	if s.newBest(r) && s.avgCount > 1 {
		lines[len(lines)-1] += "    " + label(fmt.Sprintf("avg of last %d ", s.avgCount)) + num(s.avg)
	}

	if len(m.samples) >= 2 {
		lines = append(lines, statsStyle.Render(sparkline(m.samples, m.wrapWidth()-12))+
			label(fmt.Sprintf("  peak %.0f", slices.Max(m.samples))))
	}
	return strings.Join(lines, "\n")
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
	if m.saveErr != nil {
		out.WriteString(errorStyle.Render("could not save history: " + m.saveErr.Error()))
		out.WriteString("\n\n")
	}
	if m.start.IsZero() && m.cfg.mode == modeQuote {
		out.WriteString(pendingStyle.Render("←/→ new quote • ↑/↓ mode • tab restart • esc quit"))
	} else if m.start.IsZero() {
		out.WriteString(pendingStyle.Render("←/→ length • ↑/↓ mode • ctrl+p punctuation • ctrl+n numbers"))
		out.WriteString("\n")
		out.WriteString(pendingStyle.Render("tab restart • esc quit"))
	} else {
		out.WriteString(pendingStyle.Render("tab to restart • esc to quit"))
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, out.String())
}

func newModel(cfg settings, history []result) model {
	n := cfg.words
	if cfg.mode == modeTime {
		// Starting text; more is added while typing.
		n = 50
	}
	return model{cfg: cfg, target: cfg.text(n), history: history}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "termtype: "+format+"\n", args...)
	os.Exit(2)
}

func main() {
	seconds := flag.Int("t", 30, "test length in seconds (time mode)")
	words := flag.Int("w", 0, "number of words; switches to words mode")
	quote := flag.Bool("q", false, "type a quote instead of random words")
	punct := flag.Bool("p", false, "add punctuation and capital letters")
	nums := flag.Bool("n", false, "add numbers")
	themeName := flag.String("theme", "", "colour theme: "+themeNames())
	flag.Parse()

	if *seconds <= 0 || *words < 0 {
		fail("-t must be > 0 and -w must be >= 0")
	}
	if *quote && *words > 0 {
		fail("-q and -w can't be used together")
	}

	conf, err := loadConfig()
	if err != nil {
		fail("reading config: %v", err)
	}
	name := conf.Theme
	if *themeName != "" {
		name = *themeName
	}
	t, err := resolveTheme(name, conf.Colors)
	if err != nil {
		fail("%v", err)
	}
	applyTheme(t)

	history, err := loadHistory()
	if err != nil {
		// Don't start: saving would overwrite the unreadable file.
		fail("reading history: %v", err)
	}

	cfg := settings{mode: modeTime, seconds: *seconds, words: 25, punct: *punct, nums: *nums}
	switch {
	case *quote:
		cfg.mode = modeQuote
	case *words > 0:
		cfg.mode = modeWords
		cfg.words = *words
	}

	p := tea.NewProgram(newModel(cfg, history), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
