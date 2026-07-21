package config

import (
	"sort"
	"strings"
)

// SupportedProviders is the closed set of LLM providers config may name
// (blueprint §4.3, R3.3). Deliberately duplicated rather than shared with a
// future internal/llm — see design.md Alternatives.
var SupportedProviders = []string{"openai", "anthropic", "mistral", "nvidia_nim", "openrouter", "ollama"}

// ValidationError aggregates every validation failure found in one pass (R3.5,
// invariant I4). The list is flat; there is no wrapped cause to unwrap.
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return "invalid config: " + strings.Join(e.Problems, "; ")
}

// Validate reports every rule the config breaks, or nil. It is total: it never
// returns after the first problem (R3.5).
func (c *Config) Validate() error {
	var problems []string
	if c.Project == "" {
		problems = append(problems, "project is required")
	}
	if c.TrackedBranch == "" {
		problems = append(problems, "tracked_branch is required")
	}
	for _, doc := range c.RequiredDocs {
		if !strings.HasPrefix(doc, "okf/") {
			problems = append(problems, "required_docs entry "+doc+" must be under okf/")
		}
	}
	// Provider names are checked in sorted order so the message is stable
	// across runs — map iteration is not.
	names := make([]string, 0, len(c.LLM.Providers))
	for name := range c.LLM.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !supported(name) {
			problems = append(problems, "provider "+name+" is not supported")
		}
	}
	if p := c.LLM.DefaultProvider; p != "" {
		if _, ok := c.LLM.Providers[p]; !ok {
			problems = append(problems, "llm.default_provider "+p+" has no entry under llm.providers")
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return &ValidationError{Problems: problems}
}

func supported(name string) bool {
	for _, s := range SupportedProviders {
		if s == name {
			return true
		}
	}
	return false
}
