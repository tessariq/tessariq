package container

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/authmount"
)

func TestDockerFlag_ReadWrite(t *testing.T) {
	t.Parallel()
	m := Mount{Source: "/host/work", Target: "/work", ReadOnly: false}
	require.Equal(t, "/host/work:/work", m.DockerFlag())
}

func TestDockerFlag_ReadOnly(t *testing.T) {
	t.Parallel()
	m := Mount{Source: "/host/cred", Target: "/home/tessariq/.claude/.credentials.json", ReadOnly: true}
	require.Equal(t, "/host/cred:/home/tessariq/.claude/.credentials.json:ro", m.DockerFlag())
}

func TestDockerFlag_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mount    Mount
		expected string
	}{
		{
			name:     "rw mount",
			mount:    Mount{Source: "/a", Target: "/b", ReadOnly: false},
			expected: "/a:/b",
		},
		{
			name:     "ro mount",
			mount:    Mount{Source: "/a", Target: "/b", ReadOnly: true},
			expected: "/a:/b:ro",
		},
		{
			name:     "path with spaces",
			mount:    Mount{Source: "/host/my dir", Target: "/container/my dir", ReadOnly: false},
			expected: "/host/my dir:/container/my dir",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, tt.mount.DockerFlag())
		})
	}
}

func TestAssembleMounts_WorktreeAndEvidence(t *testing.T) {
	t.Parallel()
	mounts := AssembleMounts("/host/worktree", "/host/evidence", nil, nil)

	require.Len(t, mounts, 2)
	require.Equal(t, Mount{Source: "/host/worktree", Target: "/work", ReadOnly: false}, mounts[0])
	require.Equal(t, Mount{Source: "/host/evidence", Target: "/evidence", ReadOnly: true}, mounts[1])
}

func TestAssembleMounts_WithAuthMounts(t *testing.T) {
	t.Parallel()
	authMounts := []authmount.MountSpec{
		{HostPath: "/home/user/.claude/.credentials.json", ContainerPath: "/home/tessariq/.claude/.credentials.json", ReadOnly: true},
		{HostPath: "/home/user/.claude.json", ContainerPath: "/home/tessariq/.claude.json", ReadOnly: true},
	}
	mounts := AssembleMounts("/worktree", "/evidence", authMounts, nil)

	require.Len(t, mounts, 4)
	require.Equal(t, "/home/user/.claude/.credentials.json", mounts[2].Source)
	require.Equal(t, "/home/tessariq/.claude/.credentials.json", mounts[2].Target)
	require.True(t, mounts[2].ReadOnly)
	require.Equal(t, "/home/user/.claude.json", mounts[3].Source)
	require.True(t, mounts[3].ReadOnly)
}

func TestAssembleMounts_WithConfigMounts(t *testing.T) {
	t.Parallel()
	configMounts := []authmount.MountSpec{
		{HostPath: "/home/user/.claude", ContainerPath: "/home/tessariq/.claude", ReadOnly: true},
	}
	mounts := AssembleMounts("/worktree", "/evidence", nil, configMounts)

	require.Len(t, mounts, 3)
	require.Equal(t, "/home/user/.claude", mounts[2].Source)
	require.Equal(t, "/home/tessariq/.claude", mounts[2].Target)
	require.True(t, mounts[2].ReadOnly)
}

func TestAssembleMounts_WorktreeRWEvidenceRO(t *testing.T) {
	t.Parallel()
	mounts := AssembleMounts("/wt", "/ev", nil, nil)

	require.False(t, mounts[0].ReadOnly, "worktree must be read-write")
	require.True(t, mounts[1].ReadOnly, "evidence must be read-only from container")
}

func TestAssembleMounts_EvidenceNotUnderWork(t *testing.T) {
	t.Parallel()
	mounts := AssembleMounts("/host/worktree", "/host/evidence", nil, nil)

	require.NotEqual(t, mounts[0].Target, mounts[1].Target)
	require.Equal(t, "/work", mounts[0].Target)
	require.Equal(t, "/evidence", mounts[1].Target)
}

func TestAssembleMounts_HostHomeNeverExposed(t *testing.T) {
	t.Parallel()
	authMounts := []authmount.MountSpec{
		{HostPath: "/home/user/.claude/.credentials.json", ContainerPath: "/home/tessariq/.claude/.credentials.json", ReadOnly: true},
	}
	configMounts := []authmount.MountSpec{
		{HostPath: "/home/user/.claude", ContainerPath: "/home/tessariq/.claude", ReadOnly: true},
	}
	mounts := AssembleMounts("/worktree", "/evidence", authMounts, configMounts)

	for _, m := range mounts {
		require.NotEqual(t, "/home/user", m.Source, "host HOME must never be a mount source")
	}
}

func TestAssembleMounts_AllAuthAndConfigCombined(t *testing.T) {
	t.Parallel()
	authMounts := []authmount.MountSpec{
		{HostPath: "/h/cred", ContainerPath: "/c/cred", ReadOnly: true},
	}
	configMounts := []authmount.MountSpec{
		{HostPath: "/h/config", ContainerPath: "/c/config", ReadOnly: true},
	}
	mounts := AssembleMounts("/wt", "/ev", authMounts, configMounts)

	require.Len(t, mounts, 4)
	// worktree, evidence, auth, config -- in order
	require.Equal(t, "/wt", mounts[0].Source)
	require.Equal(t, "/ev", mounts[1].Source)
	require.Equal(t, "/h/cred", mounts[2].Source)
	require.Equal(t, "/h/config", mounts[3].Source)
}
