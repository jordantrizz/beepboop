package check

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHTTPCheckerFollowsRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			http.Redirect(writer, request, "/ready", http.StatusFound)
		case "/ready":
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	checker := NewChecker(Options{
		Mode:    ModeHTTP,
		Target:  server.URL,
		Timeout: 2 * time.Second,
	})

	up, err := checker.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !up {
		t.Fatal("expected target to be up after redirect")
	}
}

func TestHTTPCheckerRedirectLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		countText := request.URL.Query().Get("count")
		count, _ := strconv.Atoi(countText)
		if count < 12 {
			next := "/?count=" + strconv.Itoa(count+1)
			http.Redirect(writer, request, next, http.StatusFound)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewChecker(Options{
		Mode:    ModeHTTP,
		Target:  server.URL + "/?count=0",
		Timeout: 2 * time.Second,
	})

	up, err := checker.CheckOnce(context.Background())
	if err == nil {
		t.Fatal("expected redirect limit error")
	}
	if up {
		t.Fatal("expected target to be down on redirect error")
	}
	if !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPCheckerExpectedStatusAfterRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			http.Redirect(writer, request, "/accepted", http.StatusMovedPermanently)
		case "/accepted":
			writer.WriteHeader(http.StatusAccepted)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	checker := NewChecker(Options{
		Mode:    ModeHTTP,
		Target:  server.URL,
		Timeout: 2 * time.Second,
		ExpectedStatuses: map[int]struct{}{
			http.StatusAccepted: {},
		},
	})

	up, err := checker.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !up {
		t.Fatal("expected accepted status to be treated as up")
	}
}
