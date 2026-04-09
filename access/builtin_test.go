package access

import (
	"testing"
)

func TestBuiltInItems_UniqueIDs(t *testing.T) {
	items := BuiltInItems()
	seen := make(map[string]bool)
	for _, item := range items {
		if seen[item.ID] {
			t.Errorf("duplicate built-in ID: %s", item.ID)
		}
		seen[item.ID] = true
	}
}

func TestBuiltInItems_WellFormed(t *testing.T) {
	for _, item := range BuiltInItems() {
		if item.ID == "" {
			t.Error("built-in item has empty ID")
		}
		if item.Label == "" {
			t.Errorf("built-in %q has empty label", item.ID)
		}
		if item.Description == "" {
			t.Errorf("built-in %q has empty description", item.ID)
		}
		if item.Method != MethodTransport {
			t.Errorf("built-in %q has unexpected method: %s", item.ID, item.Method)
		}
		if !item.BuiltIn {
			t.Errorf("built-in %q has BuiltIn=false", item.ID)
		}
		if len(item.Credentials) == 0 {
			t.Errorf("built-in %q has no credentials", item.ID)
		}

		for _, cred := range item.Credentials {
			if cred.Label == "" {
				t.Errorf("built-in %q has credential with empty label", item.ID)
			}
			// Sources may be empty on platforms where the credential is
			// intentionally unavailable (e.g. SSH agent on Windows).
			// Injections may also be empty in that case.
		}
	}
}

func TestBuiltInItemByID(t *testing.T) {
	git := BuiltInItemByID(BuiltInIDGit)
	if git == nil {
		t.Fatal("expected to find git built-in")
	}
	if git.ID != BuiltInIDGit {
		t.Errorf("expected ID %q, got %q", BuiltInIDGit, git.ID)
	}

	ssh := BuiltInItemByID(BuiltInIDSSH)
	if ssh == nil {
		t.Fatal("expected to find ssh built-in")
	}

	gpg := BuiltInItemByID(BuiltInIDGPG)
	if gpg == nil {
		t.Fatal("expected to find gpg built-in")
	}

	unknown := BuiltInItemByID("nonexistent")
	if unknown != nil {
		t.Error("expected nil for unknown ID")
	}
}

func TestIsBuiltInID(t *testing.T) {
	if !IsBuiltInID(BuiltInIDGit) {
		t.Error("expected git to be built-in")
	}
	if !IsBuiltInID(BuiltInIDSSH) {
		t.Error("expected ssh to be built-in")
	}
	if !IsBuiltInID(BuiltInIDGPG) {
		t.Error("expected gpg to be built-in")
	}
	if IsBuiltInID("custom") {
		t.Error("expected custom to not be built-in")
	}
}

func TestBuiltInGit_CredentialStructure(t *testing.T) {
	git := BuiltInGit()

	if len(git.Credentials) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(git.Credentials))
	}

	cred := git.Credentials[0]
	if len(cred.Sources) != 2 {
		t.Errorf("expected 2 sources (gitconfig fallbacks), got %d", len(cred.Sources))
	}
	if cred.Sources[0].Type != SourceFilePath {
		t.Errorf("expected file source, got %s", cred.Sources[0].Type)
	}
	if cred.Transform == nil || cred.Transform.Type != TransformGitInclude {
		t.Error("expected git_include transform")
	}
	if len(cred.Injections) != 1 || cred.Injections[0].Type != InjectionMountFile {
		t.Error("expected single mount_file injection")
	}
}

func TestBuiltInGPG_CredentialStructure(t *testing.T) {
	gpg := BuiltInGPG()

	if len(gpg.Credentials) != 1 {
		t.Fatalf("expected 1 credential (agent), got %d", len(gpg.Credentials))
	}

	agent := gpg.Credentials[0]
	if agent.Label != "GPG Agent" {
		t.Errorf("expected GPG Agent label, got %q", agent.Label)
	}
	if agent.Transform != nil {
		t.Error("expected no transform on GPG agent credential")
	}
}

func TestBuiltInSSH_CredentialStructure(t *testing.T) {
	ssh := BuiltInSSH()

	if len(ssh.Credentials) != 3 {
		t.Fatalf("expected 3 credentials (config, known_hosts, agent), got %d", len(ssh.Credentials))
	}

	// SSH Config credential has strip_lines transform.
	config := ssh.Credentials[0]
	if config.Transform == nil || config.Transform.Type != TransformStripLines {
		t.Error("expected strip_lines transform on SSH config credential")
	}

	// Known hosts has no transform.
	knownHosts := ssh.Credentials[1]
	if knownHosts.Transform != nil {
		t.Error("expected no transform on known_hosts credential")
	}

	// SSH agent credential — platform-specific (see ssh_agent_*.go).
	agent := ssh.Credentials[2]
	if agent.Label != "SSH Agent" {
		t.Errorf("expected SSH Agent label, got %q", agent.Label)
	}
}
