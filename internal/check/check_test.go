package check

import "testing"

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
