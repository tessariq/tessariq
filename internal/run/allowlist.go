package run

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// containsControlOrSpace reports whether s contains any ASCII control byte
// (0x00–0x1F), DEL (0x7F), or space (0x20). These characters are unsafe in
// proxy configuration directives and must be rejected before config generation.
// Extends ContainsControlChar with an explicit space check; keep task-path and
// trailer callers on ContainsControlChar, which allows space.
func containsControlOrSpace(s string) bool {
	if ContainsControlChar(s) {
		return true
	}
	return strings.ContainsRune(s, ' ')
}

// AllowlistResult holds the resolved allowlist and its provenance.
type AllowlistResult struct {
	Destinations []string // canonical "host:port" entries
	Source       string   // "cli", "user_config", or "built_in"
}

// ParseDestination parses a "host:port" string. Port defaults to 443 if omitted.
// Bracketed IPv6 forms like "[::1]:443" are supported. Bare IPv6 addresses
// without brackets are rejected as ambiguous.
func ParseDestination(s string) (host string, port int, err error) {
	if s == "" {
		return "", 0, errors.New("empty destination")
	}

	if s[0] == '[' {
		// Bracketed form: [host]:port — use net.SplitHostPort for correct parsing.
		hostPart, portStr, splitErr := net.SplitHostPort(s)
		if splitErr != nil {
			return "", 0, fmt.Errorf("bracketed destination %q must include a port (e.g., [::1]:443)", s)
		}
		host = hostPart
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("non-numeric port in %q", s)
		}
	} else if strings.Count(s, ":") > 1 {
		// Multiple colons without brackets — bare IPv6, ambiguous.
		return "", 0, fmt.Errorf("bare IPv6 address %q is ambiguous; use bracketed form [host]:port", s)
	} else if i := strings.LastIndex(s, ":"); i == -1 {
		// No colon at all: host-only, default port.
		host = s
		port = 443
	} else {
		// Exactly one colon: host:port.
		host = s[:i]
		portStr := s[i+1:]
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("non-numeric port in %q", s)
		}
	}

	if host == "" {
		return "", 0, fmt.Errorf("empty host in %q", s)
	}

	if containsControlOrSpace(host) {
		return "", 0, fmt.Errorf("invalid host %q: contains control character or space", host)
	}

	if host[0] == '.' {
		return "", 0, fmt.Errorf("invalid host %q: leading dot would act as a subdomain wildcard in proxy config", host)
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
