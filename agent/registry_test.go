package agent

import (
	"sync"
	"testing"
)

// mockProvider implements StatusProvider for testing.
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string                                    { return m.name }
func (m *mockProvider) ProcessName() string                             { return m.name }
func (m *mockProvider) ConfigFilePath() string                          { return "" }
func (m *mockProvider) ExtractStatus([]byte) map[string]*Status         { return nil }
func (m *mockProvider) NewSessionParser() SessionParser                 { return nil }

func TestNewRegistry_Empty(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	p, ok := r.Get("anything")
	if ok || p != nil {
		t.Errorf("expected (nil, false), got (%v, %v)", p, ok)
	}
	if d := r.Default(); d != nil {
		t.Errorf("expected nil default, got %v", d)
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	provider := &mockProvider{name: "claude-code"}
	r.Register(ClaudeCode, provider)

	got, ok := r.Get(ClaudeCode)
	if !ok || got != provider {
		t.Errorf("Get(%q) = (%v, %v), want (%v, true)", ClaudeCode, got, ok, provider)
	}

	got, ok = r.Get("unknown")
	if ok || got != nil {
		t.Errorf("Get(unknown) = (%v, %v), want (nil, false)", got, ok)
	}
}

func TestRegistry_Default(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	provider := &mockProvider{name: "claude-code"}
	r.Register(ClaudeCode, provider)

	if d := r.Default(); d != provider {
		t.Errorf("Default() = %v, want %v", d, provider)
	}
}

func TestRegistry_Resolve_ExactMatch(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	claude := &mockProvider{name: "claude-code"}
	codex := &mockProvider{name: "codex"}
	r.Register(ClaudeCode, claude)
	r.Register(Codex, codex)

	if got := r.Resolve(Codex); got != codex {
		t.Errorf("Resolve(%q) = %v, want %v", Codex, got, codex)
	}
	if got := r.Resolve(ClaudeCode); got != claude {
		t.Errorf("Resolve(%q) = %v, want %v", ClaudeCode, got, claude)
	}
}

func TestRegistry_Resolve_EmptyFallsToDefault(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	claude := &mockProvider{name: "claude-code"}
	r.Register(ClaudeCode, claude)

	if got := r.Resolve(""); got != claude {
		t.Errorf("Resolve(\"\") = %v, want %v (default)", got, claude)
	}
}

func TestRegistry_Resolve_UnknownFallsToDefault(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	claude := &mockProvider{name: "claude-code"}
	r.Register(ClaudeCode, claude)

	if got := r.Resolve("aider"); got != claude {
		t.Errorf("Resolve(\"aider\") = %v, want %v (default)", got, claude)
	}
}

func TestRegistry_Resolve_NothingRegistered(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	if got := r.Resolve(Codex); got != nil {
		t.Errorf("Resolve(%q) = %v, want nil", Codex, got)
	}
	if got := r.Resolve(""); got != nil {
		t.Errorf("Resolve(\"\") = %v, want nil", got)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations.
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			p := &mockProvider{name: "provider"}
			r.Register(ClaudeCode, p)
			r.Get(ClaudeCode)
			r.Resolve(Codex)
			r.Default()
			_ = n
		}(i)
	}
	wg.Wait()

	// Should not panic or deadlock.
	if _, ok := r.Get(ClaudeCode); !ok {
		t.Error("expected provider to be registered after concurrent writes")
	}
}

func TestAllTypes(t *testing.T) {
	t.Parallel()

	if len(AllTypes) != 2 {
		t.Fatalf("AllTypes has %d entries, want 2", len(AllTypes))
	}
	if AllTypes[0] != ClaudeCode {
		t.Errorf("AllTypes[0] = %q, want %q", AllTypes[0], ClaudeCode)
	}
	if AllTypes[1] != Codex {
		t.Errorf("AllTypes[1] = %q, want %q", AllTypes[1], Codex)
	}
}

func TestDisplayLabels(t *testing.T) {
	t.Parallel()

	if got := DisplayLabels[ClaudeCode]; got != "Claude Code" {
		t.Errorf("DisplayLabels[%q] = %q, want %q", ClaudeCode, got, "Claude Code")
	}
	if got := DisplayLabels[Codex]; got != "OpenAI Codex" {
		t.Errorf("DisplayLabels[%q] = %q, want %q", Codex, got, "OpenAI Codex")
	}
}

func TestShortLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{ClaudeCode, "claude"},
		{Codex, "codex"},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := ShortLabel(tt.input); got != tt.want {
			t.Errorf("ShortLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()

	if ClaudeCode != "claude-code" {
		t.Errorf("ClaudeCode = %q, want %q", ClaudeCode, "claude-code")
	}
	if Codex != "codex" {
		t.Errorf("Codex = %q, want %q", Codex, "codex")
	}
	if DefaultAgentType != ClaudeCode {
		t.Errorf("DefaultAgentType = %q, want %q", DefaultAgentType, ClaudeCode)
	}
}
