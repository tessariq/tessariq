package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	versioninfo "github.com/tessariq/tessariq/internal/version"
)

func TestRootHelpIncludesVersionCommand(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd()
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--help"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "version")
}

func TestVersionCommandOutputMatchesRootVersionFlag(t *testing.T) {
	t.Parallel()

	flagOut := new(bytes.Buffer)
	flagCmd := newRootCmd()
	flagCmd.SetOut(flagOut)
	flagCmd.SetErr(flagOut)
	flagCmd.SetArgs([]string{"--version"})
	require.NoError(t, flagCmd.Execute())

	subOut := new(bytes.Buffer)
	subCmd := newRootCmd()
	subCmd.SetOut(subOut)
	subCmd.SetErr(subOut)
	subCmd.SetArgs([]string{"version"})
	require.NoError(t, subCmd.Execute())

	expected := "tessariq v" + versioninfo.Version + "\n"
	require.Equal(t, expected, flagOut.String())
	require.Equal(t, expected, subOut.String())
}

func TestVersionCommandHelpIsCommandLocal(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd()
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"version", "--help"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "Print the Tessariq version")
	require.NotContains(t, out.String(), "--agent")
	require.NotContains(t, out.String(), "--attach")
	require.NotContains(t, out.String(), "--mount-agent-config")
	require.NotContains(t, out.String(), "Global Flags:")
}
