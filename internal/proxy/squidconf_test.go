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

	require.Contains(t, conf, "acl allowed_dest dstdomain api.example.com")
	require.Contains(t, conf, "acl SSL_ports port 443")
	require.Contains(t, conf, "http_access allow CONNECT SSL_ports allowed_dest")
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

	require.Contains(t, conf, "acl allowed_dest dstdomain api.example.com")
	require.Contains(t, conf, "acl allowed_dest dstdomain registry.example.com")
	require.Contains(t, conf, "acl allowed_dest dstdomain cdn.example.com")
}

func TestGenerateSquidConf_MultipleUniquePorts(t *testing.T) {
	t.Parallel()

	dests := []CompiledDestination{
		{Host: "api.example.com", Port: 443},
		{Host: "registry.example.com", Port: 8443},
	}

	conf := GenerateSquidConf(dests, 3128)

	require.Contains(t, conf, "acl SSL_ports port 443")
	require.Contains(t, conf, "acl SSL_ports port 8443")
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

	// Only one dstdomain ACL for the host, even though it appears twice.
	count := strings.Count(conf, "acl allowed_dest dstdomain api.example.com")
	require.Equal(t, 1, count, "duplicate host should produce only one dstdomain ACL")

	// Both ports should appear.
	require.Contains(t, conf, "acl SSL_ports port 443")
	require.Contains(t, conf, "acl SSL_ports port 8443")
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
