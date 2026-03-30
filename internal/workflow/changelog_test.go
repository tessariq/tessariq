package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGitStatusPorcelain(t *testing.T) {
	t.Parallel()

	output := " M cmd/tessariq/run.go\n?? internal/workflow/changelog.go\nR  internal/workflow/old.go -> internal/workflow/service.go\n?? \"docs/with space.md\"\n"
	got := parseGitStatusPorcelain(output)

	require.Equal(t, []string{
		"cmd/tessariq/run.go",
		"docs/with space.md",
		"internal/workflow/changelog.go",
		"internal/workflow/service.go",
	}, got)
}

func TestRequiresChangelogUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		changedFiles []string
		wantRequired bool
		wantEvidence []string
	}{
		{
			name:         "requires changelog for user-visible cmd change",
			changedFiles: []string{"cmd/tessariq/run.go"},
			wantRequired: true,
			wantEvidence: []string{"cmd/tessariq/run.go"},
		},
		{
			name:         "does not require changelog for workflow cli change",
			changedFiles: []string{"cmd/tessariq-workflow/main.go"},
			wantRequired: false,
		},
		{
			name:         "does not require changelog for workflow service change",
			changedFiles: []string{"internal/workflow/service.go"},
			wantRequired: false,
		},
		{
			name:         "requires changelog for non-workflow internal non-test change",
			changedFiles: []string{"internal/runner/runner.go"},
			wantRequired: true,
			wantEvidence: []string{"internal/runner/runner.go"},
		},
		{
			name:         "does not require when changelog already touched",
			changedFiles: []string{"internal/workflow/service.go", "CHANGELOG.md"},
			wantRequired: false,
		},
		{
			name:         "does not require for tests and planning only",
			changedFiles: []string{"internal/workflow/service_test.go", "planning/STATE.md", "docs/workflow/development-workflow.md"},
			wantRequired: false,
		},
		{
			name:         "does not require for internal testutil",
			changedFiles: []string{"internal/testutil/containers/runenv.go"},
			wantRequired: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			required, evidence := requiresChangelogUpdate(tt.changedFiles)
			require.Equal(t, tt.wantRequired, required)
			require.Equal(t, tt.wantEvidence, evidence)
		})
	}
}

func TestBuildChangelogFindings(t *testing.T) {
	t.Parallel()

	t.Run("adds medium finding when required", func(t *testing.T) {
		t.Parallel()

		findings := buildChangelogFindings("TASK-123-example", []string{"cmd/tessariq/run.go"})
		require.Len(t, findings, 1)
		require.Equal(t, "TASK-123-example-changelog", findings[0].ID)
		require.Equal(t, "user-visible changes missing changelog update", findings[0].Title)
		require.Equal(t, "medium", findings[0].Severity)
		require.Equal(t, "open", findings[0].Status)
		require.Equal(t, "TASK-123-example", findings[0].TaskID)
		require.Contains(t, findings[0].Details, "CHANGELOG.md")
	})

	t.Run("returns no findings when changelog touched", func(t *testing.T) {
		t.Parallel()

		findings := buildChangelogFindings("TASK-123-example", []string{"cmd/tessariq/run.go", "CHANGELOG.md"})
		require.Empty(t, findings)
	})
}

func TestBuildTaskFindings_AddsChangelogNudgeForTaskProfile(t *testing.T) {
	t.Parallel()

	scope := specScope{
		Milestone: "v0.1.0",
		Version:   "v0.1.0",
		Path:      "specs/tessariq-v0.1.0.md",
	}

	task := taskForTest("TASK-123-example", "todo", "p1", nil)
	specDocs, violations := loadReferencedSpecDocs(testRepoRoot(t), []*Task{task})
	require.Empty(t, violations)

	findings := buildTaskFindings(
		&State{Frontmatter: StateFrontmatter{MilestoneFocus: scope.Milestone, ActiveSpecVersion: scope.Version, ActiveSpecPath: scope.Path}},
		[]*Task{task},
		VerifyInput{Profile: "task", TaskID: task.Frontmatter.ID},
		scope,
		specDocs,
		nil,
		[]string{"cmd/tessariq/run.go"},
	)

	require.Len(t, findings, 1)
	require.Equal(t, "user-visible changes missing changelog update", findings[0].Title)
	require.Equal(t, "medium", findings[0].Severity)

	findings = buildTaskFindings(
		&State{Frontmatter: StateFrontmatter{MilestoneFocus: scope.Milestone, ActiveSpecVersion: scope.Version, ActiveSpecPath: scope.Path}},
		[]*Task{task},
		VerifyInput{Profile: "spec", TaskID: task.Frontmatter.ID},
		scope,
		specDocs,
		nil,
		[]string{"cmd/tessariq/run.go"},
	)
	require.Empty(t, findings)
}
