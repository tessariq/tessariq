//go:build e2e

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	versioninfo "github.com/tessariq/tessariq/internal/version"
)

// TestE2E_VersionWorksOutsideGitRepository covers manual case T4.24: both
// version forms must answer from any working directory. Reporting the build
// version is the one thing a user does before they have a repository — while
// checking an install, filing a bug, or confirming what a release archive
// shipped — so neither form may depend on repository discovery.
func TestE2E_VersionWorksOutsideGitRepository(t *testing.T) {
	t.Parallel()
	env := setupInitEnv(t)

	ctx := context.Background()
	binPath := fmt.Sprintf("%s/tessariq", env.Dir())
	expected := "tessariq v" + versioninfo.Version

	// /tmp inside the container has no .git anywhere up the tree, so
	// repository discovery would fail if either form attempted it.
	nonGitDir := "/tmp"
	_, out, err := env.Exec(ctx, []string{"sh", "-c",
		fmt.Sprintf("cd %s && git rev-parse --show-toplevel 2>&1 || true", nonGitDir)})
	require.NoError(t, err)
	require.Contains(t, out, "not a git repository",
		"fixture precondition: %s must not be inside a git repository", nonGitDir)

	for _, args := range []string{"version", "--version"} {
		t.Run(args, func(t *testing.T) {
			code, out, err := env.Exec(ctx, []string{"sh", "-c",
				fmt.Sprintf("cd %s && %s %s", nonGitDir, binPath, args)})
			require.NoError(t, err)
			require.Equal(t, 0, code, "%s must succeed outside a repository: %s", args, out)
			require.Equal(t, expected, strings.TrimSpace(out))
		})
	}
}
