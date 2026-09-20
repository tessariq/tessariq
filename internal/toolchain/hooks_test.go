package toolchain

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHookInstallationUsesCommonGitDirectory(t *testing.T) {
	t.Parallel()

	commands := stringSlice(nodeAt(loadYAML(t, taskfile), "tasks", "hooks:install", "cmds"))
	install := strings.Join(commands, "\n")
	resolve := `hooks_dir="$(cd "$(git rev-parse --git-common-dir)" && pwd)/hooks"`
	configure := `git config --local core.hooksPath "$hooks_dir"`
	require.Contains(t, install, resolve)
	require.Contains(t, install, configure)
	require.Less(t, strings.Index(install, resolve), strings.Index(install, configure))
	require.Less(t, strings.Index(install, configure), strings.Index(install, "lefthook install"))
}

func TestSetupEntrypointsUseHookInstallationTask(t *testing.T) {
	t.Parallel()

	for _, path := range []string{filepath.Join(repoRoot, ".agents", "setup"), miseConfig} {
		content := readFile(t, path)
		require.Contains(t, content, "task hooks:install", path)
		require.NotContains(t, content, "lefthook install", path)
		require.NotContains(t, content, "core.hooksPath", path)
	}
}
