package main

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestFormatStatsEmpty(t *testing.T) {
	for _, history := range [][]result{nil, {}} {
		out := formatStats(history)
		if !strings.Contains(out, "no history yet") {
			t.Fatalf("formatStats(%v) = %q, want empty-history message", history, out)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{42 * time.Second, "42s"},
		{2 * time.Minute, "2m"},
		{5*time.Minute + 12*time.Second, "5m 12s"},
		{83 * time.Minute, "1h 23m"},
	}
	for _, tc := range tests {
		if got := formatDuration(tc.d); got != tc.want {
			t.Fatalf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestGroupHistoryAndAverageLast10(t *testing.T) {
	base := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	var history []result
	// First 2 results have low WPM (20), followed by 10 results with WPM 80
	// and one peak of 100 in the last 10.
	for i := range 12 {
		wpm := 80.0
		if i < 2 {
			wpm = 20.0
		}
		if i == 11 {
			wpm = 100.0
		}
		history = append(history, result{
			Date:     base.Add(time.Duration(i) * time.Minute),
			Mode:     "time",
			Length:   30,
			WPM:      wpm,
			Accuracy: 95,
		})
	}
	history = append(history, result{
		Date:     base.Add(20 * time.Minute),
		Mode:     "quote",
		WPM:      70,
		Accuracy: 98,
		Duration: 18.5,
	})

	groups := groupHistory(history)
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].sample.Mode != "time" || groups[1].sample.Mode != "quote" {
		t.Fatalf("unexpected group order: %+v", groups)
	}
	if groups[0].best != 100 {
		t.Fatalf("time best = %v, want 100", groups[0].best)
	}
	// Last 10 of time 30: nine 80s and one 100 => average 82.
	if math.Abs(groups[0].avg-82) > 1e-9 {
		t.Fatalf("time avg(10) = %v, want 82", groups[0].avg)
	}
}

func TestFormatStatsOutputAndHistoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Missing history file should succeed with nil history.
	loaded, err := loadHistory()
	if err != nil {
		t.Fatalf("loadHistory missing file: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty history, got %d items", len(loaded))
	}

	base := time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC)
	var history []result
	for i := range 12 {
		history = append(history, result{
			Date:        base.Add(time.Duration(i) * time.Minute),
			Mode:        "time",
			Length:      30,
			Punct:       i%2 == 1,
			WPM:         float64(60 + i),
			Raw:         float64(65 + i),
			Accuracy:    96.5,
			Consistency: 82,
			MissedKeys:  map[string]int{"e": 1, " ": 1},
		})
	}
	if err := saveHistory(history); err != nil {
		t.Fatalf("saveHistory: %v", err)
	}
	loaded, err = loadHistory()
	if err != nil {
		t.Fatalf("loadHistory: %v", err)
	}

	out := formatStats(loaded)
	for _, want := range []string{
		"termtype stats",
		"tests 12",
		"time typed 6m",
		"time 30",
		"time 30 · punctuation",
		"weak keys",
		"e×12",
		"␣×12",
		"recent",
		"71 wpm",
		"82% cons",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatStats missing %q in:\n%s", want, out)
		}
	}

	// Recent section should cap at recentStatsLimit (10), so oldest entry (60 wpm) is excluded.
	recentIdx := strings.Index(out, "recent")
	if strings.Contains(out[recentIdx:], "60 wpm") {
		t.Fatalf("expected recent section to show only last %d results:\n%s", recentStatsLimit, out[recentIdx:])
	}
}
