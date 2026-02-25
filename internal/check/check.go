package check

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Mode string

const (
	ModeICMP  Mode = "icmp"
	ModeHTTP  Mode = "http"
	ModeHTTPS Mode = "https"
)

type Options struct {
	Mode             Mode
	Target           string
	Timeout          time.Duration
	ExpectedStatuses map[int]struct{}
}

type Checker struct {
	options    Options
	httpClient *http.Client
}

func NewChecker(options Options) *Checker {
	client := &http.Client{
		Timeout: options.Timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}

	return &Checker{
		options:    options,
		httpClient: client,
	}
}

func ParseExpectedStatuses(input string) (map[int]struct{}, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return map[int]struct{}{}, nil
	}

	values := strings.Split(trimmed, ",")
	statuses := make(map[int]struct{}, len(values))
	for _, value := range values {
		statusText := strings.TrimSpace(value)
		if statusText == "" {
			continue
		}
		statusCode, err := strconv.Atoi(statusText)
		if err != nil {
			return nil, fmt.Errorf("%q is not a valid status code", statusText)
		}
		if statusCode < 100 || statusCode > 599 {
			return nil, fmt.Errorf("%d is outside valid HTTP status range", statusCode)
		}
		statuses[statusCode] = struct{}{}
	}
	return statuses, nil
}

func ResolveModeAndTarget(modeText string, target string) (Mode, string, error) {
	trimmedTarget := strings.TrimSpace(target)
	if trimmedTarget == "" {
		return "", "", errors.New("target is empty")
	}

	switch strings.ToLower(strings.TrimSpace(modeText)) {
	case "icmp":
		return ModeICMP, trimmedTarget, nil
	case "http":
		if strings.HasPrefix(trimmedTarget, "http://") {
			return ModeHTTP, trimmedTarget, nil
		}
		if strings.HasPrefix(trimmedTarget, "https://") {
			return "", "", errors.New("--mode=http cannot use https target")
		}
		return ModeHTTP, "http://" + trimmedTarget, nil
	case "https":
		if strings.HasPrefix(trimmedTarget, "https://") {
			return ModeHTTPS, trimmedTarget, nil
		}
		if strings.HasPrefix(trimmedTarget, "http://") {
			return "", "", errors.New("--mode=https cannot use http target")
		}
		return ModeHTTPS, "https://" + trimmedTarget, nil
	case "auto":
		if strings.HasPrefix(trimmedTarget, "http://") {
			return ModeHTTP, trimmedTarget, nil
		}
		if strings.HasPrefix(trimmedTarget, "https://") {
			return ModeHTTPS, trimmedTarget, nil
		}
		parsed, err := url.Parse(trimmedTarget)
		if err == nil && parsed.Scheme != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			if parsed.Scheme == "http" {
				return ModeHTTP, trimmedTarget, nil
			}
			return ModeHTTPS, trimmedTarget, nil
		}
		return ModeICMP, trimmedTarget, nil
	default:
		return "", "", fmt.Errorf("unsupported mode: %s", modeText)
	}
}

func (checker *Checker) CheckWithRetries(ctx context.Context, retries int) (bool, error) {
	attempts := retries + 1
	var lastErr error

	for attempt := 0; attempt < attempts; attempt++ {
		isUp, err := checker.CheckOnce(ctx)
		if err == nil && isUp {
			return true, nil
		}
		if err != nil {
			lastErr = err
		}
		if attempt < attempts-1 {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-time.After(150 * time.Millisecond):
			}
		}
	}

	if lastErr != nil {
		return false, lastErr
	}
	return false, nil
}

func (checker *Checker) CheckOnce(ctx context.Context) (bool, error) {
	switch checker.options.Mode {
	case ModeICMP:
		return checker.checkICMP(ctx)
	case ModeHTTP, ModeHTTPS:
		return checker.checkHTTP(ctx)
	default:
		return false, fmt.Errorf("unsupported mode %q", checker.options.Mode)
	}
}

func (checker *Checker) checkHTTP(ctx context.Context) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checker.options.Target, nil)
	if err != nil {
		return false, err
	}

	response, err := checker.httpClient.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	if len(checker.options.ExpectedStatuses) == 0 {
		return response.StatusCode >= 200 && response.StatusCode <= 399, nil
	}
	_, ok := checker.options.ExpectedStatuses[response.StatusCode]
	return ok, nil
}

func (checker *Checker) checkICMP(ctx context.Context) (bool, error) {
	args, err := buildPingArgs(checker.options.Target, checker.options.Timeout)
	if err != nil {
		return false, err
	}

	command := exec.CommandContext(ctx, "ping", args...)
	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func buildPingArgs(target string, timeout time.Duration) ([]string, error) {
	if strings.TrimSpace(target) == "" {
		return nil, errors.New("icmp target is empty")
	}

	timeoutMs := timeout.Milliseconds()
	if timeoutMs <= 0 {
		timeoutMs = 1000
	}

	switch runtime.GOOS {
	case "windows":
		return []string{"-n", "1", "-w", strconv.FormatInt(timeoutMs, 10), target}, nil
	case "darwin":
		timeoutSeconds := int(timeout.Seconds())
		if timeoutSeconds < 1 {
			timeoutSeconds = 1
		}
		return []string{"-c", "1", "-W", strconv.Itoa(timeoutSeconds), target}, nil
	default:
		timeoutSeconds := int(timeout.Seconds())
		if timeoutSeconds < 1 {
			timeoutSeconds = 1
		}
		if net.ParseIP(target) == nil {
			return []string{"-c", "1", "-W", strconv.Itoa(timeoutSeconds), target}, nil
		}
		return []string{"-c", "1", "-W", strconv.Itoa(timeoutSeconds), target}, nil
	}
}
