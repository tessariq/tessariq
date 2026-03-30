package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrintRunOutput_ContainsAllFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printRunOutput(&buf, runOutput{
		RunID:         "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		EvidencePath:  "/repo/.tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV",
		WorkspacePath: "/home/user/.tessariq/worktrees/abc/01ARZ3NDEKTSV4RRFFQ69G5FAV",
		ContainerName: "tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV",
	})

	output := buf.String()
	require.Contains(t, output, "run_id: 01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "evidence_path: /repo/.tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "workspace_path: /home/user/.tessariq/worktrees/abc/01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "container_name: tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "attach: tessariq attach 01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.Contains(t, output, "promote: tessariq promote 01ARZ3NDEKTSV4RRFFQ69G5FAV")
}

func TestPrintRunOutput_ScriptFriendlyFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printRunOutput(&buf, runOutput{
		RunID:         "TESTID",
		EvidencePath:  "/evidence",
		WorkspacePath: "/workspace",
		ContainerName: "tessariq-TESTID",
	})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Equal(t, 6, len(lines), "expected exactly 6 output lines")
	for _, line := range lines {
		require.Contains(t, line, ": ", "each line must be key: value format")
	}
}

func TestPrintRunOutput_AttachCommandUsesRunID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printRunOutput(&buf, runOutput{
		RunID:         "MYRUNID",
		EvidencePath:  "/e",
		WorkspacePath: "/w",
		ContainerName: "tessariq-MYRUNID",
	})

	output := buf.String()
	require.Contains(t, output, "tessariq attach MYRUNID")
	require.Contains(t, output, "tessariq promote MYRUNID")
}
