package access

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitIncludePaths_SingleInclude(t *testing.T) {
	content := "[include]\n\tpath = ~/.config/git/identity\n"
	paths := ParseGitIncludePaths(content)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0] != "~/.config/git/identity" {
		t.Errorf("expected ~/.config/git/identity, got %q", paths[0])
	}
}

func TestParseGitIncludePaths_MultipleIncludes(t *testing.T) {
	content := `[include]
	path = ~/.config/git/identity-personal

[includeIf "gitdir:~/work/"]
	path = ~/.config/git/identity-work
`
	paths := ParseGitIncludePaths(content)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0] != "~/.config/git/identity-personal" {
		t.Errorf("expected identity-personal, got %q", paths[0])
	}
	if paths[1] != "~/.config/git/identity-work" {
		t.Errorf("expected identity-work, got %q", paths[1])
	}
}

func TestParseGitIncludePaths_IncludeIfHasConfig(t *testing.T) {
	content := `[user]
	name = Simon Ho

[includeIf "hasconfig:remote.*.url:git@github.com:*/**"]
	path = ~/.config/git/identity-personal

[includeIf "hasconfig:remote.*.url:git@work-github.com:*/**"]
	path = ~/.config/git/identity-work
`
	paths := ParseGitIncludePaths(content)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0] != "~/.config/git/identity-personal" {
		t.Errorf("expected identity-personal, got %q", paths[0])
	}
	if paths[1] != "~/.config/git/identity-work" {
		t.Errorf("expected identity-work, got %q", paths[1])
	}
}

func TestParseGitIncludePaths_RelativePath(t *testing.T) {
	content := "[include]\n\tpath = ../shared/identity\n"
	paths := ParseGitIncludePaths(content)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0] != "../shared/identity" {
		t.Errorf("expected ../shared/identity, got %q", paths[0])
	}
}

func TestParseGitIncludePaths_AbsolutePath(t *testing.T) {
	content := "[include]\n\tpath = /home/simon/.config/git/identity\n"
	paths := ParseGitIncludePaths(content)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0] != "/home/simon/.config/git/identity" {
		t.Errorf("expected /home/simon/.config/git/identity, got %q", paths[0])
	}
}

func TestParseGitIncludePaths_QuotedPath(t *testing.T) {
	content := "[include]\n\tpath = \"~/path with spaces/identity\"\n"
	paths := ParseGitIncludePaths(content)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0] != "~/path with spaces/identity" {
		t.Errorf("expected path with spaces, got %q", paths[0])
	}
}

func TestParseGitIncludePaths_NoIncludes(t *testing.T) {
	content := `[user]
	name = Test
	email = test@example.com

[core]
	autocrlf = input
`
	paths := ParseGitIncludePaths(content)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestParseGitIncludePaths_EmptyFile(t *testing.T) {
	paths := ParseGitIncludePaths("")
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestParseGitIncludePaths_PathNotInIncludeSection(t *testing.T) {
	// path = ... lines outside include sections should be ignored.
	content := `[core]
	path = /some/path
[include]
	path = ~/.config/git/real-include
[filter "lfs"]
	path = /another/path
`
	paths := ParseGitIncludePaths(content)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0] != "~/.config/git/real-include" {
		t.Errorf("expected ~/.config/git/real-include, got %q", paths[0])
	}
}

func TestParseGitIncludePaths_IncludeIfVariants(t *testing.T) {
	content := `[includeIf "gitdir:~/projects/"]
	path = ~/projects.gitconfig

[includeIf "gitdir/i:C:/Users/"]
	path = ~/windows.gitconfig

[includeIf "onbranch:release"]
	path = ~/release.gitconfig
`
	paths := ParseGitIncludePaths(content)
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
}

func TestRewriteGitIncludePaths_Basic(t *testing.T) {
	content := `[user]
	name = Simon Ho

[includeIf "hasconfig:remote.*.url:git@github.com:*/**"]
	path = ~/.config/git/identity-personal
`
	pathMap := map[string]string{
		"~/.config/git/identity-personal": "/home/warden/.gitconfig.d/identity-personal",
	}

	result := RewriteGitIncludePaths(content, pathMap)
	expected := `[user]
	name = Simon Ho

[includeIf "hasconfig:remote.*.url:git@github.com:*/**"]
	path = /home/warden/.gitconfig.d/identity-personal
`
	if result != expected {
		t.Errorf("unexpected rewrite result:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestRewriteGitIncludePaths_PreservesOtherContent(t *testing.T) {
	content := `[user]
	name = Test
	email = test@example.com

[include]
	path = ~/identity

[core]
	autocrlf = input
`
	pathMap := map[string]string{
		"~/identity": "/home/warden/.gitconfig.d/identity",
	}

	result := RewriteGitIncludePaths(content, pathMap)

	// Verify non-include content is preserved.
	if !strings.Contains(result,"\tname = Test") {
		t.Error("user.name was not preserved")
	}
	if !strings.Contains(result,"\temail = test@example.com") {
		t.Error("user.email was not preserved")
	}
	if !strings.Contains(result,"\tautocrlf = input") {
		t.Error("core.autocrlf was not preserved")
	}
	if !strings.Contains(result,"\tpath = /home/warden/.gitconfig.d/identity") {
		t.Error("include path was not rewritten")
	}
}

func TestRewriteGitIncludePaths_MultipleRewrites(t *testing.T) {
	content := `[includeIf "hasconfig:remote.*.url:git@github.com:*/**"]
	path = ~/personal

[includeIf "hasconfig:remote.*.url:git@work.com:*/**"]
	path = ~/work
`
	pathMap := map[string]string{
		"~/personal": "/home/warden/.gitconfig.d/personal",
		"~/work":     "/home/warden/.gitconfig.d/work",
	}

	result := RewriteGitIncludePaths(content, pathMap)
	if !strings.Contains(result,"\tpath = /home/warden/.gitconfig.d/personal") {
		t.Error("personal path was not rewritten")
	}
	if !strings.Contains(result,"\tpath = /home/warden/.gitconfig.d/work") {
		t.Error("work path was not rewritten")
	}
}

func TestRewriteGitIncludePaths_EmptyMap(t *testing.T) {
	content := "[include]\n\tpath = ~/identity\n"
	result := RewriteGitIncludePaths(content, nil)
	if result != content {
		t.Errorf("expected unchanged content with nil map, got %q", result)
	}
}

func TestRewriteGitIncludePaths_UnmappedPathUnchanged(t *testing.T) {
	content := "[include]\n\tpath = ~/identity\n"
	pathMap := map[string]string{
		"~/other": "/home/warden/.gitconfig.d/other",
	}
	result := RewriteGitIncludePaths(content, pathMap)
	if result != content {
		t.Errorf("expected unchanged content for unmapped path, got %q", result)
	}
}

func TestRewriteGitIncludePaths_QuotedPath(t *testing.T) {
	content := "[include]\n\tpath = \"~/my identity\"\n"
	pathMap := map[string]string{
		"~/my identity": "/home/warden/.gitconfig.d/my-identity",
	}
	result := RewriteGitIncludePaths(content, pathMap)
	if !strings.Contains(result,"\tpath = \"/home/warden/.gitconfig.d/my-identity\"") {
		t.Errorf("quoted path was not rewritten correctly: %q", result)
	}
}

func TestResolveIncludePath_Tilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	result := ResolveIncludePath("~/.config/git/identity", "/some/dir")
	expected := home + "/.config/git/identity"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveIncludePath_Absolute(t *testing.T) {
	result := ResolveIncludePath("/home/user/.config/git/id", "/some/dir")
	if result != "/home/user/.config/git/id" {
		t.Errorf("expected absolute path unchanged, got %q", result)
	}
}

func TestResolveIncludePath_Relative(t *testing.T) {
	result := ResolveIncludePath("../shared/identity", "/home/user/.config/git")
	expected := filepath.Join("/home/user/.config/git", "../shared/identity")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

