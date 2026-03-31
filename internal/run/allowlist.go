package run

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// AllowlistResult holds the resolved allowlist and its provenance.
type AllowlistResult struct {
	Destinations []string // canonical "host:port" entries
	Source       string   // "cli", "user_config", or "built_in"
}

// ParseDestination parses a "host:port" string. Port defaults to 443 if omitted.
func ParseDestination(s string) (host string, port int, err error) {
	if s == "" {
		return "", 0, errors.New("empty destination")
	}

	lastColon := strings.LastIndex(s, ":")
	if lastColon == -1 {
		host = s
		port = 443
	} else {
		host = s[:lastColon]
		portStr := s[lastColon+1:]
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("non-numeric port in %q", s)
		}
	}

	if host == "" {
		return "", 0, fmt.Errorf("empty host in %q", s)
	}

	if strings.ContainsAny(host, " \t") {
		return "", 0, fmt.Errorf("invalid host %q: contains whitespace", host)
	}

	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("port %d out of range 1-65535 in %q", port, s)
	}

	return host, port, nil
}

// parseDestinations validates and normalizes a list of "host:port" entries.
func parseDestinations(entries []string) ([]string, error) {
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		host, port, err := ParseDestination(entry)
		if err != nil {
			return nil, err
		}
		result = append(result, fmt.Sprintf("%s:%d", host, port))
	}
	return result, nil
}

// ResolveAllowlist determines the final allowlist based on precedence:
// CLI > user_config > built_in.
func ResolveAllowlist(
	cliAllow []string,
	userConfig *UserConfig,
	builtIn []string,
	egressNoDefaults bool,
	resolvedEgressMode string,
) (*AllowlistResult, error) {
	// CLI entries take highest priority.
	if len(cliAllow) > 0 {
		dests, err := parseDestinations(cliAllow)
		if err != nil {
			return nil, err
		}
		return &AllowlistResult{Destinations: dests, Source: "cli"}, nil
	}

	// --egress-no-defaults discards user config and built-in.
	if egressNoDefaults {
		if resolvedEgressMode == "proxy" {
			return nil, errors.New("proxy mode requires at least one allowlist destination; use --egress-allow or remove --egress-no-defaults")
		}
		return &AllowlistResult{Destinations: nil, Source: "cli"}, nil
	}

	// User config overrides built-in.
	if userConfig != nil && len(userConfig.EgressAllow) > 0 {
		dests, err := parseDestinations(userConfig.EgressAllow)
		if err != nil {
			return nil, err
		}
		return &AllowlistResult{Destinations: dests, Source: "user_config"}, nil
	}

	// Fall back to built-in.
	return &AllowlistResult{Destinations: builtIn, Source: "built_in"}, nil
}
