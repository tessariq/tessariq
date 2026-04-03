package proxy

import (
	"bytes"
	"context"
	"fmt"
)

// ProxyEnv holds the proxy configuration needed by the agent container.
type ProxyEnv struct {
	ProxyAddr   string // e.g. "http://tessariq-squid-<id>:3128"
	NetworkName string // e.g. "tessariq-net-<id>"
}

// Topology manages the full proxy topology lifecycle for a single run.
type Topology struct {
	RunID           string
	EvidenceDir     string
	Destinations    []string // resolved "host:port" entries from allowlist
	AllowlistSource string   // "cli", "user_config", "built_in"
	SquidImage      string   // defaults to DefaultSquidImage if empty

	// internal state
	networkName string
	squidName   string
	tornDown    bool // prevents double teardown
}

// Setup creates the proxy topology:
//  1. Build CompiledAllowlist from Destinations and AllowlistSource
//  2. WriteCompiledYAML to EvidenceDir (egress.compiled.yaml)
//  3. GenerateSquidConf from the compiled destinations
//  4. CreateNetwork (internal Docker network)
//  5. StartSquid (proxy container on internal network + bridge; config via docker cp)
//  6. Return ProxyEnv with proxy address and network name
func (t *Topology) Setup(ctx context.Context) (*ProxyEnv, error) {
	// Step 1: Build compiled allowlist.
	compiled, err := NewCompiledAllowlist(t.AllowlistSource, t.Destinations)
	if err != nil {
		return nil, fmt.Errorf("compile allowlist: %w", err)
	}

	// Step 2: Write evidence artifact.
	if err := WriteCompiledYAML(t.EvidenceDir, compiled); err != nil {
		return nil, fmt.Errorf("write compiled allowlist: %w", err)
	}

	// Step 3: Generate Squid configuration.
	conf := GenerateSquidConf(compiled.Destinations, squidListenPort)

	// Step 4: Create internal Docker network.
	t.networkName = NetworkName(t.RunID)
	if err := CreateNetwork(ctx, t.networkName); err != nil {
		return nil, fmt.Errorf("create network: %w", err)
	}

	// Step 5: Start Squid proxy container.
	image := t.SquidImage
	if image == "" {
		image = DefaultSquidImage
	}

	t.squidName = SquidContainerName(t.RunID)
	squidCfg := SquidConfig{
		Name:        t.squidName,
		Image:       image,
		NetworkName: t.networkName,
		ConfContent: conf,
	}

	if err := StartSquid(ctx, squidCfg); err != nil {
		_ = StopSquid(ctx, t.squidName)
		_ = RemoveNetwork(ctx, t.networkName)
		return nil, fmt.Errorf("start squid: %w", err)
	}

	// Step 6: Return proxy environment.
	return &ProxyEnv{
		ProxyAddr:   SquidAddress(t.squidName),
		NetworkName: t.networkName,
	}, nil
}

// Teardown cleans up the proxy topology:
//  1. CopyAccessLog from Squid container
//  2. ParseSquidAccessLog to extract blocked events
//  3. WriteEventsJSONL to EvidenceDir
//  4. CopySquidLog to EvidenceDir (capped at 10MB)
//  5. StopSquid (remove container)
//  6. RemoveNetwork
//
// Safe to call multiple times (idempotent via tornDown flag).
// Teardown is best-effort: errors are not returned.
func (t *Topology) Teardown(ctx context.Context) error {
	if t.tornDown {
		return nil
	}
	t.tornDown = true

	const maxSquidLogBytes = 10 * 1024 * 1024 // 10 MB

	// Step 1: Extract access log from Squid container.
	logData, _ := CopyAccessLog(ctx, t.squidName)

	// Step 2: Parse blocked events.
	events, _ := ParseSquidAccessLog(bytes.NewReader(logData))

	// Step 3: Write events evidence.
	_ = WriteEventsJSONL(t.EvidenceDir, events)

	// Step 4: Write raw Squid log evidence.
	_ = CopySquidLog(t.EvidenceDir, bytes.NewReader(logData), maxSquidLogBytes)

	// Step 5: Stop and remove Squid container.
	_ = StopSquid(ctx, t.squidName)

	// Step 6: Remove Docker network.
	_ = RemoveNetwork(ctx, t.networkName)

	return nil
}
