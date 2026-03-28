package runtime

import (
	"context"
	"testing"
)

func TestSocketCandidates_Docker(t *testing.T) {
	t.Parallel()

	candidates := socketCandidates(RuntimeDocker)
	if len(candidates) == 0 {
		t.Fatal("expected at least one Docker socket candidate")
	}
}

func TestSocketCandidates_Podman(t *testing.T) {
	t.Parallel()

	candidates := socketCandidates(RuntimePodman)
	if len(candidates) == 0 {
		t.Fatal("expected at least one Podman socket candidate")
	}
}

func TestSocketCandidates_Unknown(t *testing.T) {
	t.Parallel()

	candidates := socketCandidates(Runtime("unknown"))
	if candidates != nil {
		t.Errorf("expected nil for unknown runtime, got %v", candidates)
	}
}

func TestSocketCandidates_DockerHostEnv(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://localhost:2375")

	candidates := socketCandidates(RuntimeDocker)
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

func TestProbeBinary_UnknownRuntime(t *testing.T) {
	t.Parallel()

	_, err := probeBinary(context.Background(), Runtime("nonexistent-runtime"))
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestRuntimeConstants(t *testing.T) {
	t.Parallel()

	if RuntimeDocker != "docker" {
		t.Errorf("expected 'docker', got %s", RuntimeDocker)
	}
	if RuntimePodman != "podman" {
		t.Errorf("expected 'podman', got %s", RuntimePodman)
	}
}
