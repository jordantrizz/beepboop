package main

import (
	"testing"
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
