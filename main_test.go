package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
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
}
