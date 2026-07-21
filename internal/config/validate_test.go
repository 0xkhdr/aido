package config

import (
	"errors"
	"strings"
	"testing"
)

// valid returns a config that breaks no rule; each test bends one thing.
func valid() Config {
	return Config{
		Project:       "taxi",
		TrackedBranch: "main",
		RequiredDocs:  []string{"okf/architecture.md"},
		LLM: LLMConfig{
			DefaultProvider: "openrouter",
			Providers: map[string]Provider{
				"openrouter": {APIKeySource: "env:OPENROUTER_API_KEY"},
			},
		},
	}
}

// problems returns the aggregate's problem list, failing if err is not a
// *ValidationError.
func problems(t *testing.T, err error) []string {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v (%T), want *ValidationError", err, err)
	}
	return ve.Problems
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	c := valid()
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

// R3.1, R3.2, R3.3, R3.4: each rule fires on its own and names the offender.
func TestValidateSingleFailures(t *testing.T) {
	tests := []struct {
		name  string
		bend  func(*Config)
		wants string
	}{
		{"missing project", func(c *Config) { c.Project = "" }, "project is required"},
		{"missing tracked_branch", func(c *Config) { c.TrackedBranch = "" }, "tracked_branch is required"},
		{"default_provider not in providers", func(c *Config) { c.LLM.DefaultProvider = "openai" }, "openai"},
		{"unsupported provider", func(c *Config) { c.LLM.Providers["deepthought"] = Provider{} }, "deepthought"},
		{"required_docs outside okf/", func(c *Config) { c.RequiredDocs = []string{"docs/architecture.md"} }, "docs/architecture.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.bend(&c)
			err := c.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want a validation error")
			}
			got := problems(t, err)
			if len(got) != 1 {
				t.Fatalf("problems = %v, want exactly 1", got)
			}
			if !strings.Contains(got[0], tt.wants) {
				t.Errorf("problem %q does not name %q", got[0], tt.wants)
			}
		})
	}
}

// R3.1: both required keys missing at once are both reported.
func TestValidateBothRequiredKeysMissing(t *testing.T) {
	c := valid()
	c.Project = ""
	c.TrackedBranch = ""
	got := problems(t, c.Validate())
	if len(got) != 2 {
		t.Fatalf("problems = %v, want 2", got)
	}
}

// R3.5, invariant I4: validation is total — three unrelated violations all
// appear in the single returned error.
func TestValidateReportsEveryFailureAtOnce(t *testing.T) {
	c := valid()
	c.Project = ""
	c.RequiredDocs = []string{"architecture.md"}
	c.LLM.DefaultProvider = "mistral" // supported, but absent from providers
	err := c.Validate()
	got := problems(t, err)
	if len(got) != 3 {
		t.Fatalf("problems = %v, want 3", got)
	}
	for _, want := range []string{"project", "architecture.md", "mistral"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("aggregate error %q does not mention %q", err, want)
		}
	}
}

// R3.3: every provider the blueprint names is accepted, so the closed set is
// not accidentally narrower than §4.3.
func TestValidateAcceptsEverySupportedProvider(t *testing.T) {
	c := valid()
	c.LLM.Providers = map[string]Provider{}
	for _, name := range SupportedProviders {
		c.LLM.Providers[name] = Provider{}
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil for the supported set %v", err, SupportedProviders)
	}
}
