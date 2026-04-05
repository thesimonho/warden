package access

import (
	"os"
	"testing"
)

func TestProcessEnvResolver_LookupEnv(t *testing.T) {
	t.Setenv("TEST_PROCESS_RESOLVER", "hello")

	r := ProcessEnvResolver{}
	v, ok := r.LookupEnv("TEST_PROCESS_RESOLVER")
	if !ok {
		t.Fatal("expected env var to exist")
	}
	if v != "hello" {
		t.Errorf("expected 'hello', got %q", v)
	}

	_, ok = r.LookupEnv("DEFINITELY_UNSET_PROCESS_RESOLVER_TEST")
	if ok {
		t.Fatal("expected env var to not exist")
	}
}

func TestProcessEnvResolver_ExpandEnv(t *testing.T) {
	t.Setenv("TEST_EXPAND_RESOLVER", "/tmp/sock")

	r := ProcessEnvResolver{}
	got := r.ExpandEnv("$TEST_EXPAND_RESOLVER")
	if got != "/tmp/sock" {
		t.Errorf("expected '/tmp/sock', got %q", got)
	}
}

func TestProcessEnvResolver_Environ(t *testing.T) {
	r := ProcessEnvResolver{}
	env := r.Environ()
	if len(env) == 0 {
		t.Fatal("expected non-empty environment")
	}

	// Verify it matches os.Environ length.
	osEnv := os.Environ()
	if len(env) != len(osEnv) {
		t.Errorf("expected %d entries, got %d", len(osEnv), len(env))
	}
}
