package adapter

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/authmount"
	"github.com/tessariq/tessariq/internal/container"
)

func TestPrepareRuntimeState_NoSeedSpecsPassThrough(t *testing.T) {
	t.Parallel()

	specs := []authmount.MountSpec{
		{HostPath: "/host/a", ContainerPath: "/c/a", ReadOnly: true},
		{HostPath: "/host/b", ContainerPath: "/c/b", ReadOnly: true},
	}

	rs, err := PrepareRuntimeState(t.TempDir(), specs)
	require.NoError(t, err)
	require.NotNil(t, rs)
	require.Equal(t, specs, rs.EffectiveMounts, "non-seed specs must pass through unchanged")
	require.NoError(t, rs.Cleanup())
}

func TestPrepareRuntimeState_SeedSpecIsCopiedToScratch(t *testing.T) {
	t.Parallel()

	hostDir := t.TempDir()
	scratchRoot := filepath.Join(t.TempDir(), "runtime-state")
	hostFile := filepath.Join(hostDir, ".claude.json")
	content := []byte(`{"numStartups":17,"feature_flags":{"foo":true}}`)
	require.NoError(t, os.WriteFile(hostFile, content, 0o600))

	specs := []authmount.MountSpec{
		{
			HostPath:        hostFile,
			ContainerPath:   "/home/tessariq/.claude.json",
			ReadOnly:        true,
			SeedIntoRuntime: true,
		},
	}

	rs, err := PrepareRuntimeState(scratchRoot, specs)
	require.NoError(t, err)
	require.Len(t, rs.EffectiveMounts, 1)

	out := rs.EffectiveMounts[0]
	require.Equal(t, "/home/tessariq/.claude.json", out.ContainerPath)
	require.False(t, out.ReadOnly, "substituted scratch spec must be read-write")
	require.False(t, out.SeedIntoRuntime, "SeedIntoRuntime flag is consumed by the transform")
	require.NotEqual(t, hostFile, out.HostPath, "host source must not be bound directly")
	require.Contains(t, out.HostPath, scratchRoot,
		"substituted host path must live under the scratch root, not on the live host auth file")

	// Scratch file is pre-populated with the host content.
	got, err := os.ReadFile(out.HostPath)
	require.NoError(t, err)
	require.Equal(t, content, got)

	// Scratch file has restrictive perms.
	info, err := os.Stat(out.HostPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Cleanup removes the scratch tree.
	require.NoError(t, rs.Cleanup())
	_, err = os.Stat(out.HostPath)
	require.True(t, errors.Is(err, fs.ErrNotExist), "scratch file must be removed on cleanup")
}

func TestPrepareRuntimeState_SeedDoesNotMutateHostFile(t *testing.T) {
	t.Parallel()

	hostDir := t.TempDir()
	hostFile := filepath.Join(hostDir, ".claude.json")
	original := []byte(`{"original":true}`)
	require.NoError(t, os.WriteFile(hostFile, original, 0o600))

	specs := []authmount.MountSpec{
		{
			HostPath:        hostFile,
			ContainerPath:   "/home/tessariq/.claude.json",
			ReadOnly:        true,
			SeedIntoRuntime: true,
		},
	}

	rs, err := PrepareRuntimeState(t.TempDir(), specs)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rs.Cleanup() })

	// Simulate in-container mutation by rewriting the scratch file.
	require.NoError(t, os.WriteFile(rs.EffectiveMounts[0].HostPath, []byte(`{"mutated":true}`), 0o600))

	// Host file must be untouched.
	got, err := os.ReadFile(hostFile)
	require.NoError(t, err)
	require.Equal(t, original, got, "host auth file must never be mutated by in-container writes")
}

func TestPrepareRuntimeState_MixedSpecs(t *testing.T) {
	t.Parallel()

	hostDir := t.TempDir()
	hostSeed := filepath.Join(hostDir, ".claude.json")
	require.NoError(t, os.WriteFile(hostSeed, []byte("seed"), 0o600))

	hostRO := filepath.Join(hostDir, ".credentials.json")
	require.NoError(t, os.WriteFile(hostRO, []byte("cred"), 0o600))

	specs := []authmount.MountSpec{
		{HostPath: hostRO, ContainerPath: "/c/cred", ReadOnly: true},
		{HostPath: hostSeed, ContainerPath: "/c/config", ReadOnly: true, SeedIntoRuntime: true},
	}

	rs, err := PrepareRuntimeState(t.TempDir(), specs)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rs.Cleanup() })
	require.Len(t, rs.EffectiveMounts, 2)

	require.Equal(t, hostRO, rs.EffectiveMounts[0].HostPath)
	require.True(t, rs.EffectiveMounts[0].ReadOnly)

	require.NotEqual(t, hostSeed, rs.EffectiveMounts[1].HostPath)
	require.False(t, rs.EffectiveMounts[1].ReadOnly)
}

func TestPrepareRuntimeState_MissingHostSourceErrors(t *testing.T) {
	t.Parallel()

	specs := []authmount.MountSpec{
		{
			HostPath:        "/nonexistent/path/.claude.json",
			ContainerPath:   "/c/claude.json",
			ReadOnly:        true,
			SeedIntoRuntime: true,
		},
	}

	_, err := PrepareRuntimeState(t.TempDir(), specs)
	require.Error(t, err)
}

func TestPrepareRuntimeState_CleanupIsIdempotent(t *testing.T) {
	t.Parallel()

	hostFile := filepath.Join(t.TempDir(), ".claude.json")
	require.NoError(t, os.WriteFile(hostFile, []byte("x"), 0o600))

	specs := []authmount.MountSpec{
		{
			HostPath:        hostFile,
			ContainerPath:   "/c/claude.json",
			ReadOnly:        true,
			SeedIntoRuntime: true,
		},
	}

	rs, err := PrepareRuntimeState(t.TempDir(), specs)
	require.NoError(t, err)

	require.NoError(t, rs.Cleanup())
	require.NoError(t, rs.Cleanup(), "cleanup must be idempotent")
}

func TestPrepareRuntimeState_EmptySpecs(t *testing.T) {
	t.Parallel()

	rs, err := PrepareRuntimeState(t.TempDir(), nil)
	require.NoError(t, err)
	require.Empty(t, rs.EffectiveMounts)
	require.NoError(t, rs.Cleanup())
}

func TestPrepareRuntimeState_FailureCleansUpPartial(t *testing.T) {
	t.Parallel()

	hostDir := t.TempDir()
	good := filepath.Join(hostDir, "good.json")
	require.NoError(t, os.WriteFile(good, []byte("ok"), 0o600))

	scratchRoot := filepath.Join(t.TempDir(), "runtime-state")

	specs := []authmount.MountSpec{
		{HostPath: good, ContainerPath: "/c/good", ReadOnly: true, SeedIntoRuntime: true},
		{HostPath: "/no/such/file", ContainerPath: "/c/bad", ReadOnly: true, SeedIntoRuntime: true},
	}

	_, err := PrepareRuntimeState(scratchRoot, specs)
	require.Error(t, err)

	// Scratch root should be cleaned up on failure.
	_, statErr := os.Stat(scratchRoot)
	require.True(t, errors.Is(statErr, fs.ErrNotExist),
		"partial scratch must be removed on failure; got stat err: %v", statErr)
}

func TestPrepareRuntimeState_ScratchBasenameMatchesContainerPath(t *testing.T) {
	t.Parallel()

	hostFile := filepath.Join(t.TempDir(), ".claude.json")
	require.NoError(t, os.WriteFile(hostFile, []byte("x"), 0o600))

	specs := []authmount.MountSpec{
		{
			HostPath:        hostFile,
			ContainerPath:   "/home/tessariq/.claude.json",
			ReadOnly:        true,
			SeedIntoRuntime: true,
		},
	}

	rs, err := PrepareRuntimeState(t.TempDir(), specs)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rs.Cleanup() })

	// Scratch basename should match the container path basename so that
	// agent-side debugging (e.g. looking at a mount source path) is less
	// surprising.
	require.Equal(t, ".claude.json", filepath.Base(rs.EffectiveMounts[0].HostPath))
}

func TestPrepareAndHardenRuntimeState_HardenFailureCleansScratch(t *testing.T) {
	hostFile := filepath.Join(t.TempDir(), ".claude.json")
	require.NoError(t, os.WriteFile(hostFile, []byte(`{"original":true}`), 0o600))
	scratchRoot := filepath.Join(t.TempDir(), "runtime-state")

	old := hardenRuntimeStatePath
	hardenRuntimeStatePath = func(_ context.Context, _ string, _ container.RuntimeIdentity) error {
		return errors.New("boom")
	}
	t.Cleanup(func() { hardenRuntimeStatePath = old })

	_, err := PrepareAndHardenRuntimeState(t.Context(), scratchRoot, []authmount.MountSpec{{
		HostPath:        hostFile,
		ContainerPath:   "/home/tessariq/.claude.json",
		ReadOnly:        true,
		SeedIntoRuntime: true,
	}}, container.RuntimeIdentity{UID: 1234, GID: 1234})
	require.Error(t, err)

	_, statErr := os.Stat(scratchRoot)
	require.True(t, errors.Is(statErr, fs.ErrNotExist), "scratch root must be cleaned on harden failure")
}

func TestPrepareAndHardenRuntimeState_NoSeedSpecsSkipsHardening(t *testing.T) {
	hostFile := filepath.Join(t.TempDir(), "auth.json")
	require.NoError(t, os.WriteFile(hostFile, []byte(`{"token":"x"}`), 0o600))

	called := false
	old := hardenRuntimeStatePath
	hardenRuntimeStatePath = func(_ context.Context, _ string, _ container.RuntimeIdentity) error {
		called = true
		return nil
	}
	t.Cleanup(func() { hardenRuntimeStatePath = old })

	rs, err := PrepareAndHardenRuntimeState(t.Context(), filepath.Join(t.TempDir(), "runtime-state"), []authmount.MountSpec{{
		HostPath:      hostFile,
		ContainerPath: "/home/tessariq/.local/share/opencode/auth.json",
		ReadOnly:      true,
	}}, container.RuntimeIdentity{UID: 1234, GID: 1234})
	require.NoError(t, err)
	require.False(t, called, "hardening must be skipped when no scratch root was created")
	require.Equal(t, hostFile, rs.EffectiveMounts[0].HostPath)
}

func indexOf(args []string, needle string) int {
	for i, a := range args {
		if a == needle {
			return i
		}
	}
	return -1
}
