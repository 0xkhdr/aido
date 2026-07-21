package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
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

// isolateGitEnvironment points HOME and XDG_CONFIG_HOME at an empty temp dir,
// so no developer's real global git configuration reaches these tests.
//
// Without it the suite is not hermetic: an audit reproduced a failure by adding
// `.aido/` to ~/.config/git/ignore — the most sensible thing a user of this tool
// would do — and, worse, the test written to prove .git/info/exclude works
// passed via the global rule instead, proving nothing.
//
// Since gitIgnores stopped consulting machine-global sources entirely (see
// ignorePatterns), this is defence in depth rather than the load-bearing
// isolation it once was — TestWriteSecretsIgnoresProtectionOutsideTheRepository
// asserts that those sources are ignored on purpose, and
// TestIgnoreSourcesAreIsolated is the negative control that fails loudly if
// anything from the environment starts contributing patterns again.
func isolateGitEnvironment(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

// gitProject returns a Root inside a real git repository, optionally with
// .aido/.secrets.yaml git-ignored. The repository is created through go-git, so
// nothing here depends on a git binary (tech.md T3) and no case can be skipped.
func gitProject(t *testing.T, ignore bool) Root {
	t.Helper()
	isolateGitEnvironment(t)
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

// R4.6, AUDIT N1: .git/info/exclude counts. It is the canonical place to ignore
// a local tool directory without touching a shared .gitignore, and the previous
// implementation silently read none of it — go-git's worktree filesystem
// rejects a `.git` path component and ReadPatterns discards the error.
func TestWriteSecretsHonoursInfoExclude(t *testing.T) {
	r := gitProject(t, false) // no .gitignore at all
	info := filepath.Join(filepath.Dir(r.String()), ".git", "info")
	if err := os.MkdirAll(info, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(info, "exclude"), []byte("# local only\n.aido/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteSecrets(r, map[string]string{"openai_api_key": testKey}); err != nil {
		t.Fatalf("WriteSecrets() = %v, want nil for a path ignored via .git/info/exclude", err)
	}
	if _, err := os.Stat(r.SecretsPath()); err != nil {
		t.Errorf("secrets file not written: %v", err)
	}
}

// R4.6, AUDIT N7: a project reached through a symlinked parent is still inside
// its own worktree. Every macOS os.MkdirTemp produces exactly this shape.
func TestWriteSecretsThroughSymlinkedProjectPath(t *testing.T) {
	r := gitProject(t, true)
	realDir := filepath.Dir(r.String())
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	linked := NewRoot(link)
	if err := WriteSecrets(linked, map[string]string{"openai_api_key": testKey}); err != nil {
		t.Fatalf("WriteSecrets() = %v, want nil through a symlinked project path", err)
	}
	if _, err := os.Stat(filepath.Join(realDir, ".aido", ".secrets.yaml")); err != nil {
		t.Errorf("secrets file not written through the symlink: %v", err)
	}
}

// AUDIT NEW5: the refusal names its scope, so a user whose global ignore does
// cover .aido/ can tell a deliberate refusal from a bug.
func TestNotGitIgnoredMessageNamesRepositoryScope(t *testing.T) {
	message := ErrNotGitIgnored.Error()
	for _, want := range []string{"this repository", ".gitignore", ".git/info/exclude", "other clones"} {
		if !strings.Contains(message, want) {
			t.Errorf("refusal %q does not mention %q", message, want)
		}
	}
	if strings.Contains(message, testKey) {
		t.Error("refusal carries a key")
	}
}

// The negative control for the isolation above: with no ignore rule anywhere,
// the write must be refused. If a developer's global git config leaked in, this
// fails — which is what makes the positive tests meaningful.
func TestIgnoreSourcesAreIsolated(t *testing.T) {
	r := gitProject(t, false)
	if err := WriteSecrets(r, map[string]string{"openai_api_key": testKey}); !errors.Is(err, ErrNotGitIgnored) {
		t.Fatalf("err = %v, want ErrNotGitIgnored; an ignore rule reached the test from outside", err)
	}
}

// AUDIT NN1, R4.6: .aido/ not existing yet is the first-run case, not a
// security refusal. The ignore question is well defined for a file that has not
// been created; only the write itself can fail, and it must fail as a missing
// directory rather than as ErrNotGitIgnored.
func TestWriteSecretsMissingAidoDirIsNotARefusal(t *testing.T) {
	isolateGitEnvironment(t)
	dir := t.TempDir()
	if _, err := git.PlainInit(dir, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".aido/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRoot(dir) // .aido/ deliberately not created
	if _, err := os.Stat(r.String()); !os.IsNotExist(err) {
		t.Fatalf("precondition: .aido/ should not exist, got %v", err)
	}

	// The ignore decision itself must succeed and say "ignored".
	ignored, err := gitIgnores(dir, r.SecretsPath())
	if err != nil {
		t.Fatalf("gitIgnores() error = %v", err)
	}
	if !ignored {
		t.Error("gitIgnores() = false for a path .gitignore covers but that does not exist yet")
	}

	err = WriteSecrets(r, map[string]string{"openai_api_key": testKey})
	if err == nil {
		t.Fatal("WriteSecrets() = nil, want a missing-directory error")
	}
	if errors.Is(err, ErrNotGitIgnored) {
		t.Errorf("err = %v, want a missing-directory error, not a security refusal", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want it to wrap fs.ErrNotExist", err)
	}
}

// AUDIT NEW1, R4.6: protection that lives outside the repository does not
// count. Every machine-global ignore source is set here to cover .aido/ — the
// XDG default, an explicit core.excludesFile, and the repository-local
// .git/config key that git also honours — and the write must still be refused,
// because none of them protect the file in anyone else's clone.
//
// The previous version of this code consulted global sources and decided
// whether core.excludesFile was set by reading ~/.gitconfig alone. Each case
// below wrote the key to disk at a path git would happily add.
func TestWriteSecretsIgnoresProtectionOutsideTheRepository(t *testing.T) {
	cases := []struct {
		name  string
		setUp func(t *testing.T, home, projectDir string)
	}{
		{"XDG default ignore file", func(t *testing.T, home, _ string) {
			writeFileAt(t, filepath.Join(home, ".config", "git", "ignore"), ".aido/\n")
		}},
		{"core.excludesFile in ~/.gitconfig", func(t *testing.T, home, _ string) {
			writeFileAt(t, filepath.Join(home, "mine.ignore"), ".aido/\n")
			writeFileAt(t, filepath.Join(home, ".gitconfig"), "[core]\n\texcludesfile = ~/mine.ignore\n")
		}},
		{"core.excludesFile in ~/.config/git/config", func(t *testing.T, home, _ string) {
			writeFileAt(t, filepath.Join(home, "mine.ignore"), ".aido/\n")
			writeFileAt(t, filepath.Join(home, ".config", "git", "config"), "[core]\n\texcludesfile = "+filepath.Join(home, "mine.ignore")+"\n")
		}},
		{"core.excludesFile in the repository's .git/config", func(t *testing.T, home, projectDir string) {
			writeFileAt(t, filepath.Join(home, "mine.ignore"), ".aido/\n")
			writeFileAt(t, filepath.Join(projectDir, ".git", "config"),
				"[core]\n\trepositoryformatversion = 0\n\texcludesfile = "+filepath.Join(home, "mine.ignore")+"\n")
		}},
		{"core.excludesFile via an [include] directive", func(t *testing.T, home, _ string) {
			writeFileAt(t, filepath.Join(home, "mine.ignore"), ".aido/\n")
			writeFileAt(t, filepath.Join(home, "included.gitconfig"), "[core]\n\texcludesfile = "+filepath.Join(home, "mine.ignore")+"\n")
			writeFileAt(t, filepath.Join(home, ".gitconfig"), "[include]\n\tpath = "+filepath.Join(home, "included.gitconfig")+"\n")
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			isolateGitEnvironment(t)
			dir := t.TempDir()
			if _, err := git.PlainInit(dir, false); err != nil {
				t.Fatal(err)
			}
			r := NewRoot(dir)
			if err := os.MkdirAll(r.String(), 0o755); err != nil {
				t.Fatal(err)
			}
			tt.setUp(t, os.Getenv("HOME"), dir)

			err := WriteSecrets(r, map[string]string{"openai_api_key": testKey})
			if !errors.Is(err, ErrNotGitIgnored) {
				t.Fatalf("err = %v, want ErrNotGitIgnored: only repository-scoped ignore rules count", err)
			}
			if _, statErr := os.Stat(r.SecretsPath()); !os.IsNotExist(statErr) {
				t.Error("the key was written to disk despite the refusal")
			}
		})
	}
}

// writeFileAt writes body to path, creating parent directories.
func writeFileAt(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// AUDIT NN5, R4.6: a linked worktree has its own index but shares
// .git/info/exclude with the main checkout via $GIT_COMMON_DIR.
func TestWriteSecretsInLinkedWorktree(t *testing.T) {
	isolateGitEnvironment(t)
	main := t.TempDir()
	repo, err := git.PlainInit(main, false)
	if err != nil {
		t.Fatal(err)
	}
	// info/exclude lives in the main .git and must reach the linked worktree.
	info := filepath.Join(main, ".git", "info")
	if err := os.MkdirAll(info, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(info, "exclude"), []byte(".aido/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A commit is needed before a worktree can be linked.
	if err := os.WriteFile(filepath.Join(main, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Add("README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Commit("init", &git.CommitOptions{Author: &object.Signature{
		Name: "t", Email: "t@example.com", When: time.Now(),
	}}); err != nil {
		t.Fatal(err)
	}

	// go-git cannot create linked worktrees, so build one the way git does:
	// a .git *file* pointing at .git/worktrees/<name>, which carries commondir.
	linked := filepath.Join(t.TempDir(), "linked")
	gitDir := filepath.Join(main, ".git", "worktrees", "linked")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"commondir": "../..\n",
		"gitdir":    filepath.Join(linked, ".git") + "\n",
		"HEAD":      "ref: refs/heads/master\n",
	} {
		if err := os.WriteFile(filepath.Join(gitDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(linked, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := commonDir(gitDir); got != filepath.Join(main, ".git") {
		t.Errorf("commonDir() = %q, want the main git directory %q", got, filepath.Join(main, ".git"))
	}
	r := NewRoot(linked)
	if err := os.MkdirAll(r.String(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteSecrets(r, map[string]string{"openai_api_key": testKey}); err != nil {
		t.Fatalf("WriteSecrets() = %v, want nil: info/exclude from the common git dir applies", err)
	}

	// AUDIT NEW3: without an index in the linked worktree's own git directory,
	// trackedInIndex takes its early return and this fixture proves nothing
	// about per-worktree tracking. Write one naming the secrets file: tracking
	// beats ignoring, so the same write must now be refused — and it must be
	// *this* worktree's index that decides, not the main checkout's.
	writeIndexEntry(t, gitDir, ".aido/.secrets.yaml")
	if err := os.Remove(r.SecretsPath()); err != nil {
		t.Fatal(err)
	}
	if err := WriteSecrets(r, map[string]string{"openai_api_key": testKey}); !errors.Is(err, ErrNotGitIgnored) {
		t.Fatalf("err = %v, want ErrNotGitIgnored: the linked worktree's own index tracks the file", err)
	}
}

// writeIndexEntry writes a git index into gitDir containing one entry.
func writeIndexEntry(t *testing.T, gitDir, name string) {
	t.Helper()
	file, err := os.Create(filepath.Join(gitDir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	idx := &index.Index{Version: 2, Entries: []*index.Entry{{
		Name: name, Mode: filemode.Regular, Hash: plumbing.NewHash(strings.Repeat("a", 40)),
	}}}
	if err := index.NewEncoder(file).Encode(idx); err != nil {
		t.Fatal(err)
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

// AUDIT F6: an api_key_source that is neither "none" nor "env:NAME" used to
// fall through silently — the environment was skipped and the resulting error
// listed sources that had not been consulted. It is now a named refusal.
func TestResolveKeyUnsupportedSourceForm(t *testing.T) {
	for _, source := range []string{"keyring", "env", "ENV:OPENAI_API_KEY", "file:/etc/keys"} {
		t.Run(source, func(t *testing.T) {
			r := project(t, "openai_api_key: "+testKey+"\n")
			c := withProvider("openai", source)
			got, err := c.ResolveKey(r, "openai")
			if !errors.Is(err, ErrUnsupportedKeySource) {
				t.Fatalf("err = %v, want ErrUnsupportedKeySource", err)
			}
			if errors.Is(err, ErrKeyNotFound) {
				t.Error("an unreadable source is not the same condition as a key that is absent")
			}
			if got != "" {
				t.Errorf("key = %q, want empty", got)
			}
			if !strings.Contains(err.Error(), "openai") {
				t.Errorf("error %q does not name the provider", err)
			}
		})
	}
}

// I1, R4.5: a user who pastes a real key into api_key_source instead of a
// reference must not have it echoed back through the error. Checked with a
// distinctive value, since a short form like "env" appears in the message's own
// description of the expected syntax.
func TestResolveKeyUnsupportedSourceNeverEchoesTheValue(t *testing.T) {
	r := project(t, "")
	c := withProvider("openai", testKey)
	_, err := c.ResolveKey(r, "openai")
	if !errors.Is(err, ErrUnsupportedKeySource) {
		t.Fatalf("err = %v, want ErrUnsupportedKeySource", err)
	}
	if strings.Contains(err.Error(), testKey) {
		t.Errorf("error %q echoes the api_key_source value, which may be a pasted key", err)
	}
}

// An empty api_key_source is not an error: the secrets file is simply the only
// place left to look.
func TestResolveKeyEmptySourceUsesSecretsFile(t *testing.T) {
	r := project(t, "openai_api_key: "+testKey+"\n")
	c := withProvider("openai", "")
	got, err := c.ResolveKey(r, "openai")
	if err != nil {
		t.Fatalf("ResolveKey() error = %v, want nil", err)
	}
	if got != testKey {
		t.Errorf("key = %q, want the file value", got)
	}
}
