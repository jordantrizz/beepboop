package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jordantrizz/beepboop/internal/check"
)

func TestParseFlagsReverse(t *testing.T) {
	config, err := parseFlags([]string{"--target", "example.com", "--reverse", "--once"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !config.reverse {
		t.Fatal("expected reverse to be enabled")
	}

	if !config.once {
		t.Fatal("expected once to be enabled")
	}
}

func TestShouldAlert(t *testing.T) {
	testCases := []struct {
		name    string
		reverse bool
		up      bool
		want    bool
	}{
		{name: "normal mode alerts on up", reverse: false, up: true, want: true},
		{name: "normal mode ignores down", reverse: false, up: false, want: false},
		{name: "reverse mode alerts on down", reverse: true, up: false, want: true},
		{name: "reverse mode ignores up", reverse: true, up: true, want: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := shouldAlert(testCase.reverse, testCase.up)
			if got != testCase.want {
				t.Fatalf("got %t want %t", got, testCase.want)
			}
		})
	}
}

func TestAlertAndWaitingStateText(t *testing.T) {
	if got := alertStateText(false); got != "target is up" {
		t.Fatalf("unexpected normal alert text: %q", got)
	}

	if got := alertStateText(true); got != "target is down" {
		t.Fatalf("unexpected reverse alert text: %q", got)
	}

	if got := waitingStateText(false); got != "target is down" {
		t.Fatalf("unexpected normal waiting text: %q", got)
	}

	if got := waitingStateText(true); got != "target is up" {
		t.Fatalf("unexpected reverse waiting text: %q", got)
	}
}

func TestParseFlagsHTTPStatus(t *testing.T) {
	config, err := parseFlags([]string{"--target", "https://example.com", "--http-status", "301", "--once"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.httpStatus != "301" {
		t.Fatalf("expected http status value to be set, got %q", config.httpStatus)
	}
}

func TestParseFlagsRejectsLegacyStatusFlag(t *testing.T) {
	_, err := parseFlags([]string{"--target", "https://example.com", "--status", "301", "--once"})
	if err == nil {
		t.Fatal("expected error for legacy --status flag")
	}
}

func TestParseFlagsVerboseAndJSON(t *testing.T) {
	config, err := parseFlags([]string{"--target", "example.com", "--verbose", "--json", "--once"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !config.verbose {
		t.Fatal("expected verbose flag to be enabled")
	}
	if !config.jsonOutput {
		t.Fatal("expected json flag to be enabled")
	}
}

func TestCompactStatusLineIncludesRuntimeFields(t *testing.T) {
	start := time.Date(2026, time.March, 16, 12, 0, 0, 0, time.UTC)
	now := start.Add(5*time.Second + 250*time.Millisecond)
	line := compactStatusLine(colorizer{}, true, time.Time{}, start, now, 3, check.AttemptResult{Retry: 2, MaxRetries: 4, Up: false}, "target is down", "")

	expectedParts := []string{
		"start=2026-03-16T12:00:00Z",
		"now=2026-03-16T12:00:05Z",
		"elapsed=5.25s",
		"poll=3",
		"retry=2/4",
		"status=down",
	}

	for _, part := range expectedParts {
		if !strings.Contains(line, part) {
			t.Fatalf("status line missing %q: %q", part, line)
		}
	}
}

func TestCompactStatusLineNoTiming(t *testing.T) {
	start := time.Date(2026, time.March, 16, 12, 0, 0, 0, time.UTC)
	now := start.Add(5*time.Second + 250*time.Millisecond)
	line := compactStatusLine(colorizer{}, false, time.Time{}, start, now, 3, check.AttemptResult{Retry: 2, MaxRetries: 4, Up: false}, "target is down", "")

	for _, token := range []string{"start=", "now=", "elapsed=", "since=", "waiting="} {
		if strings.Contains(line, token) {
			t.Fatalf("status line should not contain %q when timing disabled: %q", token, line)
		}
	}

	for _, part := range []string{"poll=3", "retry=2/4", "status=down", `state="target is down"`} {
		if !strings.Contains(line, part) {
			t.Fatalf("status line missing %q: %q", part, line)
		}
	}
}

func TestParseFlagsNoTiming(t *testing.T) {
	config, err := parseFlags([]string{"--target", "example.com", "--no-timing", "--once", "--interval", "2s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !config.noTiming {
		t.Fatal("expected noTiming to be enabled")
	}
	if !config.once {
		t.Fatal("expected once to still be enabled")
	}
	if config.interval != 2*time.Second {
		t.Fatalf("expected interval 2s, got %v", config.interval)
	}
}

func TestCompactStatusLineWaitingFields(t *testing.T) {
	start := time.Date(2026, time.March, 16, 12, 0, 0, 0, time.UTC)
	waitingSince := start.Add(1 * time.Second)
	now := start.Add(5*time.Second + 250*time.Millisecond)

	line := compactStatusLine(colorizer{}, true, waitingSince, start, now, 3, check.AttemptResult{Retry: 1, MaxRetries: 1, Up: false}, "target is down", "")
	for _, part := range []string{"since=2026-03-16T12:00:01Z", "waiting=4.25s"} {
		if !strings.Contains(line, part) {
			t.Fatalf("status line missing %q: %q", part, line)
		}
	}

	noTiming := compactStatusLine(colorizer{}, false, waitingSince, start, now, 3, check.AttemptResult{Retry: 1, MaxRetries: 1, Up: false}, "target is down", "")
	if strings.Contains(noTiming, "since=") || strings.Contains(noTiming, "waiting=") {
		t.Fatalf("timing-disabled line should omit waiting tokens: %q", noTiming)
	}
}

func TestJSONAttemptWaitingFields(t *testing.T) {
	waitingSince := time.Date(2026, time.March, 16, 12, 0, 0, 0, time.UTC)

	timed := jsonEvent{
		SchemaVersion:    "1.0",
		EventType:        "attempt",
		WaitingState:     "down",
		WaitingSince:     waitingSince.Format(time.RFC3339Nano),
		WaitingElapsedMs: 3000,
	}
	raw, err := json.Marshal(timed)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	for _, key := range []string{"waiting_state", "waiting_since", "waiting_elapsed_ms"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing key %q in timed attempt payload: %s", key, string(raw))
		}
	}

	untimed := jsonEvent{SchemaVersion: "1.0", EventType: "attempt"}
	raw, err = json.Marshal(untimed)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	for _, key := range []string{"waiting_state", "waiting_since", "waiting_elapsed_ms"} {
		if strings.Contains(string(raw), `"`+key+`"`) {
			t.Fatalf("key %q should be omitted when unset: %s", key, string(raw))
		}
	}
}

func TestJSONEventSingleCheckShape(t *testing.T) {
	event := jsonEvent{
		SchemaVersion: "1.0",
		EventType:     "attempt",
		Timestamp:     "2026-03-16T12:00:00Z",
		Mode:          "http",
		Target:        "https://example.com",
		PollAttempt:   1,
		RetryAttempt:  1,
		RetryTotal:    1,
		Status:        "up",
		Up:            boolPtr(true),
		DurationMs:    50,
		StartTime:     "2026-03-16T11:59:59Z",
		CurrentTime:   "2026-03-16T12:00:00Z",
		ElapsedMs:     1000,
	}

	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	requiredKeys := []string{"schema_version", "event_type", "mode", "target", "poll_attempt", "retry_attempt", "retry_total", "status", "up", "duration_ms", "start_time", "current_time", "elapsed_ms"}
	for _, key := range requiredKeys {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing key %q in json event payload: %s", key, string(raw))
		}
	}
}

func TestJSONEventRunUntilSuccessShape(t *testing.T) {
	event := jsonEvent{
		SchemaVersion:    "1.0",
		EventType:        "run_result",
		Timestamp:        "2026-03-16T12:00:10Z",
		Status:           "success",
		Alerted:          boolPtr(true),
		ExitCode:         intPtr(exitSuccess),
		StartTime:        "2026-03-16T12:00:00Z",
		CurrentTime:      "2026-03-16T12:00:10Z",
		ElapsedMs:        10000,
		WaitingState:     "down",
		WaitingSince:     "2026-03-16T12:00:00Z",
		WaitingElapsedMs: 10000,
	}

	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	for _, key := range []string{"schema_version", "event_type", "status", "alerted", "exit_code", "start_time", "current_time", "elapsed_ms", "waiting_state", "waiting_since", "waiting_elapsed_ms"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing key %q in run result payload: %s", key, string(raw))
		}
	}
}

func TestDescribeWaiting(t *testing.T) {
	since := time.Date(2026, time.March, 16, 12, 0, 0, 0, time.UTC)

	if got, want := describeWaiting(false, since, since.Add(12*time.Second+300*time.Millisecond)), "down for 12.3s (since 2026-03-16T12:00:00Z)"; got != want {
		t.Fatalf("normal mode: got %q want %q", got, want)
	}

	if got, want := describeWaiting(true, since, since.Add(4*time.Second+500*time.Millisecond)), "up for 4.5s (since 2026-03-16T12:00:00Z)"; got != want {
		t.Fatalf("reverse mode: got %q want %q", got, want)
	}

	if got := describeWaiting(false, time.Time{}, since); got != "" {
		t.Fatalf("expected empty summary for zero since, got %q", got)
	}
}

func TestWithWaitingSummary(t *testing.T) {
	if got := withWaitingSummary("target is up", ""); got != "target is up" {
		t.Fatalf("empty summary changed text: %q", got)
	}
	if got, want := withWaitingSummary("target is up", "down for 1.5s (since 2026-03-16T12:00:00Z)"), "target is up after being down for 1.5s (since 2026-03-16T12:00:00Z)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEffectiveTimeoutDefaults(t *testing.T) {
	testCases := []struct {
		name         string
		config       cliConfig
		resolvedMode check.Mode
		want         time.Duration
	}{
		{
			name:         "http defaults to 30s",
			config:       cliConfig{timeout: defaultTimeout},
			resolvedMode: check.ModeHTTP,
			want:         httpDefaultTimeout,
		},
		{
			name:         "https defaults to 30s",
			config:       cliConfig{timeout: defaultTimeout},
			resolvedMode: check.ModeHTTPS,
			want:         httpDefaultTimeout,
		},
		{
			name:         "tcp keeps 3s default",
			config:       cliConfig{timeout: defaultTimeout},
			resolvedMode: check.ModeTCP,
			want:         defaultTimeout,
		},
		{
			name:         "checks with http default to 30s",
			config:       cliConfig{timeout: defaultTimeout, checks: "icmp,http"},
			resolvedMode: check.Mode(""),
			want:         httpDefaultTimeout,
		},
		{
			name:         "explicit timeout is preserved",
			config:       cliConfig{timeout: 7 * time.Second, timeoutSet: true},
			resolvedMode: check.ModeHTTPS,
			want:         7 * time.Second,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := effectiveTimeout(testCase.config, testCase.resolvedMode)
			if got != testCase.want {
				t.Fatalf("timeout mismatch: got %v want %v", got, testCase.want)
			}
		})
	}
}
