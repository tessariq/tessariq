package proxy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateSquidConf_SingleDestination(t *testing.T) {
	t.Parallel()

	dests := []CompiledDestination{
		{Host: "api.example.com", Port: 443},
	}

	conf := GenerateSquidConf(dests, 3128)

	require.Contains(t, conf, "acl hosts_443 dstdomain api.example.com")
	require.Contains(t, conf, "acl port_443 port 443")
	require.Contains(t, conf, "http_access allow CONNECT port_443 hosts_443")
	require.Contains(t, conf, "http_access deny all")
}

func TestGenerateSquidConf_MultipleDestinations(t *testing.T) {
	t.Parallel()

	dests := []CompiledDestination{
		{Host: "api.example.com", Port: 443},
		{Host: "registry.example.com", Port: 443},
		{Host: "cdn.example.com", Port: 443},
	}

	conf := GenerateSquidConf(dests, 3128)

	require.Contains(t, conf, "acl hosts_443 dstdomain api.example.com")
	require.Contains(t, conf, "acl hosts_443 dstdomain registry.example.com")
	require.Contains(t, conf, "acl hosts_443 dstdomain cdn.example.com")
	require.Contains(t, conf, "http_access allow CONNECT port_443 hosts_443")

	// Only one port group — one allow rule.
	count := strings.Count(conf, "http_access allow CONNECT")
	require.Equal(t, 1, count, "same-port destinations should produce one allow rule")
}

func TestGenerateSquidConf_MultipleUniquePorts(t *testing.T) {
	t.Parallel()

	dests := []CompiledDestination{
		{Host: "api.example.com", Port: 443},
		{Host: "registry.example.com", Port: 8443},
	}

	conf := GenerateSquidConf(dests, 3128)

	require.Contains(t, conf, "acl port_443 port 443")
	require.Contains(t, conf, "acl port_8443 port 8443")
	require.Contains(t, conf, "acl hosts_443 dstdomain api.example.com")
	require.Contains(t, conf, "acl hosts_8443 dstdomain registry.example.com")

	// Two separate allow rules.
	count := strings.Count(conf, "http_access allow CONNECT")
	require.Equal(t, 2, count, "different ports should produce separate allow rules")
}

func TestGenerateSquidConf_EmptyDestinations(t *testing.T) {
	t.Parallel()

	conf := GenerateSquidConf(nil, 3128)

	require.Contains(t, conf, "http_access deny all")
	// Should not contain any allowed_dest ACLs.
	require.NotContains(t, conf, "acl allowed_dest")
	// Should not contain any SSL_ports ACLs.
	require.NotContains(t, conf, "acl SSL_ports")
	// Should not contain the allow CONNECT rule.
	require.NotContains(t, conf, "http_access allow")
}

func TestGenerateSquidConf_DuplicateHosts(t *testing.T) {
	t.Parallel()

	dests := []CompiledDestination{
		{Host: "api.example.com", Port: 443},
		{Host: "api.example.com", Port: 8443},
	}

	conf := GenerateSquidConf(dests, 3128)

	// Host appears in both port groups — each pair is enforced independently.
	require.Contains(t, conf, "acl hosts_443 dstdomain api.example.com")
	require.Contains(t, conf, "acl hosts_8443 dstdomain api.example.com")

	// Both ports have their own ACL.
	require.Contains(t, conf, "acl port_443 port 443")
	require.Contains(t, conf, "acl port_8443 port 8443")

	// Two separate allow rules.
	count := strings.Count(conf, "http_access allow CONNECT")
	require.Equal(t, 2, count, "two ports should produce two allow rules")
}

func TestGenerateSquidConf_RequiredDirectives(t *testing.T) {
	t.Parallel()

	dests := []CompiledDestination{
		{Host: "api.example.com", Port: 443},
	}

	conf := GenerateSquidConf(dests, 3128)

	require.Contains(t, conf, "http_port 3128")
	require.Contains(t, conf, "acl CONNECT method CONNECT")
	require.Contains(t, conf, "http_access deny all")
	require.Contains(t, conf, "access_log stdio:/var/log/squid/access.log squid")
	require.Contains(t, conf, "cache deny all")
	// Old shared ACL names must not appear.
	require.NotContains(t, conf, "SSL_ports")
	require.NotContains(t, conf, "allowed_dest")
}

func TestGenerateSquidConf_MixedPortsNoCrossProduct(t *testing.T) {
	t.Parallel()

	dests := []CompiledDestination{
		{Host: "api.example.com", Port: 443},
		{Host: "cdn.example.com", Port: 8443},
	}

	conf := GenerateSquidConf(dests, 3128)

	// Each host appears only in its own port group.
	require.Contains(t, conf, "acl hosts_443 dstdomain api.example.com")
	require.Contains(t, conf, "acl hosts_8443 dstdomain cdn.example.com")
	require.NotContains(t, conf, "acl hosts_443 dstdomain cdn.example.com")
	require.NotContains(t, conf, "acl hosts_8443 dstdomain api.example.com")

	// Each port group has its own allow rule.
	require.Contains(t, conf, "http_access allow CONNECT port_443 hosts_443")
	require.Contains(t, conf, "http_access allow CONNECT port_8443 hosts_8443")

	// No shared/global allow rule exists.
	require.NotContains(t, conf, "SSL_ports")
	require.NotContains(t, conf, "allowed_dest")
}

func TestGenerateSquidConf_NoLeadingDotInOutput(t *testing.T) {
	t.Parallel()

	// Defense-in-depth: even if a leading-dot host somehow bypasses
	// ParseDestination, verify the generated config would contain it
	// so this test documents the invariant.
	dests := []CompiledDestination{
		{Host: ".evil.com", Port: 443},
		{Host: "good.com", Port: 443},
	}

	conf := GenerateSquidConf(dests, 3128)

	// The leading-dot host should appear verbatim — this test documents
	// that GenerateSquidConf does NOT strip dots, so the parser MUST
	// reject them upstream.
	require.Contains(t, conf, "dstdomain .evil.com",
		"GenerateSquidConf passes hosts through verbatim; parser must reject leading dots")
	require.Contains(t, conf, "dstdomain good.com")
}

func TestGenerateSquidConf_ListenPort(t *testing.T) {
	t.Parallel()

	dests := []CompiledDestination{
		{Host: "api.example.com", Port: 443},
	}

	conf := GenerateSquidConf(dests, 9090)

	require.Contains(t, conf, "http_port 9090")
	require.NotContains(t, conf, "http_port 3128")
}
