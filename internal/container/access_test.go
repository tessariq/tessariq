package container

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildHardenPathCommands_LinuxSameUID_UsesOnlyChmod(t *testing.T) {
	t.Parallel()

	cmds := buildHardenPathCommands("linux", "/tmp/work", os.Getuid(), RuntimeIdentity{UID: os.Getuid(), GID: 1234})
	require.Equal(t, [][]string{{"chmod", "-R", "u=rwX,go=", "/tmp/work"}}, cmds)
}

func TestBuildHardenPathCommands_LinuxDifferentUID_AddsACLs(t *testing.T) {
	t.Parallel()

	cmds := buildHardenPathCommands("linux", "/tmp/work", 501, RuntimeIdentity{UID: 1234, GID: 1234})
	require.Equal(t, [][]string{
		{"chmod", "-R", "u=rwX,go=", "/tmp/work"},
		{"setfacl", "-R", "-m", "u:1234:rwX", "/tmp/work"},
		{"find", "/tmp/work", "-type", "d", "-exec", "setfacl", "-m", "d:u:501:rwX,d:u:1234:rwX", "{}", "+"},
	}, cmds)
}

func TestBuildHardenPathCommands_DarwinDifferentUID_UsesOnlyChmod(t *testing.T) {
	t.Parallel()

	cmds := buildHardenPathCommands("darwin", "/tmp/work", 501, RuntimeIdentity{UID: 1234, GID: 1234})
	require.Equal(t, [][]string{{"chmod", "-R", "u=rwX,go=", "/tmp/work"}}, cmds)
}
