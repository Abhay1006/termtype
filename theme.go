package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// theme holds the colours used by the UI. Each value is an ANSI colour
// number ("10") or a hex colour ("#a3be8c").
type theme struct {
	Correct string `json:"correct,omitempty"`
	Wrong   string `json:"wrong,omitempty"`
	Pending string `json:"pending,omitempty"`
	Accent  string `json:"accent,omitempty"`
}

var themes = map[string]theme{
	"default": {Correct: "10", Wrong: "9", Pending: "8", Accent: "12"},
	"ocean":   {Correct: "#7fdbca", Wrong: "#ff5874", Pending: "#5f7e97", Accent: "#82aaff"},
	"warm":    {Correct: "#b8bb26", Wrong: "#fb4934", Pending: "#7c6f64", Accent: "#fabd2f"},
	"forest":  {Correct: "#a7c080", Wrong: "#e67e80", Pending: "#7a8478", Accent: "#83c092"},
	"mono":    {Correct: "255", Wrong: "203", Pending: "243", Accent: "255"},
}

func themeNames() string {
	return strings.Join(slices.Sorted(maps.Keys(themes)), ", ")
}

// config is the contents of config.json. Every field is optional.
//
//	{
//	  "theme": "ocean",
//	  "colors": { "accent": "#ffcc00" },
//	  "mode": "time",
//	  "seconds": 30,
//	  "words": 25,
//	  "punctuation": false,
//	  "numbers": false
//	}
type config struct {
	Theme   string `json:"theme,omitempty"`
	Colors  theme  `json:"colors,omitzero"` // overrides single colours of the theme
	Mode    string `json:"mode,omitempty"`
	Seconds int    `json:"seconds,omitempty"`
	Words   int    `json:"words,omitempty"`
	Punct   bool   `json:"punctuation,omitempty"`
	Numbers bool   `json:"numbers,omitempty"`
}

var configMu sync.Mutex

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// loadConfig reads config.json. A missing file gives the defaults.
func loadConfig() (config, error) {
	var c config
	path, err := configPath()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// toSettings returns the saved test settings, falling back to defaults for
// any unset or invalid fields.
func (c config) toSettings() settings {
	s := defaultSettings()
	switch c.Mode {
	case "time":
		s.mode = modeTime
	case "words":
		s.mode = modeWords
	case "quote":
		s.mode = modeQuote
	}
	if c.Seconds > 0 {
		s.seconds = c.Seconds
	}
	if c.Words > 0 {
		s.words = c.Words
	}
	s.punct = c.Punct
	s.nums = c.Numbers
	return s
}

// saveSettings persists s into config.json while preserving existing theme
// and colour settings.
func saveSettings(s settings) error {
	if s.mode == modeCustom {
		return nil
	}
	configMu.Lock()
	defer configMu.Unlock()

	c, err := loadConfig()
	if err != nil {
		return err
	}
	c.Mode = s.mode.String()
	c.Seconds = s.seconds
	c.Words = s.words
	c.Punct = s.punct
	c.Numbers = s.nums

	path, err := configPath()
	if err != nil {
		return err
	}
	return writeJSON(path, c)
}

// resolveTheme picks the named theme (default if empty) and applies the
// colour overrides on top of it.
func resolveTheme(name string, overrides theme) (theme, error) {
	if name == "" {
		name = "default"
	}
	t, ok := themes[name]
	if !ok {
		return t, fmt.Errorf("unknown theme %q (available: %s)", name, themeNames())
	}
	if overrides.Correct != "" {
		t.Correct = overrides.Correct
	}
	if overrides.Wrong != "" {
		t.Wrong = overrides.Wrong
	}
	if overrides.Pending != "" {
		t.Pending = overrides.Pending
	}
	if overrides.Accent != "" {
		t.Accent = overrides.Accent
	}
	return t, nil
}

var (
	correctStyle lipgloss.Style
	wrongStyle   lipgloss.Style
	wrongSpace   lipgloss.Style
	cursorStyle  lipgloss.Style
	pendingStyle lipgloss.Style
	statsStyle   lipgloss.Style
	errorStyle   lipgloss.Style
)

func init() {
	applyTheme(themes["default"])
}

// applyTheme rebuilds all styles from t.
func applyTheme(t theme) {
	correct := lipgloss.Color(t.Correct)
	wrong := lipgloss.Color(t.Wrong)
	pending := lipgloss.Color(t.Pending)
	accent := lipgloss.Color(t.Accent)

	correctStyle = lipgloss.NewStyle().Foreground(correct)
	wrongStyle = lipgloss.NewStyle().Foreground(wrong).Underline(true)
	wrongSpace = lipgloss.NewStyle().Background(wrong)
	cursorStyle = lipgloss.NewStyle().Reverse(true)
	pendingStyle = lipgloss.NewStyle().Foreground(pending)
	statsStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	errorStyle = lipgloss.NewStyle().Foreground(wrong).Bold(true)
}
