package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// result is one finished test, as stored in history.json.
type result struct {
	Date        time.Time `json:"date"`
	Mode        string    `json:"mode"`   // "time", "words" or "quote"
	Length      int       `json:"length"` // seconds or words; 0 for quotes
	Punct       bool      `json:"punctuation,omitempty"`
	Numbers     bool      `json:"numbers,omitempty"`
	WPM         float64   `json:"wpm"`
	Raw         float64   `json:"raw"`
	Accuracy    float64   `json:"accuracy"`
	Consistency float64   `json:"consistency"`
}

// sameKind reports whether two results come from comparable tests.
func sameKind(a, b result) bool {
	return a.Mode == b.Mode && a.Length == b.Length &&
		a.Punct == b.Punct && a.Numbers == b.Numbers
}

// averageOf is how many recent results the average covers.
const averageOf = 10

// summary compares a result with earlier results of the same kind.
type summary struct {
	prevBest float64 // best WPM before this result
	hasPrev  bool    // false on the first test of this kind
	avg      float64 // average WPM of the last averageOf results, this one included
	avgCount int     // how many results went into avg
}

func (s summary) newBest(r result) bool {
	return s.hasPrev && r.WPM > s.prevBest
}

// summarize compares r with history, which must not yet contain r.
func summarize(history []result, r result) summary {
	var s summary
	recent := []float64{r.WPM}
	for i := len(history) - 1; i >= 0; i-- {
		h := history[i]
		if !sameKind(h, r) {
			continue
		}
		if !s.hasPrev || h.WPM > s.prevBest {
			s.prevBest = h.WPM
		}
		s.hasPrev = true
		if len(recent) < averageOf {
			recent = append(recent, h.WPM)
		}
	}
	for _, w := range recent {
		s.avg += w
	}
	s.avgCount = len(recent)
	s.avg /= float64(s.avgCount)
	return s
}

// configDir is where termtype keeps its files, e.g. ~/.config/termtype.
func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "termtype"), nil
}

func historyPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.json"), nil
}

// loadHistory returns saved results. A missing file is not an error.
func loadHistory() ([]result, error) {
	path, err := historyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var history []result
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return history, nil
}

// saveHistory writes all results. It writes to a temporary file first and
// then renames it, so a crash mid-write can't corrupt the history.
func saveHistory(history []result) error {
	path, err := historyPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
