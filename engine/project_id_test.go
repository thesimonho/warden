package engine

import (
	"testing"
)

func TestProjectID_Deterministic(t *testing.T) {
	t.Parallel()

	id1, err := ProjectID("/home/user/project")
	if err != nil {
		t.Fatalf("ProjectID() error: %v", err)
	}
	id2, err := ProjectID("/home/user/project")
	if err != nil {
		t.Fatalf("ProjectID() error: %v", err)
	}

	if id1 != id2 {
		t.Errorf("same path produced different IDs: %q vs %q", id1, id2)
	}
}

func TestProjectID_Length(t *testing.T) {
	t.Parallel()

	id, err := ProjectID("/home/user/project")
	if err != nil {
		t.Fatalf("ProjectID() error: %v", err)
	}

	if len(id) != projectIDLength {
		t.Errorf("expected %d chars, got %d: %q", projectIDLength, len(id), id)
	}
}

func TestProjectID_LowercaseHex(t *testing.T) {
	t.Parallel()

	id, err := ProjectID("/home/user/project")
	if err != nil {
		t.Fatalf("ProjectID() error: %v", err)
	}

	if !ValidProjectID(id) {
		t.Errorf("ProjectID output %q is not valid lowercase hex", id)
	}
}

func TestProjectID_DifferentPaths(t *testing.T) {
	t.Parallel()

	id1, _ := ProjectID("/home/user/project-a")
	id2, _ := ProjectID("/home/user/project-b")

	if id1 == id2 {
		t.Errorf("different paths produced same ID: %q", id1)
	}
}

func TestProjectID_TrailingSlashNormalized(t *testing.T) {
	t.Parallel()

	id1, _ := ProjectID("/home/user/project")
	id2, _ := ProjectID("/home/user/project/")

	if id1 != id2 {
		t.Errorf("trailing slash produced different ID: %q vs %q", id1, id2)
	}
}

func TestProjectID_CleansDotSegments(t *testing.T) {
	t.Parallel()

	id1, _ := ProjectID("/home/user/project")
	id2, _ := ProjectID("/home/user/./project")
	id3, _ := ProjectID("/home/user/other/../project")

	if id1 != id2 {
		t.Errorf("dot segment produced different ID: %q vs %q", id1, id2)
	}
	if id1 != id3 {
		t.Errorf("dotdot segment produced different ID: %q vs %q", id1, id3)
	}
}

func TestProjectID_RelativePathError(t *testing.T) {
	t.Parallel()

	_, err := ProjectID("relative/path")
	if err == nil {
		t.Fatal("expected error for relative path")
	}
}

func TestProjectID_RootPath(t *testing.T) {
	t.Parallel()

	id, err := ProjectID("/")
	if err != nil {
		t.Fatalf("ProjectID() error: %v", err)
	}
	if !ValidProjectID(id) {
		t.Errorf("root path produced invalid ID: %q", id)
	}
}

func TestValidProjectID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"valid", "a1b2c3d4e5f6", true},
		{"all zeros", "000000000000", true},
		{"all f", "ffffffffffff", true},
		{"too short", "a1b2c3", false},
		{"too long", "a1b2c3d4e5f6a7", false},
		{"uppercase", "A1B2C3D4E5F6", false},
		{"non-hex", "g1b2c3d4e5f6", false},
		{"empty", "", false},
		{"with spaces", "a1b2 3d4e5f6", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ValidProjectID(tt.id); got != tt.valid {
				t.Errorf("ValidProjectID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}
