package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestShouldRetryTransportErrorBadRecordMAC(t *testing.T) {
	cfg := Config{}
	err := fmt.Errorf("local error: tls: bad record MAC")
	if !shouldRetryTransportError(err, cfg) {
		t.Fatal("expected bad record MAC to be retryable")
	}
}

func TestDoOpenAIRequestRetriesOnTransportError(t *testing.T) {
	orig := openAIHTTPClient
	t.Cleanup(func() {
		openAIHTTPClient = orig
	})

	attempts := 0
	closeFlags := make([]bool, 0, 2)
	openAIHTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			closeFlags = append(closeFlags, req.Close)
			if attempts == 1 {
				return nil, fmt.Errorf("local error: tls: bad record MAC")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Request:    req,
			}, nil
		}),
	}

	cfg := Config{
		APIBase: "https://api.boaiak.com/v1",
		APIKey:  "sk-test",
		Retry: RetryConfig{
			Enabled:        true,
			MaxAttempts:    2,
			InitialDelayMs: 1,
			MaxDelayMs:     1,
		},
	}

	resp, err := doOpenAIRequest(context.Background(), cfg, []byte(`{"model":"x"}`))
	if err != nil {
		t.Fatalf("doOpenAIRequest returned error: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if len(closeFlags) < 2 {
		t.Fatalf("expected 2 close flags, got %d", len(closeFlags))
	}
	if closeFlags[0] {
		t.Fatal("first attempt should not force close connection")
	}
	if !closeFlags[1] {
		t.Fatal("retry attempt should force close connection")
	}
}
