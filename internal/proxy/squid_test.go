package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSquidContainerName(t *testing.T) {
	t.Parallel()

	got := SquidContainerName("abc123")
	require.Equal(t, "tessariq-squid-abc123", got)
}

func TestSquidAddress(t *testing.T) {
	t.Parallel()

	got := SquidAddress("tessariq-squid-abc123")
	require.Equal(t, "http://tessariq-squid-abc123:3128", got)
}

func TestSquidAddress_CustomNames(t *testing.T) {
	t.Parallel()

	a := SquidAddress("squid-alpha")
	b := SquidAddress("squid-beta")

	require.NotEqual(t, a, b)
	require.Equal(t, "http://squid-alpha:3128", a)
	require.Equal(t, "http://squid-beta:3128", b)
}

// indexOf returns the first index of s in args, or -1.
func indexOf(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return -1
}

func TestBuildSquidCreateArgs_CapDropAll(t *testing.T) {
	t.Parallel()
	cfg := SquidConfig{Name: "squid-test", NetworkName: "net-test"}
	args := buildSquidCreateArgs(cfg, "squid:latest")

	capIdx := indexOf(args, "--cap-drop")
	require.GreaterOrEqual(t, capIdx, 0, "--cap-drop must be present")
	require.Equal(t, "ALL", args[capIdx+1])

	imgIdx := indexOf(args, "squid:latest")
	require.Less(t, capIdx, imgIdx, "--cap-drop must precede image")
}

func TestBuildSquidCreateArgs_CapAddSetgidSetuid(t *testing.T) {
	t.Parallel()
	cfg := SquidConfig{Name: "squid-test", NetworkName: "net-test"}
	args := buildSquidCreateArgs(cfg, "squid:latest")

	imgIdx := indexOf(args, "squid:latest")

	// Find all --cap-add flags.
	var caps []string
	for i, a := range args {
		if a == "--cap-add" && i+1 < len(args) {
			caps = append(caps, args[i+1])
			require.Less(t, i, imgIdx, "--cap-add must precede image")
		}
	}
	require.Contains(t, caps, "SETGID", "SETGID must be re-added")
	require.Contains(t, caps, "SETUID", "SETUID must be re-added")
	require.Len(t, caps, 2, "only SETGID and SETUID should be re-added")
}

func TestBuildSquidCreateArgs_NoNewPrivileges(t *testing.T) {
	t.Parallel()
	cfg := SquidConfig{Name: "squid-test", NetworkName: "net-test"}
	args := buildSquidCreateArgs(cfg, "squid:latest")

	secIdx := indexOf(args, "--security-opt")
	require.GreaterOrEqual(t, secIdx, 0, "--security-opt must be present")
	require.Equal(t, "no-new-privileges", args[secIdx+1])

	imgIdx := indexOf(args, "squid:latest")
	require.Less(t, secIdx, imgIdx, "--security-opt must precede image")
}

func TestBuildSquidCreateArgs_PreservesNameAndNetwork(t *testing.T) {
	t.Parallel()
	cfg := SquidConfig{Name: "squid-abc", NetworkName: "net-xyz"}
	args := buildSquidCreateArgs(cfg, "squid:latest")

	nameIdx := indexOf(args, "--name")
	require.GreaterOrEqual(t, nameIdx, 0, "--name must be present")
	require.Equal(t, "squid-abc", args[nameIdx+1])

	netIdx := indexOf(args, "--net")
	require.GreaterOrEqual(t, netIdx, 0, "--net must be present")
	require.Equal(t, "net-xyz", args[netIdx+1])
}
