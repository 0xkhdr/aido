package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
)

const testKey = "sk-do-not-print-me-0123456789"

// project returns a Root under a fresh temp dir with .aido/ created, plus an
// optional .secrets.yaml body.
func project(t *testing.T, secrets string) Root {
	t.Helper()
	r := NewRoot(t.TempDir())
	if err := os.MkdirAll(r.String(), 0o755); err != nil {
		t.Fatal(err)
	}
	if secrets != "" {
		if err := os.WriteFile(r.SecretsPath(), []byte(secrets), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// withProvider returns a config naming one provider with the given key source.
func withProvider(name, source string) Config {
	return Config{LLM: LLMConfig{Providers: map[string]Provider{name: {APIKeySource: source}}}}
}

// R4.1: a set, non-empty environment variable wins over the secrets file.
func TestResolveKeyEnvWins(t *testing.T) {
	r := project(t, "openai_api_key: from-file\n")
	t.Setenv("OPENAI_API_KEY", testKey)
	c := withProvider("openai", "env:OPENAI_API_KEY")
	got, err := c.ResolveKey(r, "openai")
	if err != nil {
		t.Fatalf("ResolveKey() error = %v", err)
	}
	if got != testKey {
		t.Errorf("key = %q, want the environment value", got)
	}
}

// R4.2: set-but-empty is treated as unset and falls through to the file.
func TestResolveKeyEmptyEnvFallsThroughToFile(t *testing.T) {
	r := project(t, "openai_api_key: "+testKey+"\n")
	t.Setenv("OPENAI_API_KEY", "")
	c := withProvider("openai", "env:OPENAI_API_KEY")
	got, err := c.ResolveKey(r, "openai")
	if err != nil {
		t.Fatalf("ResolveKey() error = %v", err)
	}
	if got != testKey {
		t.Errorf("key = %q, want the file value", got)
	}
}

// R4.2 plus blueprint §4.4: nvidia_nim reads nvidia_api_key, not
// nvidia_nim_api_key.
func TestResolveKeyNvidiaKeyName(t *testing.T) {
	r := project(t, "nvidia_api_key: "+testKey+"\n")
	c := withProvider("nvidia_nim", "env:NVIDIA_API_KEY")
	t.Setenv("NVIDIA_API_KEY", "")
	got, err := c.ResolveKey(r, "nvidia_nim")
	if err != nil {
		t.Fatalf("ResolveKey() error = %v", err)
	}
	if got != testKey {
		t.Errorf("key = %q, want the file value under nvidia_api_key", got)
	}
}

// R4.3: an absent secrets file is a not-found condition, not an I/O error, and
// the error names the provider and every source consulted.
func TestResolveKeyMissingFileIsNotFound(t *testing.T) {
	r := project(t, "")
	t.Setenv("OPENAI_API_KEY", "")
	c := withProvider("openai", "env:OPENAI_API_KEY")
	_, err := c.ResolveKey(r, "openai")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("err = %v, want errors.Is(err, ErrKeyNotFound)", err)
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want a not-found condition distinct from an I/O error", err)
	}
	for _, want := range []string{"openai", "$OPENAI_API_KEY", r.SecretsPath()} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// R4.4: api_key_source none yields an empty key and no error.
func TestResolveKeyNoneSource(t *testing.T) {
	c := withProvider("ollama", "none")
	got, err := c.ResolveKey(project(t, ""), "ollama")
	if err != nil {
		t.Fatalf("ResolveKey() error = %v, want nil", err)
	}
	if got != "" {
		t.Errorf("key = %q, want empty", got)
	}
}

// A provider absent from config is a named error.
func TestResolveKeyUnknownProvider(t *testing.T) {
	c := withProvider("openai", "env:OPENAI_API_KEY")
	_, err := c.ResolveKey(project(t, ""), "anthropic")
	if err == nil {
		t.Fatal("ResolveKey() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "anthropic") {
		t.Errorf("error %q does not name the provider", err)
	}
}

// R4.5, invariant I1: an unparseable secrets file names the path and quotes no
// content from the file — yaml.v3's own error would have echoed the line.
func TestResolveKeyMalformedSecretsQuotesNothing(t *testing.T) {
	r := project(t, "openai_api_key: "+testKey+"\n  bad indent: [\n")
	t.Setenv("OPENAI_API_KEY", "")
	c := withProvider("openai", "env:OPENAI_API_KEY")
	_, err := c.ResolveKey(r, "openai")
	if err == nil {
		t.Fatal("ResolveKey() error = nil, want a parse error")
	}
	if !strings.Contains(err.Error(), r.SecretsPath()) {
		t.Errorf("error %q does not name the file", err)
	}
	if strings.Contains(err.Error(), testKey) || strings.Contains(err.Error(), "bad indent") {
		t.Errorf("error %q quotes file content", err)
	}
}

// Invariant I1, R4.5: no error produced by any resolution path contains the key
// literal, from either source.
func TestResolveKeyNeverLeaksKeyIntoErrors(t *testing.T) {
	r := project(t, "openai_api_key: "+testKey+"\n")
	t.Setenv("OPENAI_API_KEY", testKey)
	c := withProvider("openai", "env:OPENAI_API_KEY")
	// A successful resolution first, so the key is demonstrably reachable.
	if got, err := c.ResolveKey(r, "openai"); err != nil || got != testKey {
		t.Fatalf("ResolveKey() = %q, %v; want the key and no error", got, err)
	}
	// Then every failing path against the same populated sources.
	for _, provider := range []string{"anthropic", "mistral"} {
		if _, err := c.ResolveKey(r, provider); err != nil && strings.Contains(err.Error(), testKey) {
			t.Errorf("error for %s leaks the key: %q", provider, err)
		}
	}
}

// gitProject returns a Root inside a real git repository, optionally with
// .aido/.secrets.yaml git-ignored. The repository is created through go-git, so
// nothing here depends on a git binary (tech.md T3) and no case can be skipped.
func gitProject(t *testing.T, ignore bool) Root {
	t.Helper()
	dir := t.TempDir()
	if _, err := git.PlainInit(dir, false); err != nil {
		t.Fatalf("git init: %v", err)
	}
	r := NewRoot(dir)
	if err := os.MkdirAll(r.String(), 0o755); err != nil {
		t.Fatal(err)
	}
	if ignore {
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".aido/.secrets.yaml\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// R4.6, R5.4: the write happens only when git ignores the target, and lands 0600.
func TestWriteSecretsWritesWhenIgnored(t *testing.T) {
	r := gitProject(t, true)
	if err := WriteSecrets(r, map[string]string{"openai_api_key": testKey}); err != nil {
		t.Fatalf("WriteSecrets() = %v, want nil", err)
	}
	info, err := os.Stat(r.SecretsPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %#o, want 0600", got)
	}
	data, err := os.ReadFile(r.SecretsPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), testKey) {
		t.Errorf("file did not receive the key")
	}
}

// R4.6: an un-ignored target is refused, and nothing is written.
func TestWriteSecretsRefusesTrackedPath(t *testing.T) {
	r := gitProject(t, false)
	err := WriteSecrets(r, map[string]string{"openai_api_key": testKey})
	if !errors.Is(err, ErrNotGitIgnored) {
		t.Fatalf("err = %v, want errors.Is(err, ErrNotGitIgnored)", err)
	}
	if strings.Contains(err.Error(), testKey) {
		t.Errorf("refusal %q leaks the key", err)
	}
	if _, statErr := os.Stat(r.SecretsPath()); !os.IsNotExist(statErr) {
		t.Errorf("secrets file exists after a refused write")
	}
}

// R4.6: a tracked .secrets.yaml is refused even when a pattern would ignore it.
// Tracking beats ignoring in git, and a tracked secrets file is exactly the leak
// this guard exists to prevent.
func TestWriteSecretsRefusesTrackedFile(t *testing.T) {
	r := gitProject(t, true)
	if err := os.WriteFile(r.SecretsPath(), []byte("openai_api_key: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(filepath.Dir(r.String()))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	// --force, because the path is ignored: this reproduces the mistake of a
	// user who committed the file before the ignore rule existed.
	if err := tree.AddWithOptions(&git.AddOptions{Path: ".aido/.secrets.yaml"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteSecrets(r, map[string]string{"openai_api_key": testKey}); !errors.Is(err, ErrNotGitIgnored) {
		t.Fatalf("err = %v, want errors.Is(err, ErrNotGitIgnored) for a tracked file", err)
	}
	data, err := os.ReadFile(r.SecretsPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), testKey) {
		t.Error("the refused write reached the file")
	}
}

// R4.3: the file exists and parses, but holds no key for the provider — the
// path that reaches the final not-found return rather than an earlier one.
func TestResolveKeyPresentFileMissingProviderKey(t *testing.T) {
	r := project(t, "anthropic_api_key: "+testKey+"\n")
	t.Setenv("OPENAI_API_KEY", "")
	c := withProvider("openai", "env:OPENAI_API_KEY")
	got, err := c.ResolveKey(r, "openai")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("err = %v, want errors.Is(err, ErrKeyNotFound)", err)
	}
	if got != "" {
		t.Errorf("key = %q, want empty", got)
	}
	if !strings.Contains(err.Error(), r.SecretsPath()) {
		t.Errorf("error %q does not name the secrets file it consulted", err)
	}
	if strings.Contains(err.Error(), testKey) {
		t.Errorf("error %q leaks another provider's key", err)
	}
}

// R4.6: outside a git repository nothing is protecting the path, so the write
// is refused there too.
func TestWriteSecretsRefusesOutsideRepository(t *testing.T) {
	r := project(t, "")
	if err := WriteSecrets(r, map[string]string{"openai_api_key": testKey}); !errors.Is(err, ErrNotGitIgnored) {
		t.Fatalf("err = %v, want errors.Is(err, ErrNotGitIgnored)", err)
	}
}
