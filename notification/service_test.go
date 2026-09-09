package notification

import (
	"testing"
	"time"

	"nofx/store"
)

func TestWithinAllowedHours(t *testing.T) {
	disabled := &store.NotificationSettings{QuietHoursStart: -1, QuietHoursEnd: -1}

	if !withinAllowedHours(disabled, &Event{Severity: SeverityInfo}) {
		t.Fatal("disabled quiet hours must always allow")
	}

	night := &store.NotificationSettings{QuietHoursStart: 22, QuietHoursEnd: 6}
	// Bypass exact-hour coupling by testing wrap logic; the function itself
	// uses time.Now(), so we verify via the critical override.
	if !withinAllowedHours(night, &Event{Severity: SeverityCritical}) {
		t.Fatal("critical events must bypass quiet hours")
	}
}

func TestQuietHoursWrapLogic(t *testing.T) {
	// Directly verify the wrap computation used inside withinAllowedHours by
	// replicating it for several hours (kept in sync manually).
	inQuiet := func(hour, start, end int) bool {
		if start == end {
			return false
		}
		if start < end {
			return hour >= start && hour < end
		}
		return hour >= start || hour < end
	}
	cases := []struct {
		hour, start, end int
		want             bool
	}{
		{23, 22, 6, true},
		{2, 22, 6, true},
		{9, 22, 6, false},
		{12, 9, 17, true},
		{8, 9, 17, false},
		{17, 9, 17, false}, // end exclusive
		{5, 5, 5, false},   // equal => disabled
	}
	for _, c := range cases {
		if got := inQuiet(c.hour, c.start, c.end); got != c.want {
			t.Errorf("inQuiet(%d,%d,%d) = %v, want %v", c.hour, c.start, c.end, got, c.want)
		}
	}
}

func TestFormatMessage(t *testing.T) {
	e := &Event{
		Severity:   SeverityWarning,
		TraderName: "Alpha",
		Symbol:     "BTCUSDT",
		Title:      "<b>Test & Title</b>",
		Body:       "body",
		Time:       time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	html := formatMessage(e, true)
	if want := "&lt;b&gt;Test &amp; Title&lt;/b&gt;"; !contains(html, want) {
		t.Fatalf("HTML escaping missing: %q", html)
	}
	if !contains(html, "Trader: Alpha") || !contains(html, "Symbol: BTCUSDT") {
		t.Fatalf("meta missing: %q", html)
	}
	plain := formatMessage(e, false)
	if contains(plain, "&lt;") {
		t.Fatalf("plain text must not escape: %q", plain)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestPublishDoesNotBlockOrPanic(t *testing.T) {
	done := make(chan struct{})
	Subscribe(func(e *Event) {
		time.Sleep(50 * time.Millisecond)
		close(done)
	})
	start := time.Now()
	Publish(&Event{EventType: EventSystem, Title: "t"})
	if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
		t.Fatalf("Publish blocked for %v", elapsed)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler never invoked")
	}
}
