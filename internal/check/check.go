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
	ModeTCP   Mode = "tcp"
	ModeUDP   Mode = "udp"
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

type AttemptResult struct {
	Mode       Mode
	Target     string
	Retry      int
	MaxRetries int
	Up         bool
	Error      string
	Timestamp  time.Time
	Duration   time.Duration
	HTTPStatus int
}

type CheckOutcome struct {
	Up       bool
	Error    string
	Attempts []AttemptResult
}

type DetailedCheckable interface {
	CheckWithRetriesDetailed(ctx context.Context, retries int) CheckOutcome
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
		if strings.Contains(trimmedTarget, "://") {
			parsed, err := url.Parse(trimmedTarget)
			if err == nil && parsed.Scheme != "" {
				scheme := strings.ToLower(parsed.Scheme)
				if scheme == "http" {
					return ModeHTTP, trimmedTarget, nil
				}
				if scheme == "https" {
					return ModeHTTPS, trimmedTarget, nil
				}
				return "", "", fmt.Errorf("unsupported URL scheme %q in --mode=auto target", parsed.Scheme)
			}
		}

		if strings.ContainsAny(trimmedTarget, "/?#") {
			normalized, scheme, normalizeErr := normalizeBareWebTarget(trimmedTarget)
			if normalizeErr != nil {
				return "", "", normalizeErr
			}
			if scheme == "https" {
				return ModeHTTPS, normalized, nil
			}
			return ModeHTTP, normalized, nil
		}

		if host, port, splitErr := net.SplitHostPort(trimmedTarget); splitErr == nil && host != "" && port != "" {
			scheme := inferWebSchemeFromPort(port)
			normalized := scheme + "://" + trimmedTarget
			if scheme == "https" {
				return ModeHTTPS, normalized, nil
			}
			return ModeHTTP, normalized, nil
		}

		return ModeICMP, trimmedTarget, nil
	case "tcp", "udp":
		host, port, err := net.SplitHostPort(trimmedTarget)
		if err != nil || host == "" || port == "" {
			return "", "", fmt.Errorf("--mode=%s target must include host and port (e.g. host:port)", modeText)
		}
		if strings.ToLower(modeText) == "tcp" {
			return ModeTCP, trimmedTarget, nil
		}
		return ModeUDP, trimmedTarget, nil
	default:
		return "", "", fmt.Errorf("unsupported mode: %s", modeText)
	}
}

func normalizeBareWebTarget(target string) (string, string, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return "", "", errors.New("target is empty")
	}

	targetForParse := trimmed
	if strings.HasPrefix(trimmed, "//") {
		targetForParse = "http:" + trimmed
	} else {
		targetForParse = "http://" + trimmed
	}

	parsed, err := url.Parse(targetForParse)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL-like target: %v", err)
	}

	if parsed.Host == "" {
		return "", "", errors.New("URL-like target has no host")
	}

	scheme := "http"
	if parsed.Port() != "" {
		scheme = inferWebSchemeFromPort(parsed.Port())
	}
	parsed.Scheme = scheme

	return parsed.String(), scheme, nil
}

func inferWebSchemeFromPort(port string) string {
	switch port {
	case "443", "8443":
		return "https"
	default:
		return "http"
	}
}

func (checker *Checker) CheckWithRetries(ctx context.Context, retries int) (bool, error) {
	outcome := checker.CheckWithRetriesDetailed(ctx, retries)
	if outcome.Up {
		return true, nil
	}
	if outcome.Error != "" {
		return false, errors.New(outcome.Error)
	}
	return false, nil
}

func (checker *Checker) CheckWithRetriesDetailed(ctx context.Context, retries int) CheckOutcome {
	attempts := retries + 1
	outcome := CheckOutcome{
		Up:       false,
		Attempts: make([]AttemptResult, 0, attempts),
	}

	for attempt := 0; attempt < attempts; attempt++ {
		started := time.Now()
		isUp, meta, err := checker.checkOnceDetailed(ctx)
		attemptResult := AttemptResult{
			Mode:       checker.options.Mode,
			Target:     checker.options.Target,
			Retry:      attempt + 1,
			MaxRetries: attempts,
			Up:         isUp,
			Timestamp:  time.Now().UTC(),
			Duration:   time.Since(started),
			HTTPStatus: meta.statusCode,
		}
		if err != nil {
			attemptResult.Error = err.Error()
			outcome.Error = err.Error()
		}
		outcome.Attempts = append(outcome.Attempts, attemptResult)

		if err == nil && isUp {
			outcome.Up = true
			outcome.Error = ""
			return outcome
		}

		if attempt < attempts-1 {
			select {
			case <-ctx.Done():
				outcome.Error = ctx.Err().Error()
				return outcome
			case <-time.After(150 * time.Millisecond):
			}
		}
	}

	return outcome
}

type checkMetadata struct {
	statusCode int
}

func (checker *Checker) checkOnceDetailed(ctx context.Context) (bool, checkMetadata, error) {
	switch checker.options.Mode {
	case ModeICMP:
		up, err := checker.checkICMP(ctx)
		return up, checkMetadata{}, err
	case ModeHTTP, ModeHTTPS:
		up, statusCode, err := checker.checkHTTPDetailed(ctx)
		return up, checkMetadata{statusCode: statusCode}, err
	case ModeTCP:
		up, err := checker.checkTCP(ctx)
		return up, checkMetadata{}, err
	case ModeUDP:
		up, err := checker.checkUDP(ctx)
		return up, checkMetadata{}, err
	default:
		return false, checkMetadata{}, fmt.Errorf("unsupported mode %q", checker.options.Mode)
	}
}

func (checker *Checker) CheckOnce(ctx context.Context) (bool, error) {
	up, _, err := checker.checkOnceDetailed(ctx)
	return up, err
}

func (checker *Checker) checkHTTP(ctx context.Context) (bool, error) {
	up, _, err := checker.checkHTTPDetailed(ctx)
	return up, err
}

func (checker *Checker) checkHTTPDetailed(ctx context.Context) (bool, int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checker.options.Target, nil)
	if err != nil {
		return false, 0, err
	}

	response, err := checker.httpClient.Do(request)
	if err != nil {
		return false, 0, err
	}
	defer response.Body.Close()

	if len(checker.options.ExpectedStatuses) == 0 {
		return response.StatusCode == http.StatusOK, response.StatusCode, nil
	}
	_, ok := checker.options.ExpectedStatuses[response.StatusCode]
	return ok, response.StatusCode, nil
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

func (checker *Checker) checkTCP(ctx context.Context) (bool, error) {
	dialer := &net.Dialer{Timeout: checker.options.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", checker.options.Target)
	if err != nil {
		return false, nil
	}
	conn.Close()
	return true, nil
}

// Checkable is implemented by both Checker and MultiChecker.
type Checkable interface {
	CheckWithRetries(ctx context.Context, retries int) (bool, error)
}

// extractHost returns the plain host (and optional port) from a target string by
// stripping any URL scheme. Returns baseTarget unchanged when no scheme is present.
func extractHost(baseTarget string) (string, error) {
	baseTarget = strings.TrimSpace(baseTarget)
	if baseTarget == "" {
		return "", errors.New("base target is empty")
	}
	if strings.Contains(baseTarget, "://") {
		u, err := url.Parse(baseTarget)
		if err != nil {
			return "", fmt.Errorf("invalid target URL: %v", err)
		}
		if u.Host == "" {
			return "", errors.New("URL target has no host")
		}
		return u.Host, nil
	}
	return baseTarget, nil
}

// ParseCheckSpec parses a single check specification into Options using baseTarget as
// the base host. Supported formats:
//   - "icmp"     — ICMP ping to the base host
//   - "tcp:PORT" — TCP connection to base host:PORT
//   - "udp:PORT" — UDP probe to base host:PORT
//   - "http"     — HTTP GET to http://base host
//   - "https"    — HTTPS GET to https://base host
func ParseCheckSpec(spec string, baseTarget string, timeout time.Duration) (Options, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return Options{}, errors.New("check spec is empty")
	}

	parts := strings.SplitN(spec, ":", 2)
	modeStr := strings.ToLower(strings.TrimSpace(parts[0]))

	host, err := extractHost(baseTarget)
	if err != nil {
		return Options{}, err
	}

	switch modeStr {
	case "icmp":
		if len(parts) > 1 {
			return Options{}, fmt.Errorf("icmp check spec does not accept a suffix, got %q", spec)
		}
		icmpHost := host
		if hostname, _, splitErr := net.SplitHostPort(host); splitErr == nil {
			icmpHost = hostname
		}
		return Options{Mode: ModeICMP, Target: icmpHost, Timeout: timeout}, nil

	case "tcp", "udp":
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return Options{}, fmt.Errorf("%s check spec requires a port, e.g. %s:80", modeStr, modeStr)
		}
		portStr := strings.TrimSpace(parts[1])
		portNum, convErr := strconv.Atoi(portStr)
		if convErr != nil || portNum < 1 || portNum > 65535 {
			return Options{}, fmt.Errorf("%q is not a valid port number in check spec", portStr)
		}
		tcpHost := host
		if hostname, _, splitErr := net.SplitHostPort(host); splitErr == nil {
			tcpHost = hostname
		}
		target := net.JoinHostPort(tcpHost, portStr)
		if modeStr == "tcp" {
			return Options{Mode: ModeTCP, Target: target, Timeout: timeout}, nil
		}
		return Options{Mode: ModeUDP, Target: target, Timeout: timeout}, nil

	case "http":
		if len(parts) > 1 {
			return Options{}, fmt.Errorf("http check spec does not accept a suffix, got %q", spec)
		}
		return Options{Mode: ModeHTTP, Target: "http://" + host, Timeout: timeout}, nil

	case "https":
		if len(parts) > 1 {
			return Options{}, fmt.Errorf("https check spec does not accept a suffix, got %q", spec)
		}
		return Options{Mode: ModeHTTPS, Target: "https://" + host, Timeout: timeout}, nil

	default:
		return Options{}, fmt.Errorf("unknown check mode %q; supported: icmp, tcp:PORT, udp:PORT, http, https", modeStr)
	}
}

// ParseChecks parses a comma-separated list of check specs into a slice of Options.
// The expectedStatuses map is applied to any http or https check options.
func ParseChecks(checksStr string, baseTarget string, timeout time.Duration, expectedStatuses map[int]struct{}) ([]Options, error) {
	checksStr = strings.TrimSpace(checksStr)
	if checksStr == "" {
		return nil, errors.New("checks value is empty")
	}

	parts := strings.Split(checksStr, ",")
	opts := make([]Options, 0, len(parts))
	for _, part := range parts {
		spec := strings.TrimSpace(part)
		if spec == "" {
			continue
		}
		opt, err := ParseCheckSpec(spec, baseTarget, timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid check spec %q: %v", spec, err)
		}
		if (opt.Mode == ModeHTTP || opt.Mode == ModeHTTPS) && len(expectedStatuses) > 0 {
			opt.ExpectedStatuses = expectedStatuses
		}
		opts = append(opts, opt)
	}
	if len(opts) == 0 {
		return nil, errors.New("no valid check specs found")
	}
	return opts, nil
}

// MultiChecker runs multiple Checkers and considers the target up only when every
// individual check passes.
type MultiChecker struct {
	checkers []*Checker
}

// NewMultiChecker creates a MultiChecker from a slice of Options.
func NewMultiChecker(opts []Options) *MultiChecker {
	checkers := make([]*Checker, len(opts))
	for i, opt := range opts {
		checkers[i] = NewChecker(opt)
	}
	return &MultiChecker{checkers: checkers}
}

// CheckWithRetries runs all contained checkers with the given retries.
// Returns (true, nil) only when every checker reports up.
// Returns (false, err) if any checker returns an error; (false, nil) if any reports down.
func (m *MultiChecker) CheckWithRetries(ctx context.Context, retries int) (bool, error) {
	outcome := m.CheckWithRetriesDetailed(ctx, retries)
	if outcome.Up {
		return true, nil
	}
	if outcome.Error != "" {
		return false, errors.New(outcome.Error)
	}
	return false, nil
}

func (m *MultiChecker) CheckWithRetriesDetailed(ctx context.Context, retries int) CheckOutcome {
	outcome := CheckOutcome{Attempts: []AttemptResult{}}
	for _, checker := range m.checkers {
		checkerOutcome := checker.CheckWithRetriesDetailed(ctx, retries)
		outcome.Attempts = append(outcome.Attempts, checkerOutcome.Attempts...)
		if checkerOutcome.Error != "" {
			outcome.Up = false
			outcome.Error = checkerOutcome.Error
			return outcome
		}
		if !checkerOutcome.Up {
			outcome.Up = false
			return outcome
		}
	}
	outcome.Up = true
	return outcome
}

// checkUDP probes a UDP port by sending a single byte and waiting for a response.
// If an ICMP "port unreachable" error is received the port is considered down.
// A read timeout (no ICMP response) is treated as up, since many UDP services
// do not reply to unknown payloads. UDP port detection is best-effort and may
// produce false positives when ICMP is filtered or the service is silent.
func (checker *Checker) checkUDP(ctx context.Context) (bool, error) {
	dialer := &net.Dialer{Timeout: checker.options.Timeout}
	conn, err := dialer.DialContext(ctx, "udp", checker.options.Target)
	if err != nil {
		return false, nil
	}
	defer conn.Close()

	deadline := time.Now().Add(checker.options.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return false, err
	}

	if _, err := conn.Write([]byte{0}); err != nil {
		return false, nil
	}

	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && !opErr.Timeout() {
			// ICMP port unreachable — port is closed
			return false, nil
		}
		// Read timed out — no ICMP unreachable received; port is likely open
		return true, nil
	}
	return true, nil
}
