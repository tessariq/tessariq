package toolchain

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The mirrored agent skill trees are vendored: every SKILL.md is a byte-exact
// copy of the skill package embedded in the taskrail release pinned in mise.toml,
// materialized by scripts/vendor-skills.sh. Nothing here is locally authored.
//
// The invariant is easy to break silently. `taskrail init --with-skills` never
// overwrites an existing file, so a stale tree survives a version bump without a
// word; with --force it writes SKILL.md.bak.<timestamp> siblings into *both*
// mirrors. These tests keep local drift loud under plain `go test ./...`, while
// task workflow:check-skills also compares both trees with the pinned package.
var (
	skillTrees   = []string{filepath.Join(repoRoot, ".agents", "skills"), filepath.Join(repoRoot, ".claude", "skills")}
	skillFile    = "SKILL.md"
	skillsMani   = filepath.Join(repoRoot, "docs", "workflow", "skills-manifest.yml")
	miseConfig   = filepath.Join(repoRoot, "mise.toml")
	taskfile     = filepath.Join(repoRoot, "Taskfile.yml")
	taskrailPin  = regexp.MustCompile(`"go:github\.com/tessariq/taskrail/cmd/taskrail"\s*=\s*"([^"]+)"`)
	skillNameKey = regexp.MustCompile(`(?m)^name:\s*(\S+)\s*$`)
)

func TestSkillsCheckComparesPinnedPackage(t *testing.T) {
	t.Parallel()

	require.Contains(t, readFile(t, taskfile), "./scripts/vendor-skills.sh --check",
		"workflow:check-skills must compare the vendored trees with the pinned taskrail package")
}

// skillsManifest is the generated provenance record read by these tests.
type skillsManifest struct {
	SchemaVersion   int    `yaml:"schema_version"`
	TaskrailVersion string `yaml:"taskrail_version"`
	Skills          []struct {
		Name   string `yaml:"name"`
		SHA256 string `yaml:"sha256"`
	} `yaml:"skills"`
}

// TestSkillsMirrorsAreByteIdentical keeps the mirror invariant under a plain
// `go test ./...`, without requiring the package download used by the workflow
// check.
func TestSkillsMirrorsAreByteIdentical(t *testing.T) {
	t.Parallel()

	agents := skillDigests(t, skillTrees[0])
	claude := skillDigests(t, skillTrees[1])

	require.Equal(t, agents, claude,
		"the mirrored skill trees must stay byte-identical; re-vendor with `task workflow:skills:vendor`")
}

// TestSkillsMatchManifestDigests catches a hand-edited skill. Editing a vendored
// file is always a mistake: the change is lost at the next re-vendor, and until
// then it silently diverges from the package the running taskrail embeds, which
// is what re-earns the standing version-skew warning this repository vendors its
// skills to avoid.
func TestSkillsMatchManifestDigests(t *testing.T) {
	t.Parallel()

	manifest := loadSkillsManifest(t)
	require.NotEmpty(t, manifest.Skills, "%s must record at least one skill", skillsMani)

	want := make(map[string]string, len(manifest.Skills))
	for _, skill := range manifest.Skills {
		want[skill.Name] = skill.SHA256
	}

	for _, tree := range skillTrees {
		got := skillDigests(t, tree)
		require.Equal(t, want, got,
			"skills in %s do not match %s; re-vendor with `task workflow:skills:vendor`", tree, skillsMani)
	}
}

// TestSkillsManifestTracksPinnedTaskrail ties the vendored trees to the toolchain
// pin. Bumping mise.toml without re-vendoring is the drift that put these trees in
// a provenance limbo the first time: the binary moves on, the skills do not, and
// nothing reports it.
func TestSkillsManifestTracksPinnedTaskrail(t *testing.T) {
	t.Parallel()

	match := taskrailPin.FindStringSubmatch(readFile(t, miseConfig))
	require.Len(t, match, 2, "mise.toml must pin the taskrail CLI")

	require.Equal(t, match[1], loadSkillsManifest(t).TaskrailVersion,
		"%s records a different taskrail version than mise.toml pins; re-vendor with `task workflow:skills:vendor`", skillsMani)
}

// TestSkillsCarryNoVersionMarker guards the reason the skills are copied from the
// module cache instead of installed with `taskrail init --with-skills`. taskrail
// suppresses version-skew warnings for a skill that is unstamped *and*
// byte-identical to the package the running binary embeds (isPackageParityCopy in
// its internal/taskrail/skills_skew.go). A stamped copy loses that exemption, and
// because the marker records the writing binary's version — 0.0.0-dev for any
// `go install` build, which is what the mise backend produces — a stamped tree
// makes every release-built taskrail report skew against it.
func TestSkillsCarryNoVersionMarker(t *testing.T) {
	t.Parallel()

	for _, tree := range skillTrees {
		for _, name := range skillNames(t, tree) {
			path := filepath.Join(tree, name, skillFile)
			require.NotContains(t, readFile(t, path), "taskrail_version:",
				"%s must stay an unstamped copy of the embedded package; install it with `task workflow:skills:vendor`, never `taskrail init --with-skills`", path)
		}
	}
}

// TestSkillsFrontmatterNameMatchesDirectory catches a half-finished rename or a
// hand-rolled skill: agent tools resolve a skill by its frontmatter name, so a
// mismatch loads nothing while the directory still looks present.
func TestSkillsFrontmatterNameMatchesDirectory(t *testing.T) {
	t.Parallel()

	for _, tree := range skillTrees {
		for _, name := range skillNames(t, tree) {
			path := filepath.Join(tree, name, skillFile)
			match := skillNameKey.FindStringSubmatch(readFile(t, path))
			require.Len(t, match, 2, "%s must declare a frontmatter name", path)
			require.Equal(t, name, match[1], "%s declares a name that is not its directory", path)
		}
	}
}

// TestSkillsHaveNoBackupSiblings gives a focused failure when taskrail init
// --with-skills --force leaves backups in both otherwise-identical mirrors. The
// backups are deliberately not gitignored.
func TestSkillsHaveNoBackupSiblings(t *testing.T) {
	t.Parallel()

	for _, tree := range skillTrees {
		var found []string
		require.NoError(t, filepath.WalkDir(tree, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.Contains(d.Name(), ".bak.") {
				found = append(found, path)
			}
			return nil
		}))

		require.Empty(t, found,
			"skill backups must not be committed; delete them and re-vendor with `task workflow:skills:vendor`")
	}
}

// skillDigests maps every skill directory in one tree to the SHA-256 of its
// SKILL.md, which is the only file the vendored package ships per skill.
func skillDigests(t *testing.T, tree string) map[string]string {
	t.Helper()

	digests := map[string]string{}
	for _, name := range skillNames(t, tree) {
		sum := sha256.Sum256([]byte(readFile(t, filepath.Join(tree, name, skillFile))))
		digests[name] = hex.EncodeToString(sum[:])
	}
	return digests
}

// skillNames returns the sorted skill directory names in one tree.
func skillNames(t *testing.T, tree string) []string {
	t.Helper()

	entries, err := os.ReadDir(tree)
	require.NoError(t, err, "read %s", tree)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	require.NotEmpty(t, names, "%s must contain vendored skills", tree)

	sort.Strings(names)
	return names
}

func loadSkillsManifest(t *testing.T) skillsManifest {
	t.Helper()

	var manifest skillsManifest
	require.NoError(t, yaml.Unmarshal([]byte(readFile(t, skillsMani)), &manifest), "parse %s", skillsMani)
	return manifest
}
