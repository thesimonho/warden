package docker

import (
	"context"
	"testing"
)

func TestSocketCandidates(t *testing.T) {
	t.Parallel()

	candidates := socketCandidates()
	if len(candidates) == 0 {
		t.Fatal("expected at least one Docker socket candidate")
	}
}

func TestSocketCandidates_DockerHostEnv(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://localhost:2375")

	candidates := socketCandidates()
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	if candidates[0] != "tcp://localhost:2375" {
		t.Errorf("expected DOCKER_HOST first, got %s", candidates[0])
	}
}

func TestSocketHost_UnixPath(t *testing.T) {
	t.Parallel()

	got := SocketHost("/var/run/docker.sock")
	if got != "unix:///var/run/docker.sock" {
		t.Errorf("expected unix:///var/run/docker.sock, got %s", got)
	}
}

func TestSocketHost_NamedPipe(t *testing.T) {
	t.Parallel()

	got := SocketHost("//./pipe/docker_engine")
	if got != "npipe:////./pipe/docker_engine" {
		t.Errorf("expected npipe:////./pipe/docker_engine, got %s", got)
	}
}

func TestSocketHost_ExistingScheme(t *testing.T) {
	t.Parallel()

	tests := []string{
		"unix:///var/run/docker.sock",
		"npipe:////./pipe/docker_engine",
		"tcp://localhost:2375",
	}
	for _, input := range tests {
		got := SocketHost(input)
		if got != input {
			t.Errorf("expected passthrough %q, got %q", input, got)
		}
	}
}

func TestProbeBinary_Nonexistent(t *testing.T) {
	// Override PATH to ensure docker binary isn't found.
	// Cannot use t.Parallel with t.Setenv.
	t.Setenv("PATH", "")
	_, err := probeBinary(context.Background())
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestDetect_PopulatesName(t *testing.T) {
	t.Parallel()

	info := Detect(context.Background())
	if info.Name != Name {
		t.Errorf("expected Name %q, got %q", Name, info.Name)
	}
}
