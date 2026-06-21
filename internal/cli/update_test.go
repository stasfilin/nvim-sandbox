package cli

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewerSemanticVersion(t *testing.T) {
	tests := []struct {
		candidate string
		current   string
		want      bool
	}{
		{candidate: "0.2.0", current: "0.1.9", want: true},
		{candidate: "v0.1.2", current: "0.1.1", want: true},
		{candidate: "1.0.0", current: "0.99.99", want: true},
		{candidate: "0.1.1", current: "0.1.1", want: false},
		{candidate: "0.1.0", current: "0.1.1", want: false},
		{candidate: "nightly", current: "0.1.1", want: false},
		{candidate: "0.2", current: "0.1.1", want: false},
	}
	for _, test := range tests {
		if got := newerSemanticVersion(test.candidate, test.current); got != test.want {
			t.Errorf("newerSemanticVersion(%q, %q) = %t, want %t", test.candidate, test.current, got, test.want)
		}
	}
}

func TestCheckForUpdateFetchesAndCachesLatestRelease(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if got := request.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q", got)
		}
		if got := request.Header.Get("X-GitHub-Api-Version"); got != "2026-03-10" {
			t.Errorf("X-GitHub-Api-Version = %q", got)
		}
		if got := request.Header.Get("User-Agent"); got != "nvim-sandbox/0.1.1" {
			t.Errorf("User-Agent = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"tag_name":"v0.2.0","html_url":"https://github.com/stasfilin/nvim-sandbox/releases/tag/v0.2.0"}`,
			)),
		}, nil
	})}

	stateBase := t.TempDir()
	now := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	first := checkForUpdateWith(client, "https://api.example.test/latest", "0.1.1", stateBase, now)
	if first.LatestVersion != "0.2.0" {
		t.Fatalf("first update = %#v", first)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}

	second := checkForUpdateWith(client, "https://api.example.test/latest", "0.1.1", stateBase, now.Add(time.Hour))
	if second != first {
		t.Fatalf("cached update = %#v, want %#v", second, first)
	}
	if requests != 1 {
		t.Fatalf("cached check made %d requests, want 1", requests)
	}
	if _, err := os.Stat(filepath.Join(stateBase, "update-check.json")); err != nil {
		t.Fatalf("cache file: %v", err)
	}
}

func TestCheckForUpdateFallsBackToStaleCacheWhenOffline(t *testing.T) {
	stateBase := t.TempDir()
	now := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	cache := updateCache{
		LatestVersion: "0.3.0",
		ReleaseURL:    "https://example.test/v0.3.0",
		CheckedAt:     now.Add(-48 * time.Hour),
	}
	if err := writeUpdateCache(filepath.Join(stateBase, "update-check.json"), cache); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, os.ErrDeadlineExceeded
	})}
	info := checkForUpdateWith(client, "https://example.test/latest", "0.1.1", stateBase, now)
	if info.LatestVersion != "0.3.0" {
		t.Fatalf("offline update = %#v", info)
	}
}

func TestUpdateCheckCanBeDisabled(t *testing.T) {
	t.Setenv("NVIM_SANDBOX_NO_UPDATE_CHECK", "true")
	if info := checkForUpdate("0.1.1", t.TempDir()); info != (updateInfo{}) {
		t.Fatalf("disabled update check = %#v", info)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
