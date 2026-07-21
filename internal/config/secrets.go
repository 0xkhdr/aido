package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"gopkg.in/yaml.v3"
)

// ErrKeyNotFound reports that no source held a key for a provider. It is
// distinct from an I/O error so a caller can tell "you have not set this up"
// from "your disk is broken" (R4.3).
var ErrKeyNotFound = errors.New("api key not found")

// ErrNotGitIgnored reports a refusal to write a resolved key to a path git
// would track (R4.6).
var ErrNotGitIgnored = errors.New("refusing to write a key to a path that is not git-ignored")

// secretsKeys maps a provider name to its key in .aido/.secrets.yaml. Only
// nvidia_nim differs from <provider>_api_key (blueprint §4.4).
var secretsKeys = map[string]string{"nvidia_nim": "nvidia_api_key"}

// secretsKey is the .secrets.yaml key holding provider's API key.
func secretsKey(provider string) string {
	if k, ok := secretsKeys[provider]; ok {
		return k
	}
	return provider + "_api_key"
}

// ResolveKey returns the API key for provider, consulting the environment
// variable named by api_key_source first and .aido/.secrets.yaml second
// (blueprint §4.5 steps 1-2). Keyring and interactive prompting are non-goals.
//
// Invariant I1: no error returned here contains the key, any substring of it,
// or any content read from .secrets.yaml. Errors name providers, variables, and
// paths only.
func (c *Config) ResolveKey(r Root, provider string) (string, error) {
	p, ok := c.LLM.Providers[provider]
	if !ok {
		return "", fmt.Errorf("provider %s has no entry under llm.providers: %w", provider, ErrKeyNotFound)
	}
	// R4.4: an explicitly keyless provider (ollama) is not a failure.
	if p.APIKeySource == "none" {
		return "", nil
	}
	consulted := make([]string, 0, 2)
	if envName, ok := strings.CutPrefix(p.APIKeySource, "env:"); ok {
		consulted = append(consulted, "$"+envName)
		// R4.2: set-but-empty is treated as unset and falls through.
		if v := os.Getenv(envName); v != "" {
			return v, nil
		}
	}
	path := r.SecretsPath()
	consulted = append(consulted, path)
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// R4.3: an absent secrets file is not-found, not an I/O failure.
		return "", fmt.Errorf("%w for provider %s (consulted %s)", ErrKeyNotFound, provider, strings.Join(consulted, ", "))
	case err != nil:
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var secrets map[string]string
	if err := yaml.Unmarshal(data, &secrets); err != nil {
		// The yaml error is deliberately NOT wrapped: it quotes the offending
		// line, which is a key value (I1, R4.5). The message is rebuilt.
		return "", fmt.Errorf("parse %s: file is not valid YAML", path)
	}
	if v := secrets[secretsKey(provider)]; v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%w for provider %s (consulted %s)", ErrKeyNotFound, provider, strings.Join(consulted, ", "))
}

// WriteSecrets writes .aido/.secrets.yaml at mode 0600 (R5.4), and only after
// confirming git ignores it (R4.6). Both guards are refusals, not warnings: a
// key written to a tracked path is a leak the moment anyone commits.
//
// It is the only function in this package that writes a key anywhere.
func WriteSecrets(r Root, secrets map[string]string) error {
	path := r.SecretsPath()
	// The project directory, not .aido/ itself: .aido/ need not exist yet, and
	// repository discovery must start from a directory that does.
	ignored, err := gitIgnores(filepath.Dir(string(r)), path)
	if err != nil {
		return err
	}
	if !ignored {
		return fmt.Errorf("%w: %s", ErrNotGitIgnored, path)
	}
	data, err := yaml.Marshal(secrets)
	if err != nil {
		// Reached only if a value is unmarshalable, which for map[string]string
		// it is not — but the error must still never carry the map.
		return fmt.Errorf("encode secrets for %s", path)
	}
	return WriteFile(path, data, 0o600)
}

// gitIgnores reports whether git ignores path, resolving the repository from
// projectDir upwards.
//
// It uses go-git's format packages rather than the git binary (`tech.md` T3
// refuses a runtime that requires `git` on PATH) and deliberately not go-git's
// root package, which pulls net/http and crypto/tls into this package's
// dependency graph for a check that touches only local files.
//
// Every source git consults is consulted here, in git's own precedence order:
// /etc/gitconfig's core.excludesFile, then the user's global excludes, then
// .git/info/exclude, then the repository's .gitignore files. An earlier
// version delegated to gitignore.ReadPatterns alone, which silently dropped
// .git/info/exclude because go-git's worktree filesystem rejects a `.git` path
// component and ReadPatterns discards that error.
//
// Three cases deliberately report false — "nothing is protecting this path":
//   - projectDir is not inside a repository with a worktree;
//   - path lies outside that worktree;
//   - the file is already tracked in the index, which in git beats any ignore
//     rule. A tracked .secrets.yaml is the leak R4.6 exists to prevent.
func gitIgnores(projectDir, path string) (bool, error) {
	worktree, gitDir, ok := findRepository(projectDir)
	if !ok {
		return false, nil
	}
	// Resolve symlinks on both sides before relating them: a project reached
	// through a symlinked parent (every macOS os.MkdirTemp, for one) otherwise
	// looks like it lies outside its own worktree.
	rel, err := relativeTo(worktree, path)
	if err != nil || rel == "" {
		return false, err
	}
	tracked, err := trackedInIndex(gitDir, rel)
	if err != nil {
		return false, err
	}
	if tracked {
		return false, nil
	}
	patterns, err := ignorePatterns(worktree, gitDir)
	if err != nil {
		return false, err
	}
	return gitignore.NewMatcher(patterns).Match(strings.Split(rel, "/"), false), nil
}

// findRepository walks up from dir to the worktree root, returning it and the
// git directory. A `.git` file (submodule or linked worktree) is followed.
func findRepository(dir string) (worktree, gitDir string, ok bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", "", false
	}
	for {
		candidate := filepath.Join(dir, ".git")
		info, err := os.Stat(candidate)
		switch {
		case err == nil && info.IsDir():
			return dir, candidate, true
		case err == nil:
			// `gitdir: <path>`, absolute or relative to the worktree.
			data, readErr := os.ReadFile(candidate)
			if readErr != nil {
				return "", "", false
			}
			target := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(data)), "gitdir:"))
			if target == "" {
				return "", "", false
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(dir, target)
			}
			return dir, filepath.Clean(target), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}
}

// relativeTo returns path relative to worktree in slash form, or "" when path
// lies outside it. Both sides are symlink-resolved first.
func relativeTo(worktree, path string) (string, error) {
	resolvedTree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", worktree, err)
	}
	// The target itself need not exist yet — resolve its directory instead.
	resolvedDir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		// A missing parent directory cannot be inside anything.
		return "", nil
	}
	rel, err := filepath.Rel(resolvedTree, filepath.Join(resolvedDir, filepath.Base(path)))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", nil
	}
	return filepath.ToSlash(rel), nil
}

// trackedInIndex reports whether rel has an entry in the repository index.
func trackedInIndex(gitDir, rel string) (bool, error) {
	file, err := os.Open(filepath.Join(gitDir, "index"))
	if errors.Is(err, fs.ErrNotExist) {
		// A repository with no index yet tracks nothing.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open git index: %w", err)
	}
	defer file.Close()
	var idx index.Index
	if err := index.NewDecoder(file).Decode(&idx); err != nil {
		return false, fmt.Errorf("decode git index: %w", err)
	}
	for _, entry := range idx.Entries {
		if entry.Name == rel {
			return true, nil
		}
	}
	return false, nil
}

// ignorePatterns collects every ignore source git would consult, in ascending
// precedence order (the matcher takes the last match).
func ignorePatterns(worktree, gitDir string) ([]gitignore.Pattern, error) {
	var patterns []gitignore.Pattern
	root := osfs.New(string(filepath.Separator))
	if system, err := gitignore.LoadSystemPatterns(root); err == nil {
		patterns = append(patterns, system...)
	}
	if global, err := gitignore.LoadGlobalPatterns(root); err == nil {
		patterns = append(patterns, global...)
	}
	// git's default global excludes file when core.excludesFile is unset.
	if home, err := os.UserHomeDir(); err == nil {
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(home, ".config")
		}
		patterns = append(patterns, parseExcludeFile(filepath.Join(xdg, "git", "ignore"))...)
	}
	// .git/info/exclude — read directly, because go-git's worktree filesystem
	// refuses a `.git` path component and swallows the resulting error.
	patterns = append(patterns, parseExcludeFile(filepath.Join(gitDir, "info", "exclude"))...)
	repo, err := gitignore.ReadPatterns(osfs.New(worktree), nil)
	if err != nil {
		return nil, fmt.Errorf("read ignore patterns under %s: %w", worktree, err)
	}
	return append(patterns, repo...), nil
}

// parseExcludeFile reads one gitignore-syntax file. A file that does not exist
// or cannot be read contributes nothing: these are advisory sources, and git
// treats an unreadable one the same way.
func parseExcludeFile(path string) []gitignore.Pattern {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var patterns []gitignore.Pattern
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			patterns = append(patterns, gitignore.ParsePattern(line, nil))
		}
	}
	return patterns
}
