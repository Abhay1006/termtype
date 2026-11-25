package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestConsistency(t *testing.T) {
	tests := []struct {
		name    string
		samples []float64
		want    float64
	}{
		{name: "empty", samples: nil, want: 0},
		{name: "single sample", samples: []float64{80}, want: 0},
		{name: "all zero", samples: []float64{0, 0, 0}, want: 0},
		{name: "steady speed", samples: []float64{75, 75, 75, 75}, want: 100},
		{name: "known variation", samples: []float64{60, 100}, want: 75},
		{name: "clamped at zero", samples: []float64{0, 0, 0, 100}, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := consistency(tc.samples)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("consistency(%v) = %v, want finite number", tc.samples, got)
			}
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("consistency(%v) = %v, want %v", tc.samples, got, tc.want)
			}
		})
	}
}

func TestFinishAndResultsConsistency(t *testing.T) {
	start := time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Second)

	m := model{
		cfg:     settings{mode: modeWords, words: 2},
		target:  "hello world",
		input:   "hello world",
		start:   start,
		samples: []float64{60, 100},
	}

	finished, _ := m.finish(end)
	if math.Abs(finished.result.Consistency-75) > 1e-9 {
		t.Fatalf("result.Consistency = %v, want 75", finished.result.Consistency)
	}

	out := finished.results()
	if !strings.Contains(out, "consistency") || !strings.Contains(out, "75%") {
		t.Fatalf("results() = %q, want consistency 75%%", out)
	}

	// Short test (< 2 samples) should not produce NaN or panic.
	short := model{
		cfg:    settings{mode: modeWords, words: 1},
		target: "hi",
		input:  "hi",
		start:  start,
	}
	shortFinished, _ := short.finish(start.Add(500 * time.Millisecond))
	if shortFinished.result.Consistency != 0 {
		t.Fatalf("short result.Consistency = %v, want 0", shortFinished.result.Consistency)
	}
	if !strings.Contains(shortFinished.results(), "0%") {
		t.Fatalf("short results() = %q, want 0%%", shortFinished.results())
	}
}

func TestResultConsistencyJSON(t *testing.T) {
	r := result{
		Date:        time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC),
		Mode:        "time",
		Length:      30,
		WPM:         82.4,
		Raw:         88.0,
		Accuracy:    96.3,
		Consistency: 84.5,
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"consistency":84.5`) {
		t.Fatalf("marshaled JSON = %s, want consistency field", data)
	}

	var decoded result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if decoded.Consistency != r.Consistency {
		t.Fatalf("decoded.Consistency = %v, want %v", decoded.Consistency, r.Consistency)
	}

	// Existing history entries without consistency should still unmarshal cleanly.
	legacy := `{"date":"2026-11-17T10:30:00Z","mode":"time","length":30,"wpm":82.4,"raw":88.0,"accuracy":96.3}`
	var old result
	if err := json.Unmarshal([]byte(legacy), &old); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if old.Consistency != 0 {
		t.Fatalf("legacy Consistency = %v, want 0", old.Consistency)
	}
	if strings.Contains(string(data), "missed_keys") {
		t.Fatalf("expected missed_keys to be omitted when empty, got %s", data)
	}
}

func TestMissedTrackingAndBackspace(t *testing.T) {
	start := time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC)
	m := model{
		cfg:    settings{mode: modeWords, words: 3},
		target: "their quick brown",
		start:  start,
	}

	// Mistype 'e' and 'i' in "their", backspace, then type everything right.
	m = m.addChars("thxar")
	m.input = ""
	m = m.addChars("their quick brown")

	if m.missedKeys['e'] != 1 || m.missedKeys['i'] != 1 {
		t.Fatalf("missedKeys = %v, want e:1 and i:1", m.missedKeys)
	}
	if m.missedWords[0] != 2 {
		t.Fatalf("missedWords = %v, want word 0 count 2", m.missedWords)
	}

	finished, _ := m.finish(start.Add(5 * time.Second))
	if finished.result.MissedKeys["e"] != 1 || finished.result.MissedKeys["i"] != 1 {
		t.Fatalf("result.MissedKeys = %v, want e:1 and i:1", finished.result.MissedKeys)
	}

	out := finished.results()
	if !strings.Contains(out, "missed") || !strings.Contains(out, "e×1") || !strings.Contains(out, "their") {
		t.Fatalf("results() = %q, want missed keys and word 'their'", out)
	}

	restarted := finished.restart(finished.cfg)
	if len(restarted.missedKeys) != 0 || len(restarted.missedWords) != 0 {
		t.Fatalf("restart should clear missed maps, got keys=%v words=%v", restarted.missedKeys, restarted.missedWords)
	}
}

func TestMissedResultsHiddenAndWidthCap(t *testing.T) {
	start := time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC)
	clean := model{
		cfg:    settings{mode: modeWords, words: 2},
		target: "hello world",
		start:  start,
	}
	clean = clean.addChars("hello world")
	cleanFinished, _ := clean.finish(start.Add(3 * time.Second))
	if cleanFinished.missedResults() != "" {
		t.Fatalf("missedResults() = %q, want empty string when no mistakes", cleanFinished.missedResults())
	}
	if strings.Contains(cleanFinished.results(), "missed") {
		t.Fatalf("results() should hide missed line when no mistakes: %q", cleanFinished.results())
	}

	// Many mistakes across words (including repeated word "their" and space).
	noisy := model{
		cfg:    settings{mode: modeWords, words: 6},
		target: "their through alpha bravo charlie their",
		start:  start,
	}
	// Make mistakes on: 'e' (x4), 'r' (x3), ','/space, and several words.
	noisy = noisy.addChars("thxir throogh xlpha xrava xharlie thxir")
	noisy.missedKeys['e'] = 4
	noisy.missedKeys['r'] = 3
	noisy.missedKeys[' '] = 2
	noisy.missedKeys['a'] = 1
	noisy.missedKeys['b'] = 1
	noisy.missedKeys['c'] = 1

	line := noisy.missedResults()
	if line == "" {
		t.Fatal("expected non-empty missedResults")
	}
	if w := lipgloss.Width(line); w > 60 {
		t.Fatalf("missedResults width = %d, want <= 60 (%q)", w, line)
	}
	if !strings.Contains(line, "e×4") || !strings.Contains(line, "their") {
		t.Fatalf("missedResults = %q, expected top key e×4 and word their", line)
	}
}

func TestResultMissedKeysJSON(t *testing.T) {
	r := result{
		Date:       time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC),
		Mode:       "words",
		Length:     10,
		MissedKeys: map[string]int{"e": 4, ",": 2},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if decoded.MissedKeys["e"] != 4 || decoded.MissedKeys[","] != 2 {
		t.Fatalf("decoded.MissedKeys = %v, want e:4 ,:2", decoded.MissedKeys)
	}
}
