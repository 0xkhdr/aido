// Package workspace manages Aido's repository-local workspace.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var DefaultRequiredDocs = []string{
	"docs/architecture.md", "docs/domain-model.md", "docs/glossary.md",
	"docs/api-contracts.md", "docs/operations.md",
}

type Options struct {
	Project, TrackedBranch, DefaultProvider, DefaultModel, CodingAgent string
	BranchConfirmed                                                    bool
	RequiredDocs                                                       []string
}

type Result struct {
	Root, ProposedBranch, Revision string
	Missing                        []string
	Ready                          bool
}

// Initialize creates only absent repository-local knowledge. The first run
// reports documents that were absent even though it creates usable templates.
func Initialize(repo string, options Options) (Result, error) {
	root, err := git(repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return Result{}, fmt.Errorf("initialize Aido: %q is not a Git repository", repo)
	}
	branch, err := git(root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		branch = strings.TrimPrefix(branch, "origin/")
	} else if branch, err = git(root, "branch", "--show-current"); err != nil || branch == "" {
		return Result{}, errors.New("initialize Aido: cannot detect the default branch")
	}
	result := Result{Root: root, ProposedBranch: branch}
	if !options.BranchConfirmed {
		return result, nil
	}
	if options.TrackedBranch == "" {
		options.TrackedBranch = branch
	}
	if options.Project == "" {
		options.Project = filepath.Base(root)
	}
	if len(options.RequiredDocs) == 0 {
		options.RequiredDocs = append([]string(nil), DefaultRequiredDocs...)
	}
	configPath := filepath.Join(root, ".aido", "config.yaml")
	if data, readErr := os.ReadFile(configPath); readErr == nil {
		if err := validateConfig(data); err != nil {
			return result, err
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return result, fmt.Errorf("read config: %w", readErr)
	}
	for _, rel := range options.RequiredDocs {
		path, err := workspacePath(root, rel)
		if err != nil {
			return result, err
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			result.Missing = append(result.Missing, rel)
		} else if err != nil {
			return result, fmt.Errorf("inspect %s: %w", rel, err)
		}
	}
	entries := map[string]string{
		".gitignore":        ".secrets.yaml\n",
		"links.yaml":        "{}\n",
		"templates/ears.md": earsTemplate,
	}
	for _, rel := range result.Missing {
		entries[rel] = documentTemplate(rel)
	}
	for rel, content := range entries {
		if err := createAbsent(filepath.Join(root, ".aido", filepath.FromSlash(rel)), content); err != nil {
			return result, err
		}
	}
	for _, dir := range []string{"requests", "witness", "docs/adr"} {
		if err := os.MkdirAll(filepath.Join(root, ".aido", dir), 0o755); err != nil {
			return result, fmt.Errorf("create workspace: %w", err)
		}
	}
	if len(result.Missing) == 0 {
		result.Revision, err = git(root, "rev-parse", "HEAD")
		if err != nil {
			return result, fmt.Errorf("record revision: %w", err)
		}
		result.Ready = true
	}
	config := renderConfig(options, result.Revision)
	if err := createAbsent(configPath, config); err != nil {
		return result, err
	}
	if result.Ready {
		if err := recordRevision(configPath, result.Revision); err != nil {
			return result, err
		}
	}
	return result, nil
}

type CredentialRequest struct {
	Provider, EnvironmentVariable string
	RequiresCredential            bool
}

type PromptCredential func(provider string) (value string, save bool, err error)

// ResolveCredential applies environment, ignored secret file, then consensual prompt precedence.
func ResolveCredential(repo string, request CredentialRequest, prompt PromptCredential) (string, error) {
	if !request.RequiresCredential {
		return "", nil
	}
	if value := os.Getenv(request.EnvironmentVariable); value != "" {
		return value, nil
	}
	secretPath := filepath.Join(repo, ".aido", ".secrets.yaml")
	if data, err := os.ReadFile(secretPath); err == nil {
		if value := secretValue(string(data), request.Provider); value != "" {
			return value, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read credential store: %w", err)
	}
	if prompt == nil {
		return "", fmt.Errorf("credential required for provider %s", request.Provider)
	}
	value, save, err := prompt(request.Provider)
	if err != nil {
		return "", fmt.Errorf("credential prompt for provider %s: %w", request.Provider, err)
	}
	if value == "" {
		return "", fmt.Errorf("credential required for provider %s", request.Provider)
	}
	if save {
		if _, err := git(repo, "check-ignore", "--quiet", ".aido/.secrets.yaml"); err != nil {
			return "", errors.New("refusing to store credential: .aido/.secrets.yaml is not Git-ignored")
		}
		data, err := os.ReadFile(secretPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read credential store: %w", err)
		}
		if secretValue(string(data), request.Provider) == "" {
			data = append(data, []byte(request.Provider+": "+value+"\n")...)
		}
		if err := atomicWrite(secretPath, data, 0o600); err != nil {
			return "", fmt.Errorf("store credential: %w", err)
		}
	}
	return value, nil
}

func git(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func workspacePath(root, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("required document escapes workspace: %s", rel)
	}
	return filepath.Join(root, ".aido", clean), nil
}

func createAbsent(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err = f.WriteString(content); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return closeErr
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".aido-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	return err
}

func renderConfig(o Options, revision string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "project: %q\ntracked_branch: %q\nlast_sync_commit: %q\nrequired_docs:\n", o.Project, o.TrackedBranch, revision)
	for _, doc := range o.RequiredDocs {
		fmt.Fprintf(&b, "  - %q\n", filepath.ToSlash(doc))
	}
	fmt.Fprintf(&b, "llm:\n  default_provider: %q\n  default_model: %q\ncoding_agent:\n  active: %q\nauto_sync: false\n", o.DefaultProvider, o.DefaultModel, o.CodingAgent)
	return b.String()
}

func validateConfig(data []byte) error {
	text := string(data)
	for _, field := range []string{"project:", "tracked_branch:", "last_sync_commit:", "required_docs:", "llm:", "coding_agent:", "auto_sync:"} {
		if !hasConfigLine(text, field) {
			return fmt.Errorf("invalid existing .aido/config.yaml: missing %s", field)
		}
	}
	return nil
}

func hasConfigLine(text, field string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, field) {
			return true
		}
	}
	return false
}

func recordRevision(path, revision string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	old := `last_sync_commit: ""`
	if !strings.Contains(string(data), old) {
		return nil
	}
	updated := strings.Replace(string(data), old, fmt.Sprintf("last_sync_commit: %q", revision), 1)
	if err := atomicWrite(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("record revision: %w", err)
	}
	return nil
}

func secretValue(data, provider string) string {
	for _, line := range strings.Split(data, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == provider {
			return strings.Trim(strings.TrimSpace(value), "\"'")
		}
	}
	return ""
}

func documentTemplate(rel string) string {
	return "# " + strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)) + "\n\nDescribe this project document.\n"
}

const earsTemplate = `# Request specification

## Context
## Domain Analysis
## Related Documents
## Specification (EARS)
## Open Questions
## Implementation Notes
`
