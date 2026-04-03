package proxy

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateSquidConf produces a Squid proxy configuration that enforces
// the given allowlist. HTTPS and WSS traffic uses CONNECT tunneling.
// Each destination is enforced as an exact host-port pair by grouping
// ACLs per port. An empty destinations list produces a deny-all config.
func GenerateSquidConf(destinations []CompiledDestination, listenPort int) string {
	var b strings.Builder

	// Section 1: Listen port.
	fmt.Fprintf(&b, "http_port %d\n", listenPort)

	if len(destinations) > 0 {
		b.WriteString("\n")

		// Group hosts by port, deduplicating and sorting for determinism.
		portHosts := make(map[int][]string)
		portHostSeen := make(map[int]map[string]struct{})
		for _, d := range destinations {
			if portHostSeen[d.Port] == nil {
				portHostSeen[d.Port] = make(map[string]struct{})
			}
			if _, ok := portHostSeen[d.Port][d.Host]; ok {
				continue
			}
			portHostSeen[d.Port][d.Host] = struct{}{}
			portHosts[d.Port] = append(portHosts[d.Port], d.Host)
		}

		ports := make([]int, 0, len(portHosts))
		for p := range portHosts {
			ports = append(ports, p)
		}
		sort.Ints(ports)

		for _, p := range ports {
			hosts := portHosts[p]
			sort.Strings(hosts)
			portHosts[p] = hosts
		}

		// Section 2: ACL definitions.
		b.WriteString("acl CONNECT method CONNECT\n")

		for _, p := range ports {
			fmt.Fprintf(&b, "acl port_%d port %d\n", p, p)
			for _, h := range portHosts[p] {
				fmt.Fprintf(&b, "acl hosts_%d dstdomain %s\n", p, h)
			}
		}

		// Section 3: Access rules — one per port group.
		b.WriteString("\n")
		for _, p := range ports {
			fmt.Fprintf(&b, "http_access allow CONNECT port_%d hosts_%d\n", p, p)
		}
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
