package proxy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

const maxSquidLogBytes = 10 * 1024 * 1024 // 10 MB

// Teardown cleans up the proxy topology:
//  1. CopyAccessLog from Squid container
//  2. ParseSquidAccessLog to extract blocked events
//  3. WriteEventsJSONL to EvidenceDir
//  4. CopySquidLog to EvidenceDir (capped at 10MB)
//  5. StopSquid (remove container)
//  6. RemoveNetwork
//
// Safe to call multiple times (idempotent via tornDown flag).
//
// Evidence extraction (steps 1-4) is fail-closed: if any extraction
// step fails, no evidence files are written so the completeness check
// rejects the run at promote time. Infrastructure cleanup (steps 5-6)
// always runs regardless of extraction outcome.
func (t *Topology) Teardown(ctx context.Context) error {
	if t.tornDown {
		return nil
	}
	t.tornDown = true

	// Steps 1-4: Evidence extraction (fail-closed).
	var extractionErr error
	logData, err := CopyAccessLog(ctx, t.squidName)
	if err != nil {
		extractionErr = fmt.Errorf("telemetry extraction: %w", err)
	} else if err := WriteExtractedEvidence(t.EvidenceDir, logData, maxSquidLogBytes); err != nil {
		extractionErr = fmt.Errorf("telemetry extraction: %w", err)
	}

	// Steps 5-6: Infrastructure cleanup (always runs).
	var cleanupErrs []string
	if err := StopSquid(ctx, t.squidName); err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Sprintf("stop squid: %s", err))
	}
	if err := RemoveNetwork(ctx, t.networkName); err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Sprintf("remove network: %s", err))
	}

	var errs []string
	if extractionErr != nil {
		errs = append(errs, extractionErr.Error())
	}
	errs = append(errs, cleanupErrs...)
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// WriteExtractedEvidence parses raw Squid access-log data and writes
// derived evidence artifacts to evidenceDir. On error, any partially
// written files are removed so no misleading evidence is left behind.
func WriteExtractedEvidence(evidenceDir string, logData []byte, maxLogBytes int64) (retErr error) {
	events, err := ParseSquidAccessLog(bytes.NewReader(logData))
	if err != nil {
		return fmt.Errorf("parse access log: %w", err)
	}
	if err := WriteEventsJSONL(evidenceDir, events); err != nil {
		return fmt.Errorf("write events: %w", err)
	}
	defer func() {
		if retErr != nil {
			os.Remove(filepath.Join(evidenceDir, eventsFileName))
		}
	}()
	if err := CopySquidLog(evidenceDir, bytes.NewReader(logData), maxLogBytes); err != nil {
		return fmt.Errorf("write squid log: %w", err)
	}
	return nil
}
