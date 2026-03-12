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
