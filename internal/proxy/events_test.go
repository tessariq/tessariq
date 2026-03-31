package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSquidAccessLog_DeniedEntries(t *testing.T) {
	t.Parallel()

	log := `1711900800.123    200 192.168.1.2 TCP_DENIED/403 3900 CONNECT blocked.example.com:443 - HIER_NONE/- -
`
	events, err := ParseSquidAccessLog(strings.NewReader(log))
	require.NoError(t, err)
	require.Len(t, events, 1)

	e := events[0]
	require.Equal(t, "blocked.example.com", e.Host)
	require.Equal(t, 443, e.Port)
	require.Equal(t, "blocked", e.Action)
	require.Equal(t, "not_in_allowlist", e.Reason)
	require.Equal(t, "TCP_DENIED/403", e.SquidResult)
}

func TestParseSquidAccessLog_MixedEntries(t *testing.T) {
	t.Parallel()

	log := `1711900800.123    200 192.168.1.2 TCP_DENIED/403 3900 CONNECT blocked.example.com:443 - HIER_NONE/- -
1711900801.456     50 192.168.1.2 TCP_MISS/200 5000 CONNECT api.anthropic.com:443 - HIER_DIRECT/api.anthropic.com -
1711900802.789    100 192.168.1.2 TCP_DENIED/403 3900 CONNECT evil.example.com:8443 - HIER_NONE/- -
`
	events, err := ParseSquidAccessLog(strings.NewReader(log))
	require.NoError(t, err)
	require.Len(t, events, 2)

	require.Equal(t, "blocked.example.com", events[0].Host)
	require.Equal(t, 443, events[0].Port)

	require.Equal(t, "evil.example.com", events[1].Host)
	require.Equal(t, 8443, events[1].Port)
}

func TestParseSquidAccessLog_EmptyLog(t *testing.T) {
	t.Parallel()

	events, err := ParseSquidAccessLog(strings.NewReader(""))
	require.NoError(t, err)
	require.Empty(t, events)
}

func TestParseSquidAccessLog_MalformedLines(t *testing.T) {
	t.Parallel()

	log := `this is not a valid log line
1711900800.123    200 192.168.1.2 TCP_DENIED/403 3900 CONNECT blocked.example.com:443 - HIER_NONE/- -
short line
`
	events, err := ParseSquidAccessLog(strings.NewReader(log))
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "blocked.example.com", events[0].Host)
}

func TestParseSquidAccessLog_TimestampConversion(t *testing.T) {
	t.Parallel()

	// 1711900800.123 = 2024-03-31T16:00:00Z (epoch 1711900800)
	log := `1711900800.123    200 192.168.1.2 TCP_DENIED/403 3900 CONNECT blocked.example.com:443 - HIER_NONE/- -
`
	events, err := ParseSquidAccessLog(strings.NewReader(log))
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "2024-03-31T16:00:00Z", events[0].Timestamp)
}

func TestParseSquidAccessLog_NonStandardPort(t *testing.T) {
	t.Parallel()

	log := `1711900802.789    100 192.168.1.2 TCP_DENIED/403 3900 CONNECT registry.example.com:8443 - HIER_NONE/- -
`
	events, err := ParseSquidAccessLog(strings.NewReader(log))
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "registry.example.com", events[0].Host)
	require.Equal(t, 8443, events[0].Port)
}

func TestWriteEventsJSONL_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	events := []Event{
		{
			Timestamp:   "2024-03-31T16:00:00Z",
			Host:        "blocked.example.com",
			Port:        443,
			Action:      "blocked",
			Reason:      "not_in_allowlist",
			SquidResult: "TCP_DENIED/403",
		},
		{
			Timestamp:   "2024-03-31T16:00:02Z",
			Host:        "evil.example.com",
			Port:        8443,
			Action:      "blocked",
			Reason:      "not_in_allowlist",
			SquidResult: "TCP_DENIED/403",
		},
	}

	err := WriteEventsJSONL(dir, events)
	require.NoError(t, err)

	got, err := ReadEventsJSONL(dir)
	require.NoError(t, err)
	require.Equal(t, events, got)
}

func TestWriteEventsJSONL_EmptyEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := WriteEventsJSONL(dir, []Event{})
	require.NoError(t, err)

	got, err := ReadEventsJSONL(dir)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestWriteEventsJSONL_Permissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	events := []Event{
		{
			Timestamp:   "2024-03-31T16:00:00Z",
			Host:        "example.com",
			Port:        443,
			Action:      "blocked",
			Reason:      "not_in_allowlist",
			SquidResult: "TCP_DENIED/403",
		},
	}

	err := WriteEventsJSONL(dir, events)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "egress.events.jsonl"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestCopySquidLog_UnderLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "line1\nline2\nline3\n"

	err := CopySquidLog(dir, strings.NewReader(content), 1024)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "squid.log"))
	require.NoError(t, err)
	require.Equal(t, content, string(data))
}

func TestCopySquidLog_OverLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := strings.Repeat("abcdefghij", 10) // 100 bytes

	err := CopySquidLog(dir, strings.NewReader(content), 50)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "squid.log"))
	require.NoError(t, err)

	// Should be truncated at 50 bytes plus the truncation marker.
	require.True(t, len(data) <= 50+len("\n[truncated]"),
		"expected truncated output, got %d bytes", len(data))
	require.Contains(t, string(data), "[truncated]")
	require.True(t, strings.HasPrefix(string(data), content[:50]))
}

func TestCopySquidLog_Permissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "some log content\n"

	err := CopySquidLog(dir, strings.NewReader(content), 1024)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "squid.log"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
