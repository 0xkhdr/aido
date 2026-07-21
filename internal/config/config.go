package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is .aido/config.yaml (blueprint §4.3). Every key is optional at parse
// time; Validate decides what is required (R2.4, R3).
type Config struct {
	Project        string            `yaml:"project"`
	TrackedBranch  string            `yaml:"tracked_branch"`
	LastSyncCommit string            `yaml:"last_sync_commit"`
	RequiredDocs   []string          `yaml:"required_docs"`
	LLM            LLMConfig         `yaml:"llm"`
	CodingAgent    CodingAgentConfig `yaml:"coding_agent"`
	AutoSync       bool              `yaml:"auto_sync"`
}

// LLMConfig is the llm: block.
type LLMConfig struct {
	DefaultProvider string              `yaml:"default_provider"`
	DefaultModel    string              `yaml:"default_model"`
	Providers       map[string]Provider `yaml:"providers"`
	Tasks           map[string]string   `yaml:"tasks"`
}

// Provider is one entry under llm.providers. APIKeySource is a source
// *reference* only — never a key (product.md P7).
type Provider struct {
	APIKeySource string `yaml:"api_key_source"`
	BaseURL      string `yaml:"base_url"`
}

// CodingAgentConfig is parsed and preserved, but unused by this spec.
type CodingAgentConfig struct {
	Active string                `yaml:"active"`
	Agents map[string]AgentEntry `yaml:"agents"`
}

// AgentEntry is one entry under coding_agent.agents.
type AgentEntry struct {
	Type          string `yaml:"type"`
	Command       string `yaml:"command"`
	ArchitectMode bool   `yaml:"architect_mode"`
	Model         string `yaml:"model"`
	EditorModel   string `yaml:"editor_model"`
	MCPServerPath string `yaml:"mcp_server_path"`
}

// Load reads and parses .aido/config.yaml. It does not validate (R3 is a
// separate call). A missing file wraps fs.ErrNotExist so callers can test it
// with errors.Is (R2.2); no default config is substituted.
func Load(r Root) (*Config, error) {
	path := r.ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c Config
	// yaml.v3 reports "line N: ..." in its error; naming the file here gives
	// R2.3's file plus position.
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &c, nil
}
