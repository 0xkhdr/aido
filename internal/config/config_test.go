package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig lays down .aido/config.yaml under a fresh temp project and
// returns its Root.
func writeConfig(t *testing.T, body string) Root {
	t.Helper()
	root := NewRoot(t.TempDir())
	if err := os.MkdirAll(root.String(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root.ConfigPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// R2.2: absent config is a distinct, testable not-found condition, not a default.
func TestLoadMissingIsNotExist(t *testing.T) {
	got, err := Load(NewRoot(t.TempDir()))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want errors.Is(err, fs.ErrNotExist)", err)
	}
	if got != nil {
		t.Errorf("config = %+v, want nil (no default substituted)", got)
	}
}

// R2.4: an empty file is a valid parse to the zero config, not an error.
func TestLoadEmptyFile(t *testing.T) {
	c, err := Load(writeConfig(t, ""))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if c.Project != "" || c.RequiredDocs != nil || c.AutoSync {
		t.Errorf("config = %+v, want zero value", *c)
	}
}

// R2.3: a parse failure names the file and the position.
func TestLoadMalformedNamesFileAndPosition(t *testing.T) {
	root := writeConfig(t, "project: taxi\n  tracked_branch: main\n")
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() error = nil, want a parse error")
	}
	if !strings.Contains(err.Error(), root.ConfigPath()) {
		t.Errorf("error %q does not name the file %q", err, root.ConfigPath())
	}
	if !strings.Contains(err.Error(), "line ") {
		t.Errorf("error %q does not name a parse position", err)
	}
}

// R2.1 plus R2.4: declared keys land, omitted keys stay zero, and an unknown
// top-level key is ignored rather than fatal.
func TestLoadPopulatesAndIgnoresUnknown(t *testing.T) {
	c, err := Load(writeConfig(t, `project: taxi
tracked_branch: main
required_docs:
  - okf/architecture.md
llm:
  default_provider: openrouter
  providers:
    openrouter:
      api_key_source: env:OPENROUTER_API_KEY
      base_url: https://openrouter.ai/api/v1
    ollama:
      api_key_source: none
coding_agent:
  active: aider
  agents:
    aider:
      type: cli
      command: aider
future_key_this_spec_never_heard_of: 7
`))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if c.Project != "taxi" || c.TrackedBranch != "main" {
		t.Errorf("project/tracked_branch = %q/%q", c.Project, c.TrackedBranch)
	}
	if len(c.RequiredDocs) != 1 || c.RequiredDocs[0] != "okf/architecture.md" {
		t.Errorf("required_docs = %v", c.RequiredDocs)
	}
	if c.LLM.DefaultProvider != "openrouter" {
		t.Errorf("default_provider = %q", c.LLM.DefaultProvider)
	}
	if got := c.LLM.Providers["openrouter"].APIKeySource; got != "env:OPENROUTER_API_KEY" {
		t.Errorf("openrouter api_key_source = %q", got)
	}
	if got := c.LLM.Providers["ollama"].BaseURL; got != "" {
		t.Errorf("ollama base_url = %q, want zero value for an omitted key", got)
	}
	if c.CodingAgent.Agents["aider"].Command != "aider" {
		t.Errorf("coding_agent.agents.aider.command = %q", c.CodingAgent.Agents["aider"].Command)
	}
	// Omitted optional keys stay zero (R2.4).
	if c.LastSyncCommit != "" || c.LLM.DefaultModel != "" || c.AutoSync {
		t.Errorf("omitted keys did not stay zero: %+v", *c)
	}
}

// I3: Load reads; it never creates .aido/ or anything under it.
func TestLoadCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(NewRoot(dir)); err == nil {
		t.Fatal("Load() on an empty project error = nil, want not-exist")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, filepath.Join(dir, e.Name()))
		}
		t.Errorf("Load created %v, want an untouched directory", names)
	}
}
