package proxy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tessariq/tessariq/internal/run"
	"gopkg.in/yaml.v3"
)

// CompiledAllowlist represents the egress.compiled.yaml evidence artifact.
type CompiledAllowlist struct {
	SchemaVersion   int                   `yaml:"schema_version"`
	AllowlistSource string                `yaml:"allowlist_source"`
	Destinations    []CompiledDestination `yaml:"destinations"`
}

// CompiledDestination is a single host:port pair in the compiled allowlist.
type CompiledDestination struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// NewCompiledAllowlist parses raw "host:port" strings into a CompiledAllowlist.
// Port defaults to 443 when omitted, using run.ParseDestination for parsing.
func NewCompiledAllowlist(source string, rawDestinations []string) (*CompiledAllowlist, error) {
	dests := make([]CompiledDestination, 0, len(rawDestinations))
	for _, raw := range rawDestinations {
		host, port, err := run.ParseDestination(raw)
		if err != nil {
			return nil, fmt.Errorf("parse destination %q: %w", raw, err)
		}
		dests = append(dests, CompiledDestination{Host: host, Port: port})
	}
	return &CompiledAllowlist{
		SchemaVersion:   1,
		AllowlistSource: source,
		Destinations:    dests,
	}, nil
}

// Validate checks that the compiled allowlist has a supported schema version
// and all spec-required fields are present.
func (c *CompiledAllowlist) Validate() error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", c.SchemaVersion)
	}
	if c.AllowlistSource == "" {
		return fmt.Errorf("missing required field %q", "allowlist_source")
	}
	if len(c.Destinations) == 0 {
		return fmt.Errorf("missing required field %q", "destinations")
	}
	for i, d := range c.Destinations {
		if d.Host == "" {
			return fmt.Errorf("destination[%d]: missing required field %q", i, "host")
		}
		if d.Port == 0 {
			return fmt.Errorf("destination[%d]: missing required field %q", i, "port")
		}
	}
	return nil
}

// compiledFileName is the evidence artifact file name.
const compiledFileName = "egress.compiled.yaml"

// WriteCompiledYAML writes the compiled allowlist to egress.compiled.yaml
// in the evidence directory. Uses an atomic write pattern (temp file + rename).
func WriteCompiledYAML(evidenceDir string, c *CompiledAllowlist) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal compiled allowlist: %w", err)
	}

	target := filepath.Join(evidenceDir, compiledFileName)
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write compiled allowlist temp file: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("rename compiled allowlist file: %w", err)
	}

	return nil
}

// ReadCompiledYAML reads and parses egress.compiled.yaml from the evidence directory.
func ReadCompiledYAML(evidenceDir string) (*CompiledAllowlist, error) {
	data, err := os.ReadFile(filepath.Join(evidenceDir, compiledFileName))
	if err != nil {
		return nil, fmt.Errorf("read compiled allowlist: %w", err)
	}

	var c CompiledAllowlist
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse compiled allowlist: %w", err)
	}

	return &c, nil
}
