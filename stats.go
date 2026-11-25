package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

const recentStatsLimit = 10

type modeGroup struct {
	sample result
	items  []result
	best   float64
	avg    float64
	wpms   []float64
}

func modeOrder(mode string) int {
	switch mode {
	case "time":
		return 0
	case "words":
		return 1
	case "quote":
		return 2
	default:
		return 3
	}
}

func groupHistory(history []result) []modeGroup {
	var groups []modeGroup
	for _, r := range history {
		idx := slices.IndexFunc(groups, func(g modeGroup) bool {
			return sameKind(g.sample, r)
		})
		if idx < 0 {
			groups = append(groups, modeGroup{sample: r})
			idx = len(groups) - 1
		}
		g := &groups[idx]
		g.items = append(g.items, r)
		g.wpms = append(g.wpms, r.WPM)
		if r.WPM > g.best {
			g.best = r.WPM
		}
	}
	for i := range groups {
		items := groups[i].items
		start := max(0, len(items)-averageOf)
		var sum float64
		for _, r := range items[start:] {
			sum += r.WPM
		}
		groups[i].avg = sum / float64(len(items)-start)
	}
	slices.SortFunc(groups, func(a, b modeGroup) int {
		if c := cmp.Compare(modeOrder(a.sample.Mode), modeOrder(b.sample.Mode)); c != 0 {
			return c
		}
		if c := cmp.Compare(a.sample.Length, b.sample.Length); c != 0 {
			return c
		}
		if a.sample.Punct != b.sample.Punct {
			if !a.sample.Punct {
				return -1
			}
			return 1
		}
		if a.sample.Numbers != b.sample.Numbers {
			if !a.sample.Numbers {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.sample.Mode, b.sample.Mode)
	})
	return groups
}

func formatDuration(d time.Duration) string {
	total := int(d.Round(time.Second).Seconds())
	if total < 60 {
		return fmt.Sprintf("%ds", total)
	}
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}

func aggregateMissedKeys(history []result) []countItem {
	totals := make(map[string]int)
	for _, r := range history {
		for k, c := range r.MissedKeys {
			if k == " " {
				k = "␣"
			}
			totals[k] += c
		}
	}
	return sortCountItems(totals)
}

// formatStats renders a readable summary of past test results.
func formatStats(history []result) string {
	if len(history) == 0 {
		return pendingStyle.Render("no history yet — finish a test to see your stats")
	}

	var totalTime time.Duration
	for _, r := range history {
		totalTime += resultDuration(r)
	}

	var lines []string
	lines = append(lines,
		statsStyle.Render("termtype stats")+"    "+
			pendingStyle.Render("tests ")+statsStyle.Render(strconv.Itoa(len(history)))+"    "+
			pendingStyle.Render("time typed ")+statsStyle.Render(formatDuration(totalTime)),
		"",
		pendingStyle.Render("by mode"),
	)

	groups := groupHistory(history)
	labelWidth := 0
	labels := make([]string, len(groups))
	for i, g := range groups {
		labels[i] = formatModeLabel(g.sample.Mode, g.sample.Length, g.sample.Punct, g.sample.Numbers)
		labelWidth = max(labelWidth, len([]rune(labels[i])))
	}
	for i, g := range groups {
		padded := fmt.Sprintf("%-*s", labelWidth, labels[i])
		avgCount := min(len(g.items), averageOf)
		row := "  " + statsStyle.Render(padded) +
			"  " + pendingStyle.Render("best ") + statsStyle.Render(fmt.Sprintf("%3.0f", g.best)) +
			"  " + pendingStyle.Render(fmt.Sprintf("avg(%d) ", avgCount)) + statsStyle.Render(fmt.Sprintf("%3.0f", g.avg)) +
			"  " + pendingStyle.Render(fmt.Sprintf("(%d)", len(g.items)))
		if len(g.wpms) >= 2 {
			row += "  " + statsStyle.Render(sparkline(g.wpms, 12))
		}
		lines = append(lines, row)
	}

	if weak := aggregateMissedKeys(history); len(weak) > 0 {
		limit := min(5, len(weak))
		parts := make([]string, 0, limit)
		for _, item := range weak[:limit] {
			parts = append(parts, errorStyle.Render(item.name)+pendingStyle.Render("×"+strconv.Itoa(item.count)))
		}
		lines = append(lines, "", pendingStyle.Render("weak keys  ")+strings.Join(parts, "  "))
	}

	lines = append(lines, "", pendingStyle.Render("recent"))
	start := max(0, len(history)-recentStatsLimit)
	for i := len(history) - 1; i >= start; i-- {
		r := history[i]
		mode := formatModeLabel(r.Mode, r.Length, r.Punct, r.Numbers)
		date := r.Date.Local().Format("2006-01-02 15:04")
		row := "  " + pendingStyle.Render(date) +
			"  " + fmt.Sprintf("%-*s", labelWidth, mode) +
			"  " + statsStyle.Render(fmt.Sprintf("%3.0f wpm", r.WPM)) +
			"  " + pendingStyle.Render(fmt.Sprintf("%5.1f%%", r.Accuracy))
		if r.Consistency > 0 {
			row += "  " + pendingStyle.Render(fmt.Sprintf("%.0f%% cons", r.Consistency))
		}
		lines = append(lines, row)
	}

	return strings.Join(lines, "\n")
}
