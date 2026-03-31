package proxy

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateSquidConf produces a Squid proxy configuration that enforces
// the given allowlist. HTTPS and WSS traffic uses CONNECT tunneling.
// An empty destinations list produces a deny-all configuration.
func GenerateSquidConf(destinations []CompiledDestination, listenPort int) string {
	var b strings.Builder

	// Section 1: Listen port.
	fmt.Fprintf(&b, "http_port %d\n", listenPort)

	if len(destinations) > 0 {
		b.WriteString("\n")

		// Section 2: ACL definitions.

		// Collect unique ports, sorted for deterministic output.
		portSet := make(map[int]struct{})
		for _, d := range destinations {
			portSet[d.Port] = struct{}{}
		}
		ports := make([]int, 0, len(portSet))
		for p := range portSet {
			ports = append(ports, p)
		}
		sort.Ints(ports)

		for _, p := range ports {
			fmt.Fprintf(&b, "acl SSL_ports port %d\n", p)
		}

		b.WriteString("acl CONNECT method CONNECT\n")

		// Collect unique hosts, preserving first-seen order.
		hostSeen := make(map[string]struct{})
		for _, d := range destinations {
			if _, ok := hostSeen[d.Host]; ok {
				continue
			}
			hostSeen[d.Host] = struct{}{}
			fmt.Fprintf(&b, "acl allowed_dest dstdomain %s\n", d.Host)
		}

		// Section 3: Access rules.
		b.WriteString("\n")
		b.WriteString("http_access allow CONNECT SSL_ports allowed_dest\n")
	}

	// Deny-all rule is always present.
	if len(destinations) == 0 {
		b.WriteString("\n")
	}
	b.WriteString("http_access deny all\n")

	// Section 4: Logging.
	b.WriteString("\n")
	b.WriteString("access_log stdio:/var/log/squid/access.log squid\n")

	// Section 5: Caching.
	b.WriteString("\n")
	b.WriteString("cache deny all\n")

	return b.String()
}
