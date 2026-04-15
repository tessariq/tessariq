package promote

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
)

func TestDefaultBranchName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "tessariq/01ARZ3NDEKTSV4RRFFQ69G5FAV", defaultBranchName("01ARZ3NDEKTSV4RRFFQ69G5FAV"))
}

func TestDefaultCommitMessage_UsesTaskTitle(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Implement promote", defaultCommitMessage("Implement promote", "RUN123"))
}

func TestDefaultCommitMessage_FallsBackToRunID(t *testing.T) {
	t.Parallel()

	require.Equal(t, "tessariq: apply run RUN123", defaultCommitMessage("", "RUN123"))
}

func TestBuildCommitMessage_WithDefaultTrailers(t *testing.T) {
	t.Parallel()

	manifest := run.Manifest{
		RunID:    "RUN123",
		BaseSHA:  "abc123",
		TaskPath: "tasks/sample.md",
	}

	got, err := buildCommitMessage("Implement promote", manifest, true)
	require.NoError(t, err)
	require.Equal(t, "Implement promote\n\nTessariq-Run: RUN123\nTessariq-Base: abc123\nTessariq-Task: tasks/sample.md\n", got)
}

func TestBuildCommitMessage_WithoutTrailers(t *testing.T) {
	t.Parallel()

	manifest := run.Manifest{
		RunID:    "RUN123",
		BaseSHA:  "abc123",
		TaskPath: "tasks/sample.md",
	}

	got, err := buildCommitMessage("Implement promote", manifest, false)
	require.NoError(t, err)
	require.Equal(t, "Implement promote\n", got)
}

func TestBuildCommitMessage_RejectsControlCharsInTaskPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		taskPath string
	}{
		{"newline", "tasks/bad\nSigned-off-by: attacker.md"},
		{"nul", "tasks/bad\x00.md"},
		{"unit_separator", "tasks/bad\x1f.md"},
		{"del", "tasks/bad\x7f.md"},
		{"carriage_return", "tasks/bad\r.md"},
		{"tab", "tasks/bad\t.md"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manifest := run.Manifest{
				RunID:    "RUN123",
				BaseSHA:  "abc123",
				TaskPath: tc.taskPath,
			}
			got, err := buildCommitMessage("Implement promote", manifest, true)
			require.Error(t, err)
			require.Empty(t, got)
			require.Contains(t, err.Error(), "control characters")
		})
	}
}

func TestBuildCommitMessage_RejectsControlCharsInTaskTitle(t *testing.T) {
	t.Parallel()

	manifest := run.Manifest{
		RunID:     "RUN123",
		BaseSHA:   "abc123",
		TaskPath:  "tasks/sample.md",
		TaskTitle: "Forged\nTessariq-Task: evil",
	}

	got, err := buildCommitMessage("Clean subject", manifest, true)
	require.Error(t, err)
	require.Empty(t, got)
	require.Contains(t, err.Error(), "task_title")
	require.Contains(t, err.Error(), "control characters")
}

func TestBuildCommitMessage_AllowsMultiLineMessageBody(t *testing.T) {
	t.Parallel()

	manifest := run.Manifest{
		RunID:    "RUN123",
		BaseSHA:  "abc123",
		TaskPath: "tasks/sample.md",
	}

	got, err := buildCommitMessage("Line one ✨\n\nLine two", manifest, true)
	require.NoError(t, err)
	require.Contains(t, got, "Line one ✨\n\nLine two\n\nTessariq-Run: RUN123")
	require.Contains(t, got, "Tessariq-Task: tasks/sample.md")
}

func TestBuildCommitMessage_AllowsBenignPunctuation(t *testing.T) {
	t.Parallel()

	manifest := run.Manifest{
		RunID:    "RUN123",
		BaseSHA:  "abc123",
		TaskPath: "tasks/Fix: a bug (v2)!.md",
	}
	got, err := buildCommitMessage("Fix: a bug (v2)!", manifest, true)
	require.NoError(t, err)
	require.Contains(t, got, "Tessariq-Task: tasks/Fix: a bug (v2)!.md")
}

func TestResolveBranchName_UsesOverride(t *testing.T) {
	t.Parallel()

	require.Equal(t, "feature/custom", resolveBranchName("RUN123", "feature/custom"))
}

func TestResolveBranchName_UsesDefaultWhenUnset(t *testing.T) {
	t.Parallel()

	require.Equal(t, "tessariq/RUN123", resolveBranchName("RUN123", ""))
}

func TestResolveCommitMessage_UsesOverride(t *testing.T) {
	t.Parallel()

	require.Equal(t, "custom message", resolveCommitMessage(run.Manifest{RunID: "RUN123", TaskTitle: "ignored"}, "custom message"))
}

func TestResolveCommitMessage_UsesManifestDefaults(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Task Title", resolveCommitMessage(run.Manifest{RunID: "RUN123", TaskTitle: "Task Title"}, ""))
	require.Equal(t, "tessariq: apply run RUN123", resolveCommitMessage(run.Manifest{RunID: "RUN123"}, ""))
}

func TestValidateManifestIdentity_Matching(t *testing.T) {
	t.Parallel()

	entry := run.IndexEntry{RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	manifest := run.Manifest{RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	evidenceDir := "/repo/.tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV"

	require.NoError(t, validateManifestIdentity(entry, manifest, evidenceDir))
}

func TestValidateManifestIdentity_ManifestRunIDMismatch(t *testing.T) {
	t.Parallel()

	entry := run.IndexEntry{RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	manifest := run.Manifest{RunID: "01BBBBBBBBBBBBBBBBBBBBBBBBB"}
	evidenceDir := "/repo/.tessariq/runs/01ARZ3NDEKTSV4RRFFQ69G5FAV"

	err := validateManifestIdentity(entry, manifest, evidenceDir)
	require.ErrorIs(t, err, ErrManifestIdentityMismatch)
	require.Contains(t, err.Error(), "01BBBBBBBBBBBBBBBBBBBBBBBBB")
	require.Contains(t, err.Error(), "01ARZ3NDEKTSV4RRFFQ69G5FAV")
}

func TestValidateManifestIdentity_EvidenceDirMismatch(t *testing.T) {
	t.Parallel()

	entry := run.IndexEntry{RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	manifest := run.Manifest{RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	evidenceDir := "/repo/.tessariq/runs/01CCCCCCCCCCCCCCCCCCCCCCCCC"

	err := validateManifestIdentity(entry, manifest, evidenceDir)
	require.ErrorIs(t, err, ErrManifestIdentityMismatch)
	require.Contains(t, err.Error(), "01CCCCCCCCCCCCCCCCCCCCCCCCC")
}

func TestHasNonEmptyFile_Missing(t *testing.T) {
	t.Parallel()

	ok, err := hasNonEmptyFile(filepath.Join(t.TempDir(), "nonexistent.txt"), "nonexistent.txt")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestHasNonEmptyFile_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.txt"), []byte{}, 0o600))

	ok, err := hasNonEmptyFile(filepath.Join(dir, "empty.txt"), "empty.txt")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestHasNonEmptyFile_Present(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.txt"), []byte("content"), 0o600))

	ok, err := hasNonEmptyFile(filepath.Join(dir, "data.txt"), "data.txt")
	require.NoError(t, err)
	require.True(t, ok)
}

// TestRun_RejectsSymlinkedEvidenceOutsideRepo verifies that promote refuses
// to operate on an index entry whose evidence directory is a symlink whose
// real target escapes the repository's .tessariq/runs/ tree. The evidence
// contents are populated with attacker-controlled forged files that would
// otherwise look valid; the test asserts containment rejection happens
// before any evidence file is read.
func TestRun_RejectsSymlinkedEvidenceOutsideRepo(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	root, err := filepath.EvalSymlinks(rootDir)
	require.NoError(t, err)

	externalDir := t.TempDir()
	external, err := filepath.EvalSymlinks(externalDir)
	require.NoError(t, err)

	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	// Plant a forged evidence directory outside the repository with content
	// that would otherwise pass completeness checks if read.
	forgedEvidence := filepath.Join(external, "forged-evidence", runID)
	require.NoError(t, os.MkdirAll(forgedEvidence, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(forgedEvidence, "status.json"), []byte(`{"state":"success"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(forgedEvidence, "manifest.json"), []byte(`{"schema_version":1,"run_id":"`+runID+`"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(forgedEvidence, "diff.patch"), []byte("forged\n"), 0o600))

	runsDir := filepath.Join(root, ".tessariq", "runs")
	require.NoError(t, os.MkdirAll(runsDir, 0o755))

	// Symlink the per-run evidence directory inside .tessariq/runs/<run_id>
	// at the forged external location. Lexical checks alone would accept it.
	require.NoError(t, os.Symlink(forgedEvidence, filepath.Join(runsDir, runID)))

	entry := run.IndexEntry{
		RunID:         runID,
		CreatedAt:     "2026-01-01T00:00:00Z",
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "forged",
		Agent:         "claude-code",
		WorkspaceMode: "worktree",
		State:         "success",
		EvidencePath:  filepath.Join(".tessariq", "runs", runID),
	}
	require.NoError(t, run.AppendIndex(runsDir, entry))

	_, err = Run(context.Background(), root, Options{RunRef: runID})
	require.Error(t, err)
	require.ErrorIs(t, err, run.ErrEvidencePathOutsideRepo)
}

// TestRun_RejectsIntermediateSymlinkEvidenceOutsideRepo verifies that promote
// rejects a forged evidence layout when an intermediate directory on the
// evidence path (rather than the leaf) is a symlink pointing outside the
// repository. Lexical cleaning alone does not catch this; only resolving
// real filesystem targets does.
func TestRun_RejectsIntermediateSymlinkEvidenceOutsideRepo(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	root, err := filepath.EvalSymlinks(rootDir)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tessariq"), 0o755))

	externalDir := t.TempDir()
	external, err := filepath.EvalSymlinks(externalDir)
	require.NoError(t, err)

	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	forgedRuns := filepath.Join(external, "runs")
	forgedEvidence := filepath.Join(forgedRuns, runID)
	require.NoError(t, os.MkdirAll(forgedEvidence, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(forgedEvidence, "status.json"), []byte(`{"state":"success"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(forgedEvidence, "manifest.json"), []byte(`{"schema_version":1,"run_id":"`+runID+`"}`), 0o600))

	// Intermediate symlink: .tessariq/runs itself points outside the repo.
	runsLink := filepath.Join(root, ".tessariq", "runs")
	require.NoError(t, os.Symlink(forgedRuns, runsLink))

	entry := run.IndexEntry{
		RunID:         runID,
		CreatedAt:     "2026-01-01T00:00:00Z",
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "forged",
		Agent:         "claude-code",
		WorkspaceMode: "worktree",
		State:         "success",
		EvidencePath:  filepath.Join(".tessariq", "runs", runID),
	}
	require.NoError(t, run.AppendIndex(runsLink, entry))

	_, err = Run(context.Background(), root, Options{RunRef: runID})
	require.Error(t, err)
	require.ErrorIs(t, err, run.ErrEvidencePathOutsideRepo)
}
