package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrintPromoteOutput_ContainsBranchAndCommit(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printPromoteOutput(&buf, promoteOutput{Branch: "tessariq/RUN123", Commit: "abc123"})

	output := buf.String()
	require.Contains(t, output, "branch: tessariq/RUN123")
	require.Contains(t, output, "commit: abc123")
}
