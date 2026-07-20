package workspace

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializePreservesKnowledgeAndReportsReadiness(t *testing.T) {
	repo := gitRepo(t)
	sentinel := filepath.Join(repo, ".aido", "docs", "architecture.md")
	mustWrite(t, sentinel, "human knowledge\n")

	proposal, err := Initialize(repo, Options{Project: "demo"})
	if err != nil || proposal.ProposedBranch != "main" {
		t.Fatalf("proposal: %#v, %v", proposal, err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".aido", "config.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("mutated before confirmation")
	}

	first, err := Initialize(repo, Options{Project: "demo", BranchConfirmed: true, DefaultProvider: "ollama", DefaultModel: "llama", CodingAgent: "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Ready || len(first.Missing) != len(DefaultRequiredDocs)-1 {
		t.Fatalf("unexpected readiness: %#v", first)
	}
	data, _ := os.ReadFile(sentinel)
	if string(data) != "human knowledge\n" {
		t.Fatal("existing knowledge overwritten")
	}
	config, _ := os.ReadFile(filepath.Join(repo, ".aido", "config.yaml"))
	if strings.Contains(string(config), "secret") || !strings.Contains(string(config), `default_provider: "ollama"`) {
		t.Fatalf("unsafe/incomplete config: %s", config)
	}

	second, err := Initialize(repo, Options{Project: "changed", BranchConfirmed: true})
	if err != nil || !second.Ready || second.Revision == "" {
		t.Fatalf("second init: %#v, %v", second, err)
	}
	after, _ := os.ReadFile(filepath.Join(repo, ".aido", "config.yaml"))
	if !strings.Contains(string(after), `project: "demo"`) || !strings.Contains(string(after), `last_sync_commit: "`+second.Revision+`"`) {
		t.Fatalf("config not safely advanced: %s", after)
	}
}

func TestInitializeRejectsNonGitBeforeMutation(t *testing.T) {
	dir := t.TempDir()
	if _, err := Initialize(dir, Options{BranchConfirmed: true}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(filepath.Join(dir, ".aido")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created state outside Git")
	}
}

func TestInitializeRejectsMalformedConfigBeforeMutation(t *testing.T) {
	repo := gitRepo(t)
	config := filepath.Join(repo, ".aido", "config.yaml")
	mustWrite(t, config, "not: an aido config\n")
	if _, err := Initialize(repo, Options{BranchConfirmed: true}); err == nil || !strings.Contains(err.Error(), "invalid existing") {
		t.Fatalf("expected actionable config error, got %v", err)
	}
	data, _ := os.ReadFile(config)
	if string(data) != "not: an aido config\n" {
		t.Fatal("malformed config changed")
	}
	if _, err := os.Stat(filepath.Join(repo, ".aido", "docs")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("workspace mutated after malformed config")
	}
}

func TestCredentialPrecedenceConsentAndLocalProvider(t *testing.T) {
	repo := gitRepo(t)
	mustWrite(t, filepath.Join(repo, ".aido", ".gitignore"), ".secrets.yaml\n")
	mustWrite(t, filepath.Join(repo, ".aido", ".secrets.yaml"), "remote: file-value\n")
	t.Setenv("REMOTE_KEY", "environment-value")
	value, err := ResolveCredential(repo, CredentialRequest{Provider: "remote", EnvironmentVariable: "REMOTE_KEY", RequiresCredential: true}, nil)
	if err != nil || value != "environment-value" {
		t.Fatalf("environment precedence: %q %v", value, err)
	}
	t.Setenv("REMOTE_KEY", "")
	value, err = ResolveCredential(repo, CredentialRequest{Provider: "remote", RequiresCredential: true}, nil)
	if err != nil || value != "file-value" {
		t.Fatalf("file fallback: %q %v", value, err)
	}
	value, err = ResolveCredential(repo, CredentialRequest{Provider: "ollama"}, func(string) (string, bool, error) { t.Fatal("prompted local provider"); return "", false, nil })
	if err != nil || value != "" {
		t.Fatalf("local provider: %q %v", value, err)
	}

	value, err = ResolveCredential(repo, CredentialRequest{Provider: "new", RequiresCredential: true}, func(string) (string, bool, error) { return "prompt-value", true, nil })
	if err != nil || value != "prompt-value" {
		t.Fatalf("prompt: %q %v", value, err)
	}
	secret, _ := os.ReadFile(filepath.Join(repo, ".aido", ".secrets.yaml"))
	if string(secret) != "remote: file-value\nnew: prompt-value\n" {
		t.Fatalf("stored secret: %s", secret)
	}
	if _, err = ResolveCredential(repo, CredentialRequest{Provider: "cancelled", RequiresCredential: true}, func(string) (string, bool, error) { return "", false, errors.New("cancelled") }); err == nil {
		t.Fatal("expected cancellation")
	}
}

func TestCredentialSaveRequiresIgnoredStore(t *testing.T) {
	repo := gitRepo(t)
	value, err := ResolveCredential(repo, CredentialRequest{Provider: "remote", RequiresCredential: true}, func(string) (string, bool, error) {
		return "must-not-write", true, nil
	})
	if err == nil || value != "" || !strings.Contains(err.Error(), "not Git-ignored") {
		t.Fatalf("expected ignored-store error, got %q, %v", value, err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".aido", ".secrets.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("credential store mutated")
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	mustWrite(t, filepath.Join(dir, "README.md"), "test\n")
	cmd := exec.Command("git", "-C", dir, "add", "README.md")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	cmd = exec.Command("git", "-C", dir, "commit", "-m", "initial")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	return dir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
