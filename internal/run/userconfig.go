package run

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// UserConfig represents the user-level tessariq configuration.
type UserConfig struct {
	EgressAllow []string `yaml:"egress_allow"`
}

// UserConfigPath returns the path to config.yaml following XDG_CONFIG_HOME
// convention, or "" if the config directory does not exist.
func UserConfigPath(xdgConfigHome, homeDir string, dirExists func(string) bool) string {
	var configDir string
	if xdgConfigHome != "" {
		configDir = filepath.Join(xdgConfigHome, "tessariq")
	} else {
		configDir = filepath.Join(homeDir, ".config", "tessariq")
	}

	if !dirExists(configDir) {
		return ""
	}
	return filepath.Join(configDir, "config.yaml")
}

// LoadUserConfig reads and parses the user-level config file.
// Returns (nil, nil) when path is "" or the file does not exist.
func LoadUserConfig(path string, readFile func(string) ([]byte, error)) (*UserConfig, error) {
	if path == "" {
		return nil, nil
	}

	data, err := readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("unreadable config file %s: %w", path, err)
		}
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg UserConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return &cfg, nil
		}
		if isUnknownFieldError(err) {
			return nil, fmt.Errorf("unknown field in config file %s: %w; check key names against documentation", path, err)
		}
		return nil, fmt.Errorf("malformed config file %s: %w; check YAML syntax", path, err)
	}

	return &cfg, nil
}

// isUnknownFieldError reports whether err is from yaml.v3 KnownFields
// rejecting an unrecognised key. The library uses a "not found" message
// rather than a typed error, so we match on the message prefix.
func isUnknownFieldError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found in type")
}
