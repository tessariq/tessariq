package run

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	validAgents = map[string]bool{"claude-code": true, "opencode": true}
	validEgress = map[string]bool{"none": true, "proxy": true, "open": true, "auto": true}
)

type Config struct {
	TaskPath         string
	Timeout          time.Duration
	Grace            time.Duration
	Agent            string
	Image            string
	Model            string
	Interactive      bool
	Egress           string
	UnsafeEgress     bool
	EgressAllow      []string
	EgressNoDefaults bool
	Pre              []string
	Verify           []string
	Attach           bool
	MountAgentConfig bool
}

func DefaultConfig() Config {
	return Config{
		Timeout: 30 * time.Minute,
		Grace:   30 * time.Second,
		Agent:   "claude-code",
		Egress:  "auto",
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.TaskPath) == "" {
		return errors.New("task path is required")
	}

	if c.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}

	if c.Grace < 0 {
		return errors.New("grace period must not be negative")
	}

	if c.Grace > c.Timeout {
		return errors.New("grace period must not exceed timeout")
	}

	if !validAgents[c.Agent] {
		return fmt.Errorf("unsupported agent: %s", c.Agent)
	}

	if c.UnsafeEgress && c.Egress != "auto" {
		return errors.New("unsafe-egress and egress flags are mutually exclusive")
	}

	egress := c.ResolveEgress()
	if !validEgress[egress] {
		return fmt.Errorf("unsupported egress mode: %s", egress)
	}

	if egress == "none" && len(c.EgressAllow) > 0 {
		return errors.New("egress-allow cannot be used with egress mode none")
	}

	for i, cmd := range c.Pre {
		if strings.TrimSpace(cmd) == "" {
			return fmt.Errorf("pre command %d must not be empty", i)
		}
	}

	for i, cmd := range c.Verify {
		if strings.TrimSpace(cmd) == "" {
			return fmt.Errorf("verify command %d must not be empty", i)
		}
	}

	return nil
}

func (c Config) ResolveEgress() string {
	if c.UnsafeEgress {
		return "open"
	}
	return c.Egress
}
