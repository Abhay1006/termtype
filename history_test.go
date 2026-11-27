package main

import (
	"math"
	"testing"
)

func TestSameKind(t *testing.T) {
	base := result{Mode: "time", Length: 30, Punct: true, Numbers: false}
	if !sameKind(base, result{Mode: "time", Length: 30, Punct: true, Numbers: false, WPM: 90}) {
		t.Fatal("expected identical settings to be sameKind")
	}
	diffs := []result{
		{Mode: "words", Length: 30, Punct: true, Numbers: false},
		{Mode: "time", Length: 60, Punct: true, Numbers: false},
		{Mode: "time", Length: 30, Punct: false, Numbers: false},
		{Mode: "time", Length: 30, Punct: true, Numbers: true},
	}
	for _, d := range diffs {
		if sameKind(base, d) {
			t.Fatalf("expected %+v and %+v not to be sameKind", base, d)
		}
	}
}

func TestSummarize(t *testing.T) {
	r := result{Mode: "time", Length: 30, WPM: 80}

	// First result for this mode.
	first := summarize(nil, r)
	if first.hasPrev || first.newBest(r) || first.avgCount != 1 || first.avg != 80 {
		t.Fatalf("unexpected first summary: %+v", first)
	}

	// Unrelated mode in history should still behave as first result.
	other := []result{{Mode: "words", Length: 25, WPM: 120}}
	firstForKind := summarize(other, r)
	if firstForKind.hasPrev || firstForKind.newBest(r) {
		t.Fatalf("unexpected summary with unrelated history: %+v", firstForKind)
	}

	// New best when beating previous best.
	history := []result{
		{Mode: "time", Length: 30, WPM: 60},
		{Mode: "time", Length: 30, WPM: 70},
	}
	best := summarize(history, r)
	if !best.hasPrev || best.prevBest != 70 || !best.newBest(r) {
		t.Fatalf("expected new best over 70: %+v", best)
	}
	if math.Abs(best.avg-70) > 1e-9 || best.avgCount != 3 {
		t.Fatalf("avg = %v (%d), want 70 (3)", best.avg, best.avgCount)
	}

	// Average covers at most the last 10 results (9 from history + current).
	var longHistory []result
	// Two old results at 10 WPM that should fall outside the 10-test window,
	// followed by 9 results at 50 WPM (one at 100 WPM as prevBest).
	longHistory = append(longHistory,
		result{Mode: "time", Length: 30, WPM: 10},
		result{Mode: "time", Length: 30, WPM: 10},
	)
	for i := range 9 {
		wpm := 50.0
		if i == 0 {
			wpm = 100.0
		}
		longHistory = append(longHistory, result{Mode: "time", Length: 30, WPM: wpm})
	}
	cur := result{Mode: "time", Length: 30, WPM: 70}
	s := summarize(longHistory, cur)
	if s.newBest(cur) || s.prevBest != 100 {
		t.Fatalf("expected prevBest=100 and newBest=false: %+v", s)
	}
	// Window = 100 + 8*50 + 70 = 570 => avg = 57.
	if s.avgCount != 10 || math.Abs(s.avg-57) > 1e-9 {
		t.Fatalf("avg = %v (%d), want 57 (10)", s.avg, s.avgCount)
	}
}
