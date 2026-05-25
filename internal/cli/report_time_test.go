package cli

import (
	"testing"
	"time"
)

func TestParseSinceFlagDurationAndAll(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	got, label, err := parseSinceFlag("7d", now)
	if err != nil {
		t.Fatalf("parseSinceFlag: %v", err)
	}
	want := now.Add(-7 * 24 * time.Hour).UnixNano()
	if got != want || label != "7d" {
		t.Fatalf("parseSinceFlag(7d) = (%d, %q), want (%d, 7d)", got, label, want)
	}
	got, label, err = parseSinceFlag("all", now)
	if err != nil {
		t.Fatalf("parseSinceFlag(all): %v", err)
	}
	if got != 0 || label != "all" {
		t.Fatalf("parseSinceFlag(all) = (%d, %q), want (0, all)", got, label)
	}
}

func TestQuotedLabelName(t *testing.T) {
	got := quotedLabelName(`Attached label "security" to task abc12345`)
	if got != "security" {
		t.Fatalf("quotedLabelName = %q, want security", got)
	}
}
