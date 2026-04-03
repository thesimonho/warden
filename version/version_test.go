package version

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"newer patch", "v0.5.2", "v0.5.3", true},
		{"newer minor", "v0.5.2", "v0.6.0", true},
		{"newer major", "v0.5.2", "v1.0.0", true},
		{"same version", "v0.5.2", "v0.5.2", false},
		{"older version", "v0.6.0", "v0.5.2", false},
		{"no v prefix", "0.5.2", "0.5.3", true},
		{"mixed prefix", "v0.5.2", "0.5.3", true},
		{"invalid current", "dev", "v0.5.3", false},
		{"invalid latest", "v0.5.2", "invalid", false},
		{"both invalid", "foo", "bar", false},
		{"pre-release stripped", "v0.5.2", "v0.6.0-beta.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNewer(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestCheckLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Error("missing Accept header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://github.com/thesimonho/warden/releases/tag/v1.2.3"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	original := releaseURL
	releaseURL = srv.URL
	defer func() { releaseURL = original }()

	latest, pageURL, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest != "v1.2.3" {
		t.Errorf("latest = %q, want %q", latest, "v1.2.3")
	}
	if pageURL != "https://github.com/thesimonho/warden/releases/tag/v1.2.3" {
		t.Errorf("pageURL = %q, want release URL", pageURL)
	}
}

func TestCheckLatestServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	original := releaseURL
	releaseURL = srv.URL
	defer func() { releaseURL = original }()

	_, _, err := CheckLatest(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestCheckLatestNetworkError(t *testing.T) {
	original := releaseURL
	releaseURL = "http://localhost:1" // connection refused
	defer func() { releaseURL = original }()

	_, _, err := CheckLatest(context.Background())
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
		ok    bool
	}{
		{"v1.2.3", [3]int{1, 2, 3}, true},
		{"0.5.2", [3]int{0, 5, 2}, true},
		{"v0.6.0-beta.1", [3]int{0, 6, 0}, true},
		{"dev", [3]int{}, false},
		{"1.2", [3]int{}, false},
		{"a.b.c", [3]int{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseSemver(tt.input)
			if ok != tt.ok {
				t.Errorf("parseSemver(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("parseSemver(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
