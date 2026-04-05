package access

import (
	"strings"
	"testing"
	"time"
)

func TestParseShellEnvOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKeys map[string]string
		skipKeys []string
	}{
		{
			name:  "standard KEY=VALUE lines",
			input: "HOME=/home/user\nPATH=/usr/bin\nSHELL=/bin/bash\n",
			wantKeys: map[string]string{
				"HOME":  "/home/user",
				"PATH":  "/usr/bin",
				"SHELL": "/bin/bash",
			},
		},
		{
			name:  "values containing equals signs",
			input: "API_KEY=sk-abc=def==\n",
			wantKeys: map[string]string{
				"API_KEY": "sk-abc=def==",
			},
		},
		{
			name:     "prompt noise filtered out",
			input:    "Last login: Mon Jan 1 00:00:00\n[user@host ~]$ \x1b[0m\nHOME=/home/user\n",
			wantKeys: map[string]string{"HOME": "/home/user"},
			skipKeys: []string{"Last login", "[user@host ~]$"},
		},
		{
			name:     "empty lines and no-equals lines skipped",
			input:    "\n\nno_equals_here\nVALID=yes\n\n",
			wantKeys: map[string]string{"VALID": "yes"},
		},
		{
			name:     "keys with spaces are filtered",
			input:    "NOT VALID=value\n_VALID_KEY=ok\n",
			wantKeys: map[string]string{"_VALID_KEY": "ok"},
			skipKeys: []string{"NOT VALID"},
		},
		{
			name:     "keys starting with numbers are filtered",
			input:    "1BAD=no\nGOOD=yes\n",
			wantKeys: map[string]string{"GOOD": "yes"},
			skipKeys: []string{"1BAD"},
		},
		{
			name:  "empty value is preserved",
			input: "EMPTY_VAR=\n",
			wantKeys: map[string]string{
				"EMPTY_VAR": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseShellEnvOutput([]byte(tt.input))

			for k, want := range tt.wantKeys {
				v, ok := got[k]
				if !ok {
					t.Errorf("expected key %q to be present", k)
					continue
				}
				if v != want {
					t.Errorf("key %q: expected %q, got %q", k, want, v)
				}
			}

			for _, k := range tt.skipKeys {
				if _, ok := got[k]; ok {
					t.Errorf("expected key %q to be filtered out", k)
				}
			}
		})
	}
}

func TestShellEnvResolver_LookupEnv_ProcessEnvFirst(t *testing.T) {
	t.Setenv("TEST_SHELLENV_PRECEDENCE", "from_process")

	r := &ShellEnvResolver{
		cache: map[string]string{
			"TEST_SHELLENV_PRECEDENCE": "from_cache",
		},
		loadedAt: time.Now(),
	}

	v, ok := r.LookupEnv("TEST_SHELLENV_PRECEDENCE")
	if !ok {
		t.Fatal("expected var to exist")
	}
	if v != "from_process" {
		t.Errorf("expected process env to win, got %q", v)
	}
}

func TestShellEnvResolver_LookupEnv_FallsBackToCache(t *testing.T) {
	r := &ShellEnvResolver{
		cache: map[string]string{
			"TEST_SHELLENV_ONLY_IN_CACHE": "cached_value",
		},
		loadedAt: time.Now(),
	}

	v, ok := r.LookupEnv("TEST_SHELLENV_ONLY_IN_CACHE")
	if !ok {
		t.Fatal("expected var to exist in cache")
	}
	if v != "cached_value" {
		t.Errorf("expected 'cached_value', got %q", v)
	}
}

func TestShellEnvResolver_LookupEnv_MissingEverywhere(t *testing.T) {
	r := &ShellEnvResolver{
		cache:    map[string]string{},
		loadedAt: time.Now(),
	}

	_, ok := r.LookupEnv("DEFINITELY_NOT_SET_SHELLENV_TEST_999")
	if ok {
		t.Fatal("expected var to not exist")
	}
}

func TestShellEnvResolver_LookupEnv_NilCache(t *testing.T) {
	r := &ShellEnvResolver{}

	// Should not panic, just fall back to process env.
	t.Setenv("TEST_SHELLENV_NIL_CACHE", "works")
	v, ok := r.LookupEnv("TEST_SHELLENV_NIL_CACHE")
	if !ok || v != "works" {
		t.Errorf("expected process env fallback, got %q, ok=%v", v, ok)
	}
}

func TestShellEnvResolver_ExpandEnv(t *testing.T) {
	t.Setenv("TEST_EXPAND_PROCESS", "/proc/val")

	r := &ShellEnvResolver{
		cache: map[string]string{
			"TEST_EXPAND_CACHE": "/cache/val",
		},
		loadedAt: time.Now(),
	}

	// Process var.
	got := r.ExpandEnv("$TEST_EXPAND_PROCESS")
	if got != "/proc/val" {
		t.Errorf("expected '/proc/val', got %q", got)
	}

	// Cache var.
	got = r.ExpandEnv("$TEST_EXPAND_CACHE")
	if got != "/cache/val" {
		t.Errorf("expected '/cache/val', got %q", got)
	}

	// Mixed.
	got = r.ExpandEnv("${TEST_EXPAND_PROCESS}:${TEST_EXPAND_CACHE}")
	if got != "/proc/val:/cache/val" {
		t.Errorf("expected '/proc/val:/cache/val', got %q", got)
	}
}

func TestShellEnvResolver_Environ_MergesCorrectly(t *testing.T) {
	t.Setenv("TEST_ENVIRON_CONFLICT", "process_wins")

	r := &ShellEnvResolver{
		cache: map[string]string{
			"TEST_ENVIRON_CONFLICT":  "cache_loses",
			"TEST_ENVIRON_CACHEONLY": "only_in_cache",
		},
		loadedAt: time.Now(),
	}

	env := r.Environ()
	envMap := make(map[string]string, len(env))
	for _, entry := range env {
		k, v, _ := strings.Cut(entry, "=")
		envMap[k] = v
	}

	if envMap["TEST_ENVIRON_CONFLICT"] != "process_wins" {
		t.Errorf("expected process env to win conflict, got %q", envMap["TEST_ENVIRON_CONFLICT"])
	}
	if envMap["TEST_ENVIRON_CACHEONLY"] != "only_in_cache" {
		t.Errorf("expected cache-only var to be present, got %q", envMap["TEST_ENVIRON_CACHEONLY"])
	}
}

func TestShellEnvResolver_RefreshCooldown(t *testing.T) {
	r := &ShellEnvResolver{
		cache:    map[string]string{"A": "1"},
		loadedAt: time.Now(),
		timeout:  1 * time.Second,
		cooldown: 30 * time.Second,
	}

	// Refresh within cooldown should be a no-op (no shell spawn).
	if err := r.Refresh(); err != nil {
		t.Fatalf("refresh within cooldown should not error: %v", err)
	}

	// Cache should be unchanged (no shell was spawned).
	r.mu.RLock()
	v, ok := r.cache["A"]
	r.mu.RUnlock()
	if !ok || v != "1" {
		t.Error("expected cache to be unchanged after cooldown skip")
	}
}

func TestShellEnvResolver_RefreshAfterCooldown(t *testing.T) {
	r := &ShellEnvResolver{
		cache:    map[string]string{"A": "1"},
		loadedAt: time.Now().Add(-60 * time.Second), // expired
		timeout:  5 * time.Second,
		cooldown: 30 * time.Second,
	}

	// Refresh after cooldown should re-spawn the shell.
	// On CI this may fail (no proper shell), but the loadedAt should update.
	_ = r.Refresh()

	r.mu.RLock()
	age := time.Since(r.loadedAt)
	r.mu.RUnlock()

	if age > 5*time.Second {
		t.Error("expected loadedAt to be updated after refresh")
	}
}
