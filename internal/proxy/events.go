package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Event represents a single blocked egress attempt recorded in egress.events.jsonl.
type Event struct {
	Timestamp   string `json:"timestamp"` // RFC3339 UTC
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Action      string `json:"action"`       // "blocked"
	Reason      string `json:"reason"`       // "not_in_allowlist"
	SquidResult string `json:"squid_result"` // e.g. "TCP_DENIED/403"
}

const (
	eventsFileName   = "egress.events.jsonl"
	squidLogName     = "squid.log"
	truncationMarker = "\n[truncated]"
)

// ParseSquidAccessLog parses a Squid access.log, filters for TCP_DENIED entries,
// and transforms them to Event structs.
func ParseSquidAccessLog(r io.Reader) ([]Event, error) {
	var events []Event
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		resultCode := fields[3]
		if !strings.HasPrefix(resultCode, "TCP_DENIED") {
			continue
		}

		ts, ok := parseSquidTimestamp(fields[0])
		if !ok {
			continue
		}

		host, port, ok := parseSquidURL(fields[5], fields[6])
		if !ok {
			continue
		}

		events = append(events, Event{
			Timestamp:   ts,
			Host:        host,
			Port:        port,
			Action:      "blocked",
			Reason:      "not_in_allowlist",
			SquidResult: resultCode,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan squid access log: %w", err)
	}

	return events, nil
}

// parseSquidTimestamp converts a Squid epoch.ms timestamp to RFC3339 UTC.
func parseSquidTimestamp(raw string) (string, bool) {
	dotIdx := strings.Index(raw, ".")
	if dotIdx < 0 {
		// Try parsing as pure integer seconds.
		sec, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return "", false
		}
		return time.Unix(sec, 0).UTC().Format(time.RFC3339), true
	}

	secStr := raw[:dotIdx]
	sec, err := strconv.ParseInt(secStr, 10, 64)
	if err != nil {
		return "", false
	}

	msStr := raw[dotIdx+1:]
	ms, err := strconv.ParseInt(msStr, 10, 64)
	if err != nil {
		return "", false
	}

	// Normalize to nanoseconds. Squid uses milliseconds (3 digits),
	// but handle variable precision gracefully.
	digits := len(msStr)
	nsec := ms * int64(math.Pow10(9-digits))

	return time.Unix(sec, nsec).UTC().Format(time.RFC3339), true
}

// parseSquidURL extracts host and port from the Squid method and URL fields.
// For CONNECT, the URL is host:port. For other methods, it is a full URL.
func parseSquidURL(method, rawURL string) (string, int, bool) {
	if strings.EqualFold(method, "CONNECT") {
		host, portStr, err := net.SplitHostPort(rawURL)
		if err != nil {
			return "", 0, false
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return "", 0, false
		}
		return host, port, true
	}

	// Non-CONNECT: parse as URL.
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, false
	}

	host := u.Hostname()
	if host == "" {
		return "", 0, false
	}

	portStr := u.Port()
	if portStr == "" {
		if u.Scheme == "https" {
			return host, 443, true
		}
		return host, 80, true
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, false
	}

	return host, port, true
}

// WriteEventsJSONL writes events as JSONL (one JSON object per newline-terminated line)
// to egress.events.jsonl with 0o600 permissions.
func WriteEventsJSONL(evidenceDir string, events []Event) error {
	target := filepath.Join(evidenceDir, eventsFileName)
	tmp := target + ".tmp"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create events temp file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("encode event: %w", err)
		}
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close events temp file: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("rename events file: %w", err)
	}

	return nil
}

// ReadEventsJSONL reads events from egress.events.jsonl in the evidence directory.
func ReadEventsJSONL(evidenceDir string) ([]Event, error) {
	f, err := os.Open(filepath.Join(evidenceDir, eventsFileName))
	if err != nil {
		return nil, fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("decode event line: %w", err)
		}
		events = append(events, e)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan events file: %w", err)
	}

	return events, nil
}

// CopySquidLog copies a Squid access log to squid.log in the evidence directory,
// capped at maxBytes. If the input exceeds maxBytes, the output is truncated
// and a "[truncated]" marker is appended. File permissions are 0o600.
func CopySquidLog(evidenceDir string, r io.Reader, maxBytes int64) error {
	target := filepath.Join(evidenceDir, squidLogName)
	tmp := target + ".tmp"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create squid log temp file: %w", err)
	}
	defer f.Close()

	n, err := io.CopyN(f, r, maxBytes)
	if err != nil && err != io.EOF {
		return fmt.Errorf("copy squid log: %w", err)
	}

	truncated := n == maxBytes && err == nil
	if truncated {
		// Check if there is more data remaining.
		buf := make([]byte, 1)
		_, readErr := r.Read(buf)
		if readErr == nil || readErr != io.EOF {
			if _, writeErr := f.WriteString(truncationMarker); writeErr != nil {
				return fmt.Errorf("write truncation marker: %w", writeErr)
			}
		}
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close squid log temp file: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("rename squid log file: %w", err)
	}

	return nil
}
