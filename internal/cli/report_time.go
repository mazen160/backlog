package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseSinceFlag(value string, now time.Time) (int64, string, error) {
	label := strings.TrimSpace(value)
	if label == "" {
		label = "7d"
	}
	if strings.EqualFold(label, "all") {
		return 0, "all", nil
	}
	if d, err := parseDurationFlag(label); err == nil {
		return now.Add(-d).UnixNano(), label, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if ts, err := time.Parse(layout, label); err == nil {
			return ts.UnixNano(), label, nil
		}
	}
	return 0, "", fmt.Errorf("invalid time window %q; use values like 7d, 24h, all, or 2026-05-24", value)
}

func parseDurationFlag(value string) (time.Duration, error) {
	s := strings.TrimSpace(strings.ToLower(value))
	if s == "" {
		return 0, fmt.Errorf("duration is required")
	}
	unit := s[len(s)-1]
	if unit == 'd' || unit == 'w' {
		n, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", value)
		}
		hours := 24 * n
		if unit == 'w' {
			hours *= 7
		}
		return time.Duration(hours * float64(time.Hour)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid duration %q", value)
	}
	return d, nil
}

func formatDurationNanos(ns int64) string {
	if ns <= 0 {
		return "0s"
	}
	d := time.Duration(ns).Round(time.Second)
	if d >= 48*time.Hour {
		return fmt.Sprintf("%.1fd", d.Hours()/24)
	}
	if d >= 2*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	if d >= 2*time.Minute {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return d.String()
}
