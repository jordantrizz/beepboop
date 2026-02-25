package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
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
	interval    time.Duration
	timeout     time.Duration
	retries     int
	once        bool
	status      string
	quiet       bool
	noColor     bool
}

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

func parseFlags() (cliConfig, error) {
	config := cliConfig{}

	flag.StringVar(&config.target, "target", "", "Target host/IP/URL to check")
	flag.BoolVar(&config.showVersion, "version", false, "Print version and exit")
	flag.StringVar(&config.mode, "mode", "auto", "Check mode: auto|icmp|http|https")
	flag.DurationVar(&config.interval, "interval", 5*time.Second, "Polling interval")
	flag.DurationVar(&config.timeout, "timeout", 3*time.Second, "Per-check timeout")
	flag.IntVar(&config.retries, "retries", 0, "Additional retry attempts per interval")
	flag.BoolVar(&config.once, "once", false, "Run one check and exit")
	flag.StringVar(&config.status, "status", "", "Expected HTTP status codes, comma-separated (e.g. 200,204)")
	flag.BoolVar(&config.quiet, "quiet", false, "Suppress non-essential output")
	flag.BoolVar(&config.noColor, "no-color", false, "Disable colored output")
	flag.Parse()

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

	mode := strings.ToLower(strings.TrimSpace(config.mode))
	switch mode {
	case "auto", "icmp", "http", "https":
		config.mode = mode
	default:
		return config, errors.New("--mode must be one of auto|icmp|http|https")
	}

	return config, nil
}

func main() {
	appVersion := resolveVersion()

	config, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "usage error: %v\n", err)
		os.Exit(exitUsage)
	}

	if config.showVersion {
		fmt.Println(appVersion)
		os.Exit(exitSuccess)
	}

	outputColors := newColorizer(config.noColor)

	expectedStatuses, err := check.ParseExpectedStatuses(config.status)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --status: %v\n", err)
		os.Exit(exitUsage)
	}

	resolvedMode, normalizedTarget, err := check.ResolveModeAndTarget(config.mode, config.target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "target error: %v\n", err)
		os.Exit(exitUsage)
	}

	if !config.quiet {
		fmt.Printf("beepboop %s: mode=%s target=%s interval=%s timeout=%s retries=%d once=%t\n", appVersion, resolvedMode, normalizedTarget, config.interval, config.timeout, config.retries, config.once)
	}

	checker := check.NewChecker(check.Options{
		Mode:             resolvedMode,
		Target:           normalizedTarget,
		Timeout:          config.timeout,
		ExpectedStatuses: expectedStatuses,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if config.once {
		up, checkErr := checker.CheckWithRetries(ctx, config.retries)
		if checkErr != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", outputColors.err("check failed"), checkErr)
			os.Exit(exitFailure)
		}
		if up {
			beep.Emit()
			if !config.quiet {
				fmt.Println(outputColors.up("target is up"))
			}
			os.Exit(exitSuccess)
		}
		if !config.quiet {
			fmt.Println(outputColors.down("target is down"))
		}
		os.Exit(exitFailure)
	}

	ticker := time.NewTicker(config.interval)
	defer ticker.Stop()

	for {
		up, checkErr := checker.CheckWithRetries(ctx, config.retries)
		if checkErr == nil && up {
			beep.Emit()
			if !config.quiet {
				fmt.Println(outputColors.up("target is up"))
			}
			os.Exit(exitSuccess)
		}

		if !config.quiet {
			if checkErr != nil {
				fmt.Printf("%s: %v\n", outputColors.waiting("still waiting"), checkErr)
			} else {
				fmt.Printf("%s: %s\n", outputColors.waiting("still waiting"), outputColors.down("target is down"))
			}
		}

		select {
		case <-ctx.Done():
			os.Exit(exitCancelled)
		case <-ticker.C:
		}
	}
}
