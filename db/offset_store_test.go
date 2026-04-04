package db

import "testing"

func TestTailerOffset_RoundTrip(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck

	if err := store.SaveTailerOffset("proj1", "claude-code", "/path/a.jsonl", 42); err != nil {
		t.Fatal(err)
	}

	offset, err := store.LoadTailerOffset("proj1", "claude-code", "/path/a.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if offset != 42 {
		t.Errorf("expected 42, got %d", offset)
	}
}

func TestTailerOffset_NotFound(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck

	offset, err := store.LoadTailerOffset("proj1", "claude-code", "/nonexistent.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if offset != 0 {
		t.Errorf("expected 0 for missing key, got %d", offset)
	}
}

func TestTailerOffset_Upsert(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck

	if err := store.SaveTailerOffset("proj1", "claude-code", "/path/a.jsonl", 10); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTailerOffset("proj1", "claude-code", "/path/a.jsonl", 99); err != nil {
		t.Fatal(err)
	}

	offset, err := store.LoadTailerOffset("proj1", "claude-code", "/path/a.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if offset != 99 {
		t.Errorf("expected 99 after upsert, got %d", offset)
	}
}

func TestTailerOffset_DeleteSingle(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck

	_ = store.SaveTailerOffset("proj1", "claude-code", "/path/a.jsonl", 42)
	_ = store.SaveTailerOffset("proj1", "claude-code", "/path/b.jsonl", 84)

	if err := store.DeleteTailerOffset("proj1", "claude-code", "/path/a.jsonl"); err != nil {
		t.Fatal(err)
	}

	// Deleted key returns 0.
	offset, _ := store.LoadTailerOffset("proj1", "claude-code", "/path/a.jsonl")
	if offset != 0 {
		t.Errorf("expected 0 after delete, got %d", offset)
	}

	// Other key untouched.
	offset, _ = store.LoadTailerOffset("proj1", "claude-code", "/path/b.jsonl")
	if offset != 84 {
		t.Errorf("expected 84 for undeleted key, got %d", offset)
	}
}

func TestTailerOffset_DeleteAll(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck

	_ = store.SaveTailerOffset("proj1", "claude-code", "/path/a.jsonl", 10)
	_ = store.SaveTailerOffset("proj1", "claude-code", "/path/b.jsonl", 20)
	_ = store.SaveTailerOffset("proj2", "codex", "/path/c.jsonl", 30)

	if err := store.DeleteTailerOffsets("proj1", "claude-code"); err != nil {
		t.Fatal(err)
	}

	// proj1/claude-code keys are gone.
	a, _ := store.LoadTailerOffset("proj1", "claude-code", "/path/a.jsonl")
	b, _ := store.LoadTailerOffset("proj1", "claude-code", "/path/b.jsonl")
	if a != 0 || b != 0 {
		t.Errorf("expected 0 for deleted project, got a=%d b=%d", a, b)
	}

	// proj2/codex key is untouched.
	c, _ := store.LoadTailerOffset("proj2", "codex", "/path/c.jsonl")
	if c != 30 {
		t.Errorf("expected 30 for other project, got %d", c)
	}
}

func TestTailerOffset_NilStore(t *testing.T) {
	t.Parallel()
	var store *Store

	// All methods should be no-ops on nil store.
	offset, err := store.LoadTailerOffset("proj", "agent", "/path.jsonl")
	if err != nil || offset != 0 {
		t.Errorf("nil Load: offset=%d err=%v", offset, err)
	}
	if err := store.SaveTailerOffset("proj", "agent", "/path.jsonl", 42); err != nil {
		t.Errorf("nil Save: err=%v", err)
	}
	if err := store.DeleteTailerOffset("proj", "agent", "/path.jsonl"); err != nil {
		t.Errorf("nil Delete: err=%v", err)
	}
	if err := store.DeleteTailerOffsets("proj", "agent"); err != nil {
		t.Errorf("nil DeleteAll: err=%v", err)
	}
}

func TestOffsetStoreAdapter_ImplementsInterface(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck

	adapter := &OffsetStoreAdapter{Store: store}

	// Save via adapter.
	if err := adapter.SaveOffset("proj1", "claude-code", "/path/a.jsonl", 55); err != nil {
		t.Fatal(err)
	}

	// Load via adapter.
	offset, err := adapter.LoadOffset("proj1", "claude-code", "/path/a.jsonl")
	if err != nil || offset != 55 {
		t.Errorf("adapter Load: offset=%d err=%v", offset, err)
	}

	// Delete via adapter.
	if err := adapter.DeleteOffset("proj1", "claude-code", "/path/a.jsonl"); err != nil {
		t.Fatal(err)
	}

	offset, _ = adapter.LoadOffset("proj1", "claude-code", "/path/a.jsonl")
	if offset != 0 {
		t.Errorf("expected 0 after adapter delete, got %d", offset)
	}
}
