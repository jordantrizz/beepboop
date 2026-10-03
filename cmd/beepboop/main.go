package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jordantrizz/beepboop/internal/beep"
	"github.com/jordantrizz/beepboop/internal/check"
)

const (
	exitSuccess   = 0
	exitFailure   = 1
	exitUsage     = 2
	exitCancelled = 130
)

var version = "dev"

func resolveVersion() string {
	if trimmed := strings.TrimSpace(version); trimmed != "" && trimmed != "dev" {
		return trimmed
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		if buildVersion := normalizeBuildVersion(buildInfo.Main.Version); buildVersion != "" {
			return buildVersion
		}
	}

	if fileVersion := resolveVersionFromFile(); fileVersion != "" {
		return fileVersion
	}

	return "dev"
}

func normalizeBuildVersion(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "(devel)" {
		return ""
	}
	return strings.TrimPrefix(trimmed, "v")
}

func resolveVersionFromFile() string {
	paths := []string{"VERSION"}
	if executablePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executablePath), "VERSION"))
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if trimmed := strings.TrimSpace(string(content)); trimmed != "" {
			return strings.TrimPrefix(trimmed, "v")
		}
	}

	return ""
}

type cliConfig struct {
	showVersion bool
	target      string
	mode        string
	port        int
	checks      string
	interval    time.Duration
	timeout     time.Duration
	timeoutSet  bool
	retries     int
	once        bool
	reverse     bool
	httpStatus  string
	verbose     bool
	jsonOutput  bool
	quiet       bool
	noColor     bool
}

const (
	defaultTimeout     = 3 * time.Second
	httpDefaultTimeout = 30 * time.Second
)

type colorizer struct {
	enabled bool
}

const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

func newColorizer(disabledByFlag bool) colorizer {
	if disabledByFlag {
		return colorizer{enabled: false}
	}

	if strings.EqualFold(os.Getenv("NO_COLOR"), "1") || os.Getenv("NO_COLOR") != "" {
		return colorizer{enabled: false}
	}

	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return colorizer{enabled: false}
	}

	info, err := os.Stdout.Stat()
	if err != nil {
		return colorizer{enabled: false}
	}

	if info.Mode()&os.ModeCharDevice == 0 {
		return colorizer{enabled: false}
	}

	return colorizer{enabled: true}
}

func (value colorizer) up(text string) string {
	return value.wrap(text, ansiGreen)
}

func (value colorizer) down(text string) string {
	return value.wrap(text, ansiYellow)
}

func (value colorizer) waiting(text string) string {
	return value.wrap(text, ansiCyan)
}

func (value colorizer) err(text string) string {
	return value.wrap(text, ansiRed)
}

func (value colorizer) wrap(text string, color string) string {
	if !value.enabled {
		return text
	}
	return color + text + ansiReset
}

func parseFlags(args []string) (cliConfig, error) {
	config := cliConfig{}

	flagSet := flag.NewFlagSet("beepboop", flag.ContinueOnError)
	flagSet.SetOutput(os.Stderr)
	flagSet.StringVar(&config.target, "target", "", "Target host/IP/URL to check")
	flagSet.BoolVar(&config.showVersion, "version", false, "Print version and exit")
	flagSet.StringVar(&config.mode, "mode", "auto", "Check mode: auto|icmp|http|https|tcp|udp")
	flagSet.IntVar(&config.port, "port", 0, "Port number for tcp/udp checks (can also be embedded in --target as host:port)")
	flagSet.StringVar(&config.checks, "checks", "", "Comma-separated check specs, e.g. icmp,tcp:22,tcp:80 (uses --target as base host; mutually exclusive with --mode/--port)")
	flagSet.DurationVar(&config.interval, "interval", 5*time.Second, "Polling interval")
	flagSet.DurationVar(&config.timeout, "timeout", defaultTimeout, "Per-check timeout")
	flagSet.IntVar(&config.retries, "retries", 0, "Additional retry attempts per interval")
	flagSet.BoolVar(&config.once, "once", false, "Run one check and exit")
	flagSet.BoolVar(&config.reverse, "reverse", false, "Alert when the target is down instead of up")
	flagSet.StringVar(&config.httpStatus, "http-status", "", "Expected HTTP status codes, comma-separated (e.g. 200,204)")
	flagSet.BoolVar(&config.verbose, "verbose", false, "Show per-attempt diagnostic details")
	flagSet.BoolVar(&config.jsonOutput, "json", false, "Output structured JSON lines")
	flagSet.BoolVar(&config.quiet, "quiet", false, "Suppress non-essential output")
	flagSet.BoolVar(&config.noColor, "no-color", false, "Disable colored output")
	if err := flagSet.Parse(args); err != nil {
		return config, err
	}

	if config.showVersion {
		return config, nil
	}

	if strings.TrimSpace(config.target) == "" {
		return config, errors.New("--target is required")
	}
	if config.interval <= 0 {
		return config, errors.New("--interval must be > 0")
	}
	if config.timeout <= 0 {
		return config, errors.New("--timeout must be > 0")
	}
	if config.retries < 0 {
		return config, errors.New("--retries must be >= 0")
	}

	config.timeoutSet = hasDurationFlag(args, "timeout")

	if config.checks != "" && config.mode != "auto" {
		return config, errors.New("--checks and --mode cannot be used together")
	}
	if config.checks != "" && config.port != 0 {
		return config, errors.New("--port cannot be used with --checks; embed the port in the check spec (e.g. tcp:22)")
	}

	mode := strings.ToLower(strings.TrimSpace(config.mode))
	switch mode {
	case "auto", "icmp", "http", "https", "tcp", "udp":
		config.mode = mode
	default:
		return config, errors.New("--mode must be one of auto|icmp|http|https|tcp|udp")
	}

	if config.port != 0 && (config.port < 1 || config.port > 65535) {
		return config, errors.New("--port must be between 1 and 65535")
	}

	if (mode == "tcp" || mode == "udp") && config.port > 0 {
		config.target = net.JoinHostPort(strings.TrimSpace(config.target), strconv.Itoa(config.port))
	}

	return config, nil
}

func hasDurationFlag(args []string, name string) bool {
	flagName := "--" + name
	prefix := flagName + "="
	for _, arg := range args {
		if arg == flagName || strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func checksIncludeHTTP(checks string) bool {
	for _, rawPart := range strings.Split(checks, ",") {
		spec := strings.ToLower(strings.TrimSpace(rawPart))
		if spec == "http" || spec == "https" {
			return true
		}
	}
	return false
}

func effectiveTimeout(config cliConfig, resolvedMode check.Mode) time.Duration {
	if config.timeoutSet {
		return config.timeout
	}

	if resolvedMode == check.ModeHTTP || resolvedMode == check.ModeHTTPS {
		return httpDefaultTimeout
	}

	if config.checks != "" && checksIncludeHTTP(config.checks) {
		return httpDefaultTimeout
	}

	return config.timeout
}

func alertCondition(reverse bool) string {
	if reverse {
		return "down"
	}
	return "up"
}

func alertStateText(reverse bool) string {
	return "target is " + alertCondition(reverse)
}

func waitingStateText(reverse bool) string {
	if reverse {
		return "target is up"
	}
	return "target is down"
}

func shouldAlert(reverse bool, up bool) bool {
	if reverse {
		return !up
	}
	return up
}

type runtimeOutput struct {
	colorizer        colorizer
	isTTY            bool
	quietHumanOutput bool
	verbose          bool
	jsonOutput       bool
	compactActive    bool
}

type jsonEvent struct {
	SchemaVersion string `json:"schema_version"`
	EventType     string `json:"event_type"`
	Timestamp     string `json:"timestamp"`
	RunMode       string `json:"run_mode,omitempty"`
	AlertOn       string `json:"alert_on,omitempty"`
	Mode          string `json:"mode,omitempty"`
	Target        string `json:"target,omitempty"`
	Checks        string `json:"checks,omitempty"`
	PollAttempt   int    `json:"poll_attempt,omitempty"`
	RetryAttempt  int    `json:"retry_attempt,omitempty"`
	RetryTotal    int    `json:"retry_total,omitempty"`
	Status        string `json:"status,omitempty"`
	Error         string `json:"error,omitempty"`
	Up            *bool  `json:"up,omitempty"`
	Alerted       *bool  `json:"alerted,omitempty"`
	StartTime     string `json:"start_time,omitempty"`
	CurrentTime   string `json:"current_time,omitempty"`
	ElapsedMs     int64  `json:"elapsed_ms,omitempty"`
	DurationMs    int64  `json:"duration_ms,omitempty"`
	HTTPStatus    int    `json:"http_status,omitempty"`
	Interval      string `json:"interval,omitempty"`
	Timeout       string `json:"timeout,omitempty"`
	Retries       int    `json:"retries,omitempty"`
	Once          *bool  `json:"once,omitempty"`
	Reverse       *bool  `json:"reverse,omitempty"`
	ExitCode      *int   `json:"exit_code,omitempty"`
}

func newRuntimeOutput(colors colorizer, quiet bool, verbose bool, jsonOutput bool) runtimeOutput {
	return runtimeOutput{
		colorizer:        colors,
		isTTY:            stdoutIsTTY(),
		quietHumanOutput: quiet,
		verbose:          verbose,
		jsonOutput:       jsonOutput,
	}
}

func stdoutIsTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (value *runtimeOutput) printHeader(line string) {
	if value.jsonOutput || value.quietHumanOutput {
		return
	}
	fmt.Println(line)
}

func (value *runtimeOutput) printTerminalStatus(reverse bool, up bool) {
	if value.jsonOutput || value.quietHumanOutput {
		return
	}
	if reverse {
		if up {
			fmt.Println(value.colorizer.up(waitingStateText(reverse)))
			return
		}
		fmt.Println(value.colorizer.down(alertStateText(reverse)))
		return
	}
	if up {
		fmt.Println(value.colorizer.up(alertStateText(reverse)))
		return
	}
	fmt.Println(value.colorizer.down(waitingStateText(reverse)))
}

func (value *runtimeOutput) printCheckError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", value.colorizer.err("check failed"), err)
}

func (value *runtimeOutput) printCompactProgress(line string) {
	if value.jsonOutput || value.quietHumanOutput {
		return
	}
	if value.isTTY {
		value.compactActive = true
		fmt.Printf("\r\033[2K%s", line)
		return
	}
	fmt.Println(line)
}

func (value *runtimeOutput) printVerbose(line string) {
	if value.jsonOutput || value.quietHumanOutput || !value.verbose {
		return
	}
	if value.compactActive {
		fmt.Print("\n")
		value.compactActive = false
	}
	fmt.Println(line)
}

func (value *runtimeOutput) finishProgressLine() {
	if value.compactActive {
		fmt.Print("\n")
		value.compactActive = false
	}
}

func (value *runtimeOutput) emitJSON(event jsonEvent) {
	if !value.jsonOutput {
		return
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		return
	}
	fmt.Println(string(encoded))
}

func boolPtr(input bool) *bool {
	value := input
	return &value
}

func intPtr(input int) *int {
	value := input
	return &value
}

func compactStatusLine(colors colorizer, start time.Time, now time.Time, pollAttempt int, attempt check.AttemptResult, waitingText string, errText string) string {
	status := "down"
	if attempt.Up {
		status = "up"
	}
	if errText != "" {
		status = "error"
	}

	prefix := colors.waiting("still waiting")
	core := fmt.Sprintf(
		"start=%s now=%s elapsed=%s poll=%d retry=%d/%d status=%s",
		start.Format(time.RFC3339),
		now.Format(time.RFC3339),
		now.Sub(start).Round(time.Millisecond),
		pollAttempt,
		attempt.Retry,
		attempt.MaxRetries,
		status,
	)

	if errText != "" {
		return fmt.Sprintf("%s %s err=%q", prefix, core, errText)
	}
	return fmt.Sprintf("%s %s state=%q", prefix, core, waitingText)
}

func verboseAttemptLine(pollAttempt int, attempt check.AttemptResult, errText string) string {
	status := "down"
	if attempt.Up {
		status = "up"
	}
	if errText != "" {
		status = "error"
	}

	if errText != "" {
		return fmt.Sprintf(
			"verbose: poll=%d retry=%d/%d mode=%s target=%s status=%s err=%q duration=%s",
			pollAttempt,
			attempt.Retry,
			attempt.MaxRetries,
			attempt.Mode,
			attempt.Target,
			status,
			errText,
			attempt.Duration.Round(time.Millisecond),
		)
	}

	extra := ""
	if attempt.HTTPStatus > 0 {
		extra = fmt.Sprintf(" http_status=%d", attempt.HTTPStatus)
	}

	return fmt.Sprintf(
		"verbose: poll=%d retry=%d/%d mode=%s target=%s status=%s duration=%s%s",
		pollAttempt,
		attempt.Retry,
		attempt.MaxRetries,
		attempt.Mode,
		attempt.Target,
		status,
		attempt.Duration.Round(time.Millisecond),
		extra,
	)
}

func attemptStatusText(attempt check.AttemptResult) string {
	if attempt.Error != "" {
		return "error"
	}
	if attempt.Up {
		return "up"
	}
	return "down"
}

func main() {
	appVersion := resolveVersion()

	config, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "usage error: %v\n", err)
		os.Exit(exitUsage)
	}

	if config.showVersion {
		fmt.Println(appVersion)
		os.Exit(exitSuccess)
	}

	outputColors := newColorizer(config.noColor)
	runtimeOutput := newRuntimeOutput(outputColors, config.quiet, config.verbose, config.jsonOutput)

	resolvedMode := check.Mode("")
	normalizedTarget := ""
	if config.checks == "" {
		resolvedMode, normalizedTarget, err = check.ResolveModeAndTarget(config.mode, config.target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "target error: %v\n", err)
			os.Exit(exitUsage)
		}
	}

	resolvedTimeout := effectiveTimeout(config, resolvedMode)

	expectedStatuses, err := check.ParseExpectedStatuses(config.httpStatus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --http-status: %v\n", err)
		os.Exit(exitUsage)
	}

	var checkable check.Checkable
	resolvedTargetForOutput := normalizedTarget
	resolvedModeForOutput := string(resolvedMode)
	if config.checks != "" {
		checkOpts, err := check.ParseChecks(config.checks, config.target, resolvedTimeout, expectedStatuses)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --checks: %v\n", err)
			os.Exit(exitUsage)
		}
		checkable = check.NewMultiChecker(checkOpts)
		resolvedModeForOutput = "checks"
		resolvedTargetForOutput = config.target
		runtimeOutput.printHeader(fmt.Sprintf("beepboop %s: checks=%s target=%s interval=%s timeout=%s retries=%d once=%t reverse=%t alert=%s", appVersion, config.checks, config.target, config.interval, resolvedTimeout, config.retries, config.once, config.reverse, alertCondition(config.reverse)))
	} else {
		runtimeOutput.printHeader(fmt.Sprintf("beepboop %s: mode=%s target=%s interval=%s timeout=%s retries=%d once=%t reverse=%t alert=%s", appVersion, resolvedMode, normalizedTarget, config.interval, resolvedTimeout, config.retries, config.once, config.reverse, alertCondition(config.reverse)))
		checkable = check.NewChecker(check.Options{
			Mode:             resolvedMode,
			Target:           normalizedTarget,
			Timeout:          resolvedTimeout,
			ExpectedStatuses: expectedStatuses,
		})
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	detailedCheckable, ok := checkable.(check.DetailedCheckable)
	if !ok {
		fmt.Fprintln(os.Stderr, "runtime error: selected checker does not support detailed output")
		os.Exit(exitFailure)
	}

	runStart := time.Now().UTC()
	runtimeOutput.emitJSON(jsonEvent{
		SchemaVersion: "1.0",
		EventType:     "run_start",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		RunMode: func() string {
			if config.once {
				return "once"
			}
			return "until_success"
		}(),
		AlertOn:   alertCondition(config.reverse),
		Mode:      resolvedModeForOutput,
		Target:    resolvedTargetForOutput,
		Checks:    config.checks,
		Interval:  config.interval.String(),
		Timeout:   resolvedTimeout.String(),
		Retries:   config.retries,
		Once:      boolPtr(config.once),
		Reverse:   boolPtr(config.reverse),
		StartTime: runStart.Format(time.RFC3339Nano),
	})

	if config.once {
		pollAttempt := 1
		outcome := detailedCheckable.CheckWithRetriesDetailed(ctx, config.retries)
		for _, attempt := range outcome.Attempts {
			errText := attempt.Error
			if errText == "" && outcome.Error != "" {
				errText = outcome.Error
			}
			runtimeOutput.printVerbose(verboseAttemptLine(pollAttempt, attempt, errText))
			runtimeOutput.emitJSON(jsonEvent{
				SchemaVersion: "1.0",
				EventType:     "attempt",
				Timestamp:     attempt.Timestamp.Format(time.RFC3339Nano),
				Mode:          string(attempt.Mode),
				Target:        attempt.Target,
				PollAttempt:   pollAttempt,
				RetryAttempt:  attempt.Retry,
				RetryTotal:    attempt.MaxRetries,
				Status:        attemptStatusText(attempt),
				Error:         errText,
				Up:            boolPtr(attempt.Up),
				DurationMs:    attempt.Duration.Milliseconds(),
				HTTPStatus:    attempt.HTTPStatus,
				StartTime:     runStart.Format(time.RFC3339Nano),
				CurrentTime:   attempt.Timestamp.Format(time.RFC3339Nano),
				ElapsedMs:     attempt.Timestamp.Sub(runStart).Milliseconds(),
			})
		}

		if outcome.Error != "" {
			runtimeOutput.printCheckError(errors.New(outcome.Error))
			runtimeOutput.emitJSON(jsonEvent{
				SchemaVersion: "1.0",
				EventType:     "run_result",
				Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
				Status:        "failure",
				Error:         outcome.Error,
				Alerted:       boolPtr(false),
				ExitCode:      intPtr(exitFailure),
				StartTime:     runStart.Format(time.RFC3339Nano),
				CurrentTime:   time.Now().UTC().Format(time.RFC3339Nano),
				ElapsedMs:     time.Since(runStart).Milliseconds(),
			})
			os.Exit(exitFailure)
		}

		alerted := shouldAlert(config.reverse, outcome.Up)
		if alerted {
			beep.Emit()
			runtimeOutput.printTerminalStatus(config.reverse, outcome.Up)
			runtimeOutput.emitJSON(jsonEvent{
				SchemaVersion: "1.0",
				EventType:     "run_result",
				Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
				Status:        "success",
				Up:            boolPtr(outcome.Up),
				Alerted:       boolPtr(true),
				ExitCode:      intPtr(exitSuccess),
				StartTime:     runStart.Format(time.RFC3339Nano),
				CurrentTime:   time.Now().UTC().Format(time.RFC3339Nano),
				ElapsedMs:     time.Since(runStart).Milliseconds(),
			})
			os.Exit(exitSuccess)
		}

		runtimeOutput.printTerminalStatus(config.reverse, outcome.Up)
		runtimeOutput.emitJSON(jsonEvent{
			SchemaVersion: "1.0",
			EventType:     "run_result",
			Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
			Status:        "no_alert",
			Up:            boolPtr(outcome.Up),
			Alerted:       boolPtr(false),
			ExitCode:      intPtr(exitFailure),
			StartTime:     runStart.Format(time.RFC3339Nano),
			CurrentTime:   time.Now().UTC().Format(time.RFC3339Nano),
			ElapsedMs:     time.Since(runStart).Milliseconds(),
		})
		os.Exit(exitFailure)
	}

	ticker := time.NewTicker(config.interval)
	defer ticker.Stop()

	pollAttempt := 0
	for {
		pollAttempt++
		outcome := detailedCheckable.CheckWithRetriesDetailed(ctx, config.retries)
		for _, attempt := range outcome.Attempts {
			errText := attempt.Error
			if errText == "" && outcome.Error != "" {
				errText = outcome.Error
			}
			runtimeOutput.printVerbose(verboseAttemptLine(pollAttempt, attempt, errText))
			runtimeOutput.emitJSON(jsonEvent{
				SchemaVersion: "1.0",
				EventType:     "attempt",
				Timestamp:     attempt.Timestamp.Format(time.RFC3339Nano),
				Mode:          string(attempt.Mode),
				Target:        attempt.Target,
				PollAttempt:   pollAttempt,
				RetryAttempt:  attempt.Retry,
				RetryTotal:    attempt.MaxRetries,
				Status:        attemptStatusText(attempt),
				Error:         errText,
				Up:            boolPtr(attempt.Up),
				DurationMs:    attempt.Duration.Milliseconds(),
				HTTPStatus:    attempt.HTTPStatus,
				StartTime:     runStart.Format(time.RFC3339Nano),
				CurrentTime:   attempt.Timestamp.Format(time.RFC3339Nano),
				ElapsedMs:     attempt.Timestamp.Sub(runStart).Milliseconds(),
			})
		}

		if outcome.Error == "" && shouldAlert(config.reverse, outcome.Up) {
			runtimeOutput.finishProgressLine()
			beep.Emit()
			runtimeOutput.printTerminalStatus(config.reverse, outcome.Up)
			runtimeOutput.emitJSON(jsonEvent{
				SchemaVersion: "1.0",
				EventType:     "run_result",
				Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
				Status:        "success",
				Up:            boolPtr(outcome.Up),
				Alerted:       boolPtr(true),
				ExitCode:      intPtr(exitSuccess),
				StartTime:     runStart.Format(time.RFC3339Nano),
				CurrentTime:   time.Now().UTC().Format(time.RFC3339Nano),
				ElapsedMs:     time.Since(runStart).Milliseconds(),
			})
			os.Exit(exitSuccess)
		}

		if !runtimeOutput.quietHumanOutput && !runtimeOutput.jsonOutput {
			latestAttempt := check.AttemptResult{Retry: 1, MaxRetries: config.retries + 1}
			if len(outcome.Attempts) > 0 {
				latestAttempt = outcome.Attempts[len(outcome.Attempts)-1]
			}

			waitingText := waitingStateText(config.reverse)
			errText := outcome.Error
			line := compactStatusLine(outputColors, runStart, time.Now().UTC(), pollAttempt, latestAttempt, waitingText, errText)
			runtimeOutput.printCompactProgress(line)
		}

		select {
		case <-ctx.Done():
			runtimeOutput.finishProgressLine()
			runtimeOutput.emitJSON(jsonEvent{
				SchemaVersion: "1.0",
				EventType:     "run_result",
				Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
				Status:        "cancelled",
				Alerted:       boolPtr(false),
				ExitCode:      intPtr(exitCancelled),
				StartTime:     runStart.Format(time.RFC3339Nano),
				CurrentTime:   time.Now().UTC().Format(time.RFC3339Nano),
				ElapsedMs:     time.Since(runStart).Milliseconds(),
				Error:         ctx.Err().Error(),
			})
			os.Exit(exitCancelled)
		case <-ticker.C:
		}
	}
}
