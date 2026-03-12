package check

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestResolveModeAndTargetAuto(t *testing.T) {
	testCases := []struct {
		name           string
		target         string
		expectedMode   Mode
		expectedTarget string
	}{
		{
			name:           "auto host uses icmp",
			target:         "example.com",
			expectedMode:   ModeICMP,
			expectedTarget: "example.com",
		},
		{
			name:           "auto host:port uses tcp",
			target:         "example.com:22",
			expectedMode:   ModeTCP,
			expectedTarget: "example.com:22",
		},
		{
			name:           "auto ip:port uses tcp",
			target:         "192.0.2.1:80",
			expectedMode:   ModeTCP,
			expectedTarget: "192.0.2.1:80",
		},
		{
			name:           "auto http url uses http",
			target:         "http://example.com",
			expectedMode:   ModeHTTP,
			expectedTarget: "http://example.com",
		},
		{
			name:           "auto https url uses https",
			target:         "https://example.com",
			expectedMode:   ModeHTTPS,
			expectedTarget: "https://example.com",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mode, normalizedTarget, err := ResolveModeAndTarget("auto", testCase.target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != testCase.expectedMode {
				t.Fatalf("mode mismatch: got %s want %s", mode, testCase.expectedMode)
			}
			if normalizedTarget != testCase.expectedTarget {
				t.Fatalf("target mismatch: got %s want %s", normalizedTarget, testCase.expectedTarget)
			}
		})
	}
}

func TestParseExpectedStatuses(t *testing.T) {
	statuses, err := ParseExpectedStatuses("200, 204,301")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(statuses) != 3 {
		t.Fatalf("unexpected number of statuses: got %d", len(statuses))
	}

	if _, ok := statuses[200]; !ok {
		t.Fatal("missing 200")
	}
	if _, ok := statuses[204]; !ok {
		t.Fatal("missing 204")
	}
	if _, ok := statuses[301]; !ok {
		t.Fatal("missing 301")
	}
}

func TestResolveModeAndTargetTCP(t *testing.T) {
	testCases := []struct {
		name           string
		target         string
		expectedMode   Mode
		expectedTarget string
		wantErr        bool
	}{
		{
			name:           "tcp with host:port",
			target:         "example.com:80",
			expectedMode:   ModeTCP,
			expectedTarget: "example.com:80",
		},
		{
			name:           "tcp with ip:port",
			target:         "127.0.0.1:443",
			expectedMode:   ModeTCP,
			expectedTarget: "127.0.0.1:443",
		},
		{
			name:    "tcp without port returns error",
			target:  "example.com",
			wantErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mode, normalizedTarget, err := ResolveModeAndTarget("tcp", testCase.target)
			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != testCase.expectedMode {
				t.Fatalf("mode mismatch: got %s want %s", mode, testCase.expectedMode)
			}
			if normalizedTarget != testCase.expectedTarget {
				t.Fatalf("target mismatch: got %s want %s", normalizedTarget, testCase.expectedTarget)
			}
		})
	}
}

func TestResolveModeAndTargetUDP(t *testing.T) {
	testCases := []struct {
		name           string
		target         string
		expectedMode   Mode
		expectedTarget string
		wantErr        bool
	}{
		{
			name:           "udp with host:port",
			target:         "example.com:53",
			expectedMode:   ModeUDP,
			expectedTarget: "example.com:53",
		},
		{
			name:    "udp without port returns error",
			target:  "example.com",
			wantErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mode, normalizedTarget, err := ResolveModeAndTarget("udp", testCase.target)
			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != testCase.expectedMode {
				t.Fatalf("mode mismatch: got %s want %s", mode, testCase.expectedMode)
			}
			if normalizedTarget != testCase.expectedTarget {
				t.Fatalf("target mismatch: got %s want %s", normalizedTarget, testCase.expectedTarget)
			}
		})
	}
}

func TestCheckTCPUp(t *testing.T) {
	// Start a real TCP listener to test against
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	checker := NewChecker(Options{
		Mode:    ModeTCP,
		Target:  addr,
		Timeout: 3 * time.Second,
	})

	up, err := checker.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !up {
		t.Fatal("expected target to be up")
	}
}

func TestCheckTCPDown(t *testing.T) {
	// Use a port that should be closed (pick a free port then immediately close)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close() // Close immediately so port is not listening.
	// Note: a brief race exists where another process could bind this port before
	// CheckOnce runs. This is acceptable in tests as it would cause a false
	// positive that is very unlikely in practice.

	checker := NewChecker(Options{
		Mode:    ModeTCP,
		Target:  addr,
		Timeout: 3 * time.Second,
	})

	up, err := checker.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if up {
		t.Fatal("expected target to be down")
	}
}

func TestParseCheckSpec(t *testing.T) {
	timeout := 3 * time.Second
	testCases := []struct {
		name        string
		spec        string
		baseTarget  string
		wantMode    Mode
		wantTarget  string
		wantErr     bool
	}{
		{name: "icmp plain host", spec: "icmp", baseTarget: "example.com", wantMode: ModeICMP, wantTarget: "example.com"},
		{name: "icmp strips port from base", spec: "icmp", baseTarget: "example.com:22", wantMode: ModeICMP, wantTarget: "example.com"},
		{name: "icmp with url base", spec: "icmp", baseTarget: "https://example.com", wantMode: ModeICMP, wantTarget: "example.com"},
		{name: "tcp port 22", spec: "tcp:22", baseTarget: "example.com", wantMode: ModeTCP, wantTarget: "example.com:22"},
		{name: "tcp port 80", spec: "tcp:80", baseTarget: "example.com", wantMode: ModeTCP, wantTarget: "example.com:80"},
		{name: "tcp with url base", spec: "tcp:443", baseTarget: "https://example.com", wantMode: ModeTCP, wantTarget: "example.com:443"},
		{name: "udp port 53", spec: "udp:53", baseTarget: "example.com", wantMode: ModeUDP, wantTarget: "example.com:53"},
		{name: "http plain host", spec: "http", baseTarget: "example.com", wantMode: ModeHTTP, wantTarget: "http://example.com"},
		{name: "http with url base", spec: "http", baseTarget: "https://example.com", wantMode: ModeHTTP, wantTarget: "http://example.com"},
		{name: "https plain host", spec: "https", baseTarget: "example.com", wantMode: ModeHTTPS, wantTarget: "https://example.com"},
		// error cases
		{name: "empty spec", spec: "", baseTarget: "example.com", wantErr: true},
		{name: "empty base target", spec: "icmp", baseTarget: "", wantErr: true},
		{name: "icmp with suffix", spec: "icmp:something", baseTarget: "example.com", wantErr: true},
		{name: "tcp no port", spec: "tcp", baseTarget: "example.com", wantErr: true},
		{name: "udp no port", spec: "udp", baseTarget: "example.com", wantErr: true},
		{name: "tcp invalid port string", spec: "tcp:abc", baseTarget: "example.com", wantErr: true},
		{name: "tcp port zero", spec: "tcp:0", baseTarget: "example.com", wantErr: true},
		{name: "tcp port out of range", spec: "tcp:99999", baseTarget: "example.com", wantErr: true},
		{name: "http with suffix", spec: "http:extra", baseTarget: "example.com", wantErr: true},
		{name: "https with suffix", spec: "https:extra", baseTarget: "example.com", wantErr: true},
		{name: "unknown mode", spec: "ftp", baseTarget: "example.com", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := ParseCheckSpec(tc.spec, tc.baseTarget, timeout)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil; opts=%+v", opts)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.Mode != tc.wantMode {
				t.Errorf("mode: got %q, want %q", opts.Mode, tc.wantMode)
			}
			if opts.Target != tc.wantTarget {
				t.Errorf("target: got %q, want %q", opts.Target, tc.wantTarget)
			}
			if opts.Timeout != timeout {
				t.Errorf("timeout: got %v, want %v", opts.Timeout, timeout)
			}
		})
	}
}

func TestParseChecks(t *testing.T) {
	timeout := 3 * time.Second

	opts, err := ParseChecks("icmp,tcp:22,tcp:80", "example.com", timeout, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 3 {
		t.Fatalf("expected 3 opts, got %d", len(opts))
	}
	if opts[0].Mode != ModeICMP || opts[0].Target != "example.com" {
		t.Errorf("first check: got mode=%s target=%s", opts[0].Mode, opts[0].Target)
	}
	if opts[1].Mode != ModeTCP || opts[1].Target != "example.com:22" {
		t.Errorf("second check: got mode=%s target=%s", opts[1].Mode, opts[1].Target)
	}
	if opts[2].Mode != ModeTCP || opts[2].Target != "example.com:80" {
		t.Errorf("third check: got mode=%s target=%s", opts[2].Mode, opts[2].Target)
	}
}

func TestParseChecksAppliesExpectedStatuses(t *testing.T) {
	statuses := map[int]struct{}{200: {}}
	opts, err := ParseChecks("http,https", "example.com", 3*time.Second, statuses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, opt := range opts {
		if _, ok := opt.ExpectedStatuses[200]; !ok {
			t.Errorf("mode %s: expected 200 in ExpectedStatuses", opt.Mode)
		}
	}
}

func TestParseChecksEmptyReturnsError(t *testing.T) {
	_, err := ParseChecks("", "example.com", 3*time.Second, nil)
	if err == nil {
		t.Error("expected error for empty checks string, got nil")
	}
}

func TestParseChecksInvalidSpecReturnsError(t *testing.T) {
	_, err := ParseChecks("icmp,tcp:notaport", "example.com", 3*time.Second, nil)
	if err == nil {
		t.Error("expected error for invalid spec, got nil")
	}
}

func TestMultiCheckerAllUp(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()
	addr := listener.Addr().String()

	opts := []Options{
		{Mode: ModeTCP, Target: addr, Timeout: 2 * time.Second},
		{Mode: ModeTCP, Target: addr, Timeout: 2 * time.Second},
	}
	mc := NewMultiChecker(opts)
	up, err := mc.CheckWithRetries(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !up {
		t.Error("expected up when all checks pass, got down")
	}
}

func TestMultiCheckerOneDown(t *testing.T) {
	openListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start open listener: %v", err)
	}
	defer openListener.Close()
	openAddr := openListener.Addr().String()

	closedListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find closed port: %v", err)
	}
	closedAddr := closedListener.Addr().String()
	closedListener.Close()

	opts := []Options{
		{Mode: ModeTCP, Target: openAddr, Timeout: 2 * time.Second},
		{Mode: ModeTCP, Target: closedAddr, Timeout: 500 * time.Millisecond},
	}
	mc := NewMultiChecker(opts)
	up, err := mc.CheckWithRetries(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if up {
		t.Error("expected down when one check fails, got up")
	}
}
