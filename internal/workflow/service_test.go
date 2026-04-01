package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateStateAndTasksRejectsMissingTaskSections(t *testing.T) {
	t.Parallel()

	state := &State{
		Frontmatter: StateFrontmatter{
			SchemaVersion:     1,
			UpdatedAt:         time.Now().UTC().Format(timeLayout),
			MilestoneFocus:    "v0.1.0",
			ActiveSpecVersion: "v0.1.0",
			ActiveSpecPath:    "specs/tessariq-v0.1.0.md",
			StaleAfterMinutes: 180,
			MaxRetries:        2,
		},
	}
	task := &Task{
		Frontmatter: TaskFrontmatter{
			ID:          "TASK-001-example",
			Title:       "Example",
			Status:      "todo",
			Priority:    "p1",
			Milestone:   "v0.1.0",
			SpecVersion: "v0.1.0",
			SpecRefs:    []string{"specs/tessariq-v0.1.0.md#tessariq-run-task-path"},
			Verification: TaskVerification{
				Unit:        VerificationTier{Rationale: "required"},
				Integration: VerificationTier{Rationale: "considered"},
				E2E:         VerificationTier{Rationale: "considered"},
				Mutation:    VerificationTier{Rationale: "considered"},
				ManualTest:  VerificationTier{Rationale: "considered"},
			},
		},
		Body: "## Summary\n",
	}

	violations := validateStateAndTasks(state, []*Task{task}, time.Now().UTC(), testRepoRoot(t))
	require.NotEmpty(t, violations)
	require.Contains(t, strings.Join(violations, "\n"), "missing sections")
}

func TestValidateStateAndTasksRejectsBrokenState(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	state := &State{
		Frontmatter: StateFrontmatter{
			SchemaVersion:       0,
			UpdatedAt:           now.Format(timeLayout),
			ActiveTask:          "TASK-001-a",
			ActiveTaskStartedAt: now.Add(-4 * time.Hour).Format(timeLayout),
			MilestoneFocus:      "v0.1.0",
			ActiveSpecVersion:   "v0.1.0",
			ActiveSpecPath:      "specs/tessariq-v0.1.0.md",
			StaleAfterMinutes:   180,
			MaxRetries:          0,
		},
	}
	tasks := []*Task{
		taskForTest("TASK-001-a", "todo", "p1", nil),
		taskForTest("TASK-002-b", "in_progress", "p2", nil),
	}
	tasks[1].Frontmatter.DependsOn = []string{"TASK-999-missing"}

	violations := validateStateAndTasks(state, tasks, now, testRepoRoot(t))
	joined := strings.Join(violations, "\n")
	require.Contains(t, joined, "state schema_version")
	require.Contains(t, joined, "state max_retries")
	require.Contains(t, joined, "active task TASK-001-a is not marked in_progress")
	require.Contains(t, joined, "active task TASK-001-a is stale")
	require.Contains(t, joined, "depends on unknown task")
}

func TestEligibleTasksSortsByPriorityThenID(t *testing.T) {
	t.Parallel()

	tasks := []*Task{
		taskForTest("TASK-002-b", "todo", "p1", nil),
		taskForTest("TASK-001-a", "todo", "p1", nil),
		taskForTest("TASK-003-c", "todo", "p0", nil),
	}

	eligible := eligibleTasks(tasks)
	require.Len(t, eligible, 3)
	require.Equal(t, "TASK-003-c", eligible[0].Frontmatter.ID)
	require.Equal(t, "TASK-001-a", eligible[1].Frontmatter.ID)
	require.Equal(t, "TASK-002-b", eligible[2].Frontmatter.ID)
}

func TestSelectNextTaskRecoversStaleActiveTask(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-4 * time.Hour).Format(timeLayout)
	state := &State{
		Frontmatter: StateFrontmatter{
			SchemaVersion:       1,
			UpdatedAt:           time.Now().UTC().Format(timeLayout),
			ActiveTask:          "TASK-001-a",
			ActiveTaskStartedAt: startedAt,
			Attempt:             0,
			MilestoneFocus:      "v0.1.0",
			StaleAfterMinutes:   180,
			MaxRetries:          2,
		},
	}
	tasks := []*Task{
		taskForTest("TASK-001-a", "in_progress", "p1", nil),
		taskForTest("TASK-002-b", "todo", "p1", nil),
	}

	result, updatedState, updatedTasks, err := selectNextTask(state, tasks, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, result.Recovered)
	require.Equal(t, "TASK-001-a", result.SelectedTask)
	require.Equal(t, "", updatedState.Frontmatter.ActiveTask)
	require.Equal(t, "todo", updatedTasks[0].Frontmatter.Status)
}

func TestSelectNextTaskKeepsFreshActiveTask(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	state := &State{
		Frontmatter: StateFrontmatter{
			SchemaVersion:       1,
			UpdatedAt:           now.Format(timeLayout),
			ActiveTask:          "TASK-001-a",
			ActiveTaskStartedAt: now.Add(-10 * time.Minute).Format(timeLayout),
			StaleAfterMinutes:   180,
			MaxRetries:          2,
		},
	}
	tasks := []*Task{
		taskForTest("TASK-001-a", "in_progress", "p1", nil),
		taskForTest("TASK-002-b", "todo", "p0", nil),
	}

	result, updatedState, _, err := selectNextTask(state, tasks, now)
	require.NoError(t, err)
	require.False(t, result.Recovered)
	require.Equal(t, "TASK-001-a", result.SelectedTask)
	require.Equal(t, "TASK-001-a", updatedState.Frontmatter.ActiveTask)
	require.Equal(t, "continue active task", result.Reason)
}

func TestSelectNextTaskBlocksAfterRepeatedStaleRecovery(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	state := &State{
		Frontmatter: StateFrontmatter{
			SchemaVersion:       1,
			UpdatedAt:           now.Format(timeLayout),
			ActiveTask:          "TASK-001-a",
			ActiveTaskStartedAt: now.Add(-4 * time.Hour).Format(timeLayout),
			Attempt:             1,
			StaleAfterMinutes:   180,
			MaxRetries:          2,
		},
	}
	tasks := []*Task{
		taskForTest("TASK-001-a", "in_progress", "p1", nil),
		taskForTest("TASK-002-b", "todo", "p0", nil),
	}

	result, _, updatedTasks, err := selectNextTask(state, tasks, now)
	require.NoError(t, err)
	require.True(t, result.Recovered)
	require.Equal(t, "blocked", updatedTasks[0].Frontmatter.Status)
	require.Equal(t, "TASK-002-b", result.SelectedTask)
}

func TestBuildSpecFindingsReportsMissingCoverage(t *testing.T) {
	t.Parallel()

	tasks := []*Task{
		taskForTest("TASK-001-a", "todo", "p1", []string{"specs/tessariq-v0.1.0.md#tessariq-run-task-path"}),
	}

	findings := buildSpecFindings(tasks, specScope{
		Milestone: "v0.1.0",
		Version:   "v0.1.0",
		Path:      "specs/tessariq-v0.1.0.md",
	}, nil)
	require.NotEmpty(t, findings)
	require.Equal(t, "high", findings[0].Severity)
}

func TestValidateStateAndTasksRejectsDeadSpecRefs(t *testing.T) {
	t.Parallel()

	state := &State{
		Frontmatter: StateFrontmatter{
			SchemaVersion:     1,
			UpdatedAt:         time.Now().UTC().Format(timeLayout),
			MilestoneFocus:    "v0.1.0",
			ActiveSpecVersion: "v0.1.0",
			ActiveSpecPath:    "specs/tessariq-v0.1.0.md",
			StaleAfterMinutes: 180,
			MaxRetries:        2,
		},
	}
	task := taskForTest("TASK-001-a", "todo", "p1", []string{"specs/tessariq-v0.1.0.md#cli-run"})

	violations := validateStateAndTasks(state, []*Task{task}, time.Now().UTC(), testRepoRoot(t))
	require.Contains(t, strings.Join(violations, "\n"), "unknown heading anchor")
}

func TestBuildTaskFindingsReportsVersionMismatchAndDeadRefs(t *testing.T) {
	t.Parallel()

	scope := specScope{
		Milestone: "v0.1.0",
		Version:   "v0.1.0",
		Path:      "specs/tessariq-v0.1.0.md",
	}
	specDocs, violations := loadReferencedSpecDocs(testRepoRoot(t), []*Task{
		taskForTest("TASK-001-a", "todo", "p1", []string{"specs/tessariq-v0.1.0.md#cli-run"}),
	})
	require.Empty(t, violations)
	_, err := loadSpecDocument(testRepoRoot(t), scope)
	require.NoError(t, err)

	task := taskForTest("TASK-001-a", "todo", "p1", []string{"specs/tessariq-v0.1.0.md#cli-run"})
	task.Frontmatter.SpecVersion = "v0.2.0"

	findings := buildTaskFindings(
		&State{Frontmatter: StateFrontmatter{MilestoneFocus: scope.Milestone, ActiveSpecVersion: scope.Version, ActiveSpecPath: scope.Path}},
		[]*Task{task},
		VerifyInput{Profile: "task", TaskID: task.Frontmatter.ID},
		scope,
		specDocs,
		nil,
		nil,
	)

	require.Len(t, findings, 2)
	require.Equal(t, "task spec version does not match its spec refs", findings[0].Title)
	require.Equal(t, "task spec ref points to an unknown heading", findings[1].Title)
}

func TestCompareSkillTreesReportsParityProblems(t *testing.T) {
	t.Parallel()

	left := map[string]string{"autonomous-task.md": "same", "extra.md": "x"}
	right := map[string]string{"autonomous-task.md": "different"}

	mismatches := compareSkillTrees(left, right)
	require.Len(t, mismatches, 2)
}

func TestCandidateTasksCapsAtFive(t *testing.T) {
	t.Parallel()

	var tasks []*Task
	for i := 1; i <= 6; i++ {
		tasks = append(tasks, taskForTest(
			"TASK-00"+string(rune('0'+i))+"-x",
			"todo",
			"p1",
			nil,
		))
	}

	candidates := candidateTasks(tasks)
	require.Len(t, candidates, 5)
}

func TestDependencyHelpers(t *testing.T) {
	t.Parallel()

	done := taskForTest("TASK-001-a", "done", "p1", nil)
	waiting := taskForTest("TASK-002-b", "todo", "p1", nil)
	waiting.Frontmatter.DependsOn = []string{"TASK-001-a"}
	missing := taskForTest("TASK-003-c", "todo", "p1", nil)
	missing.Frontmatter.DependsOn = []string{"TASK-999-z"}

	require.False(t, unresolvedDependency(waiting, []*Task{done, waiting}))
	require.True(t, unresolvedDependency(missing, []*Task{done, missing}))

	_, err := findTask([]*Task{done}, "TASK-999-z")
	require.Error(t, err)
}

func TestBuildTaskFindingsBranches(t *testing.T) {
	t.Parallel()

	task := taskForTest("TASK-001-a", "todo", "p1", []string{})
	task.Body = "## Summary\n\nonly summary\n"
	task.Frontmatter.Verification.Unit.Rationale = ""

	scope := specScope{
		Milestone: "v0.1.0",
		Version:   "v0.1.0",
		Path:      "specs/tessariq-v0.1.0.md",
	}
	specDocs, violations := loadReferencedSpecDocs(testRepoRoot(t), []*Task{task})
	require.Empty(t, violations)

	findings := buildTaskFindings(
		&State{Frontmatter: StateFrontmatter{MilestoneFocus: scope.Milestone, ActiveSpecVersion: scope.Version, ActiveSpecPath: scope.Path}},
		[]*Task{task},
		VerifyInput{Profile: "task", TaskID: "TASK-001-a"},
		scope,
		specDocs,
		nil,
		nil,
	)
	require.Len(t, findings, 3)

	missing := buildTaskFindings(
		&State{Frontmatter: StateFrontmatter{MilestoneFocus: scope.Milestone, ActiveSpecVersion: scope.Version, ActiveSpecPath: scope.Path}},
		[]*Task{},
		VerifyInput{Profile: "task", TaskID: "TASK-404"},
		scope,
		specDocs,
		nil,
		nil,
	)
	require.Len(t, missing, 1)
	require.Equal(t, "high", missing[0].Severity)
}

func TestBuildImplementedFindingsAndSummary(t *testing.T) {
	t.Parallel()

	done := taskForTest("TASK-001-a", "done", "p1", nil)
	done.Frontmatter.Verification.Mutation.Rationale = ""

	findings := buildImplementedFindings([]*Task{done})
	require.Len(t, findings, 1)

	summary := summarizeFindings([]Finding{
		{Severity: "high"},
		{Severity: "medium"},
		{Severity: "low"},
	})
	require.Equal(t, VerificationSummary{High: 1, Medium: 1, Low: 1}, summary)
	require.Equal(t, "failed", validationStatus(summary))
	require.Equal(t, "warnings", validationStatus(VerificationSummary{Medium: 1}))
	require.Equal(t, "passed", validationStatus(VerificationSummary{}))
}

func TestRenderVerificationPlanAndVerifyTarget(t *testing.T) {
	t.Parallel()

	report := VerificationReport{
		Profile:           "spec",
		Disposition:       "report",
		GeneratedAt:       "2026-03-29T00:00:00Z",
		MilestoneFocus:    "v0.1.0",
		ActiveSpecVersion: "v0.1.0",
		ActiveSpecPath:    "specs/tessariq-v0.1.0.md",
		PlanPath:          "/tmp/plan.md",
		ReportPath:        "/tmp/report.json",
	}
	plan := renderVerificationPlan(report)
	require.Contains(t, plan, "Validate seeded tasks cover the active milestone spec")
	require.Equal(t, "spec:v0.1.0", verificationScopeLabel(report))

	state := &State{Frontmatter: StateFrontmatter{ActiveTask: "TASK-001-a"}}
	require.Equal(t, "TASK-123", verifyTarget(VerifyInput{TaskID: "TASK-123"}, state))
	require.Equal(t, "TASK-001-a", verifyTarget(VerifyInput{Profile: "task"}, state))
	require.Equal(t, "sweep", verifyTarget(VerifyInput{Profile: "spec"}, state))
}

func TestFollowupTaskAndHelpers(t *testing.T) {
	t.Parallel()

	task := newFollowupTask(17, Finding{
		ID:          "missing-spec",
		Title:       "Missing Spec Coverage",
		Severity:    "high",
		SpecVersion: "v0.1.0",
		SpecRefs:    []string{"specs/tessariq-v0.1.0.md#tessariq-run-task-path"},
		Details:     "missing",
	}, "v0.1.0")

	require.Equal(t, "TASK-017-missing-spec-coverage", task.Frontmatter.ID)
	require.Equal(t, "p0", task.Frontmatter.Priority)
	require.Equal(t, 18, nextTaskNumber([]*Task{task}))
	require.Equal(t, 17, parseTaskNumber(task.Frontmatter.ID))
	require.Equal(t, "active", repoState("TASK-001"))
	require.Equal(t, "idle", repoState(""))
	require.Equal(t, "item", slugify("`'\""))
	require.Equal(t, "fallback", nonEmpty("", "fallback"))
	require.Equal(t, 1, min(1, 2))
}

func TestAppendTaskNoteAndMissingRationales(t *testing.T) {
	t.Parallel()

	task := taskForTest("TASK-001-a", "todo", "p1", nil)
	task.Body = "## Summary\n\nx\n\n## Acceptance Criteria\n\nx\n\n## Test Expectations\n\nx\n\n## TDD Plan\n\nx\n"
	appendTaskNote(task, "note")
	require.Contains(t, task.Body, "## Notes")
	require.Contains(t, task.Body, "note")

	missing := missingVerificationRationales(TaskVerification{})
	require.ElementsMatch(t, []string{"unit", "integration", "e2e", "mutation", "manual_test"}, missing)
}

func TestPriorityAndSeverityRank(t *testing.T) {
	t.Parallel()

	require.Less(t, priorityRank("p0"), priorityRank("p2"))
	require.Less(t, severityRank("low"), severityRank("high"))
}

func TestCheckManualTestArtifactsRequiresPlanAndReport(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	taskID := "TASK-001-example"

	// No artifacts directory at all → error.
	err := checkManualTestArtifacts(base, taskID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "manual test artifacts missing")

	// Create task directory but no timestamped subdirectory → error.
	taskDir := filepath.Join(base, "manual-test", taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))
	err = checkManualTestArtifacts(base, taskID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "manual test artifacts missing")

	// Create timestamped directory with only plan.md → error.
	tsDir := filepath.Join(taskDir, "20260329T140000Z")
	require.NoError(t, os.MkdirAll(tsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tsDir, "plan.md"), []byte("plan"), 0o644))
	err = checkManualTestArtifacts(base, taskID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "report.md")

	// Add report.md → success.
	require.NoError(t, os.WriteFile(filepath.Join(tsDir, "report.md"), []byte("report"), 0o644))
	err = checkManualTestArtifacts(base, taskID)
	require.NoError(t, err)
}

func TestCheckManualTestArtifactsSkipsForNonDoneStatus(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	taskID := "TASK-001-example"

	// No artifacts, but status is "blocked" → no check needed.
	// This test documents the expectation that the caller (Finish) only
	// invokes checkManualTestArtifacts for status "done".
	err := checkManualTestArtifacts(base, taskID)
	require.Error(t, err, "the function itself always checks; the caller gates on status")
}

func TestVerifyDoesNotPersistArtifactPathsInState(t *testing.T) {
	t.Parallel()

	svc, paths := newTestService(t)

	result, err := svc.Verify(VerifyInput{Profile: "spec", Disposition: "report"})
	require.NoError(t, err)
	require.NotEmpty(t, result.ReportPath)

	stateData, err := os.ReadFile(paths.StateFile)
	require.NoError(t, err)
	require.NotContains(t, string(stateData), "validation_plan")
	require.NotContains(t, string(stateData), "validation_report")
	require.Contains(t, string(stateData), "validation_last_run")
	require.Contains(t, string(stateData), "validation_checked_at")
}

func TestCreateFollowupsLoadsLatestLocalReportForRecordedRun(t *testing.T) {
	t.Parallel()

	svc, paths := newTestService(t)
	state, err := svc.loadState()
	require.NoError(t, err)

	generatedAt := "2026-04-01T16:00:00Z"
	state.Frontmatter.ValidationLastRun = generatedAt
	state.Frontmatter.ValidationStatus = "failed"
	state.Frontmatter.ValidationScope = "task:v0.1.0"
	state.Frontmatter.ValidationCheckedAt = generatedAt
	require.NoError(t, svc.saveState(state))

	artifactDir := filepath.Join(paths.ArtifactsDir, "verify", "task", "TASK-001-example", "20260401T160000Z")
	require.NoError(t, os.MkdirAll(artifactDir, 0o755))
	report := VerificationReport{
		SchemaVersion:     reportSchemaVersion,
		Profile:           "task",
		Disposition:       "report",
		GeneratedAt:       generatedAt,
		MilestoneFocus:    "v0.1.0",
		ActiveSpecVersion: "v0.1.0",
		ActiveSpecPath:    "specs/tessariq-v0.1.0.md",
		ArtifactDir:       artifactDir,
		PlanPath:          filepath.Join(artifactDir, "plan.md"),
		ReportPath:        filepath.Join(artifactDir, "report.json"),
		Findings: []Finding{{
			ID:          "missing-changelog",
			Title:       "Missing changelog",
			Severity:    "medium",
			Status:      "open",
			SpecVersion: "v0.1.0",
			SpecRefs:    []string{"specs/tessariq-v0.1.0.md#cli-run"},
			Details:     "update changelog",
		}},
	}
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(report.ReportPath, reportJSON, 0o644))

	result, err := svc.CreateFollowups(FollowupsInput{Mode: "create", MinSeverity: "medium"})
	require.NoError(t, err)
	require.Equal(t, relPath(paths.RepoRoot, report.ReportPath), result.ReportPath)
	require.Len(t, result.CreatedTaskIDs, 1)

	createdTask := filepath.Join(paths.TasksDir, result.CreatedTaskIDs[0]+".md")
	_, err = os.Stat(createdTask)
	require.NoError(t, err)
}

func newTestService(t *testing.T) (*Service, Paths) {
	t.Helper()

	repoRoot := t.TempDir()
	planningDir := filepath.Join(repoRoot, "planning")
	tasksDir := filepath.Join(planningDir, "tasks")
	artifactsDir := filepath.Join(planningDir, "artifacts")
	specsDir := filepath.Join(repoRoot, "specs")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))
	require.NoError(t, os.MkdirAll(specsDir, 0o755))

	specData, err := os.ReadFile(filepath.Join(testRepoRoot(t), "specs", "tessariq-v0.1.0.md"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "tessariq-v0.1.0.md"), specData, 0o644))

	state := &State{Frontmatter: StateFrontmatter{
		SchemaVersion:       stateSchemaVersion,
		UpdatedAt:           "2026-04-01T15:00:00Z",
		Mode:                "user_request",
		RepoState:           "idle",
		SelectionReason:     "next eligible todo by priority",
		MilestoneFocus:      "v0.1.0",
		ActiveSpecVersion:   "v0.1.0",
		ActiveSpecPath:      "specs/tessariq-v0.1.0.md",
		StaleAfterMinutes:   180,
		MaxRetries:          2,
		ValidationStatus:    "not_run",
		ValidationCheckedAt: "",
		NextTasks:           []string{"TASK-001-example"},
	}}
	state.Body = renderStateSnapshot(state.Frontmatter, []*Task{taskForTest("TASK-001-example", "todo", "p1", nil)})
	stateData, err := marshalFrontmatter(state.Frontmatter, state.Body)
	require.NoError(t, err)

	stateFile := filepath.Join(planningDir, "STATE.md")
	require.NoError(t, os.WriteFile(stateFile, stateData, 0o644))

	task := taskForTest("TASK-001-example", "todo", "p1", nil)
	task.Filename = filepath.Join(tasksDir, task.Frontmatter.ID+".md")
	taskData, err := marshalFrontmatter(task.Frontmatter, task.Body)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(task.Filename, taskData, 0o644))

	paths := Paths{
		RepoRoot:     repoRoot,
		PlanningDir:  planningDir,
		StateFile:    stateFile,
		TasksDir:     tasksDir,
		ArtifactsDir: artifactsDir,
		AgentSkills:  filepath.Join(repoRoot, ".agents", "skills"),
		ClaudeSkills: filepath.Join(repoRoot, ".claude", "skills"),
		PrimarySpec:  filepath.Join(specsDir, "tessariq-v0.1.0.md"),
	}

	return &Service{paths: paths}, paths
}

func TestValidateStateAndTasksRejectsMissingSpecFile(t *testing.T) {
	t.Parallel()

	state := &State{
		Frontmatter: StateFrontmatter{
			SchemaVersion:     1,
			UpdatedAt:         time.Now().UTC().Format(timeLayout),
			MilestoneFocus:    "v0.1.0",
			ActiveSpecVersion: "v0.1.0",
			ActiveSpecPath:    "specs/tessariq-v0.1.0.md",
			StaleAfterMinutes: 180,
			MaxRetries:        2,
		},
	}
	task := taskForTest("TASK-001-a", "todo", "p1", []string{"specs/tessariq-v9.9.9.md#release-intent"})
	task.Frontmatter.SpecVersion = "v9.9.9"

	violations := validateStateAndTasks(state, []*Task{task}, time.Now().UTC(), testRepoRoot(t))
	joined := strings.Join(violations, "\n")
	require.Contains(t, joined, "read spec")
}

func TestVerifySpecProfileIncludesScopeMetadata(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t)

	result, err := svc.Verify(VerifyInput{Profile: "spec", Disposition: "report"})
	require.NoError(t, err)

	require.Equal(t, "v0.1.0", result.MilestoneFocus)
	require.Equal(t, "v0.1.0", result.ActiveSpecVersion)
	require.Equal(t, "specs/tessariq-v0.1.0.md", result.ActiveSpecPath)

	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(data), `"milestone_focus":"v0.1.0"`)
	require.Contains(t, string(data), `"active_spec_version":"v0.1.0"`)
	require.Contains(t, string(data), `"active_spec_path":"specs/tessariq-v0.1.0.md"`)
}

func TestBuildSpecFindingsAcceptsHistoricalAdapterAlias(t *testing.T) {
	t.Parallel()

	// Collect all normative refs except agent-and-runtime-contract.
	var refsWithoutAgent []string
	for _, req := range requiredSpecCoverageByVersion["v0.1.0"] {
		if req.Ref != "specs/tessariq-v0.1.0.md#agent-and-runtime-contract" {
			refsWithoutAgent = append(refsWithoutAgent, req.Ref)
		}
	}

	tasks := []*Task{
		taskForTest("TASK-001-modern", "todo", "p1", refsWithoutAgent),
		// Completed task uses the historical alias.
		taskForTest("TASK-002-legacy", "done", "p1", []string{"specs/tessariq-v0.1.0.md#adapter-contract"}),
	}

	findings := buildSpecFindings(tasks, specScope{
		Milestone: "v0.1.0",
		Version:   "v0.1.0",
		Path:      "specs/tessariq-v0.1.0.md",
	}, nil)

	require.Empty(t, findings, "adapter-contract alias should satisfy agent-and-runtime-contract coverage")
}

func TestBuildSpecFindingsReportsAllUncoveredNormativeSections(t *testing.T) {
	t.Parallel()

	tasks := []*Task{
		taskForTest("TASK-001-a", "todo", "p1", []string{"specs/tessariq-v0.1.0.md#release-intent"}),
	}

	findings := buildSpecFindings(tasks, specScope{
		Milestone: "v0.1.0",
		Version:   "v0.1.0",
		Path:      "specs/tessariq-v0.1.0.md",
	}, nil)

	expectedCount := len(requiredSpecCoverageByVersion["v0.1.0"]) - 1
	require.Len(t, findings, expectedCount)
	for _, f := range findings {
		require.Equal(t, "high", f.Severity)
		require.Equal(t, "missing spec coverage", f.Title)
	}

	details := make(map[string]bool)
	for _, f := range findings {
		details[f.Details] = true
	}
	require.True(t, details["No tracked task covers host prerequisites."])
	require.True(t, details["No tracked task covers compatibility rules."])
	require.True(t, details["No tracked task covers agent and runtime contract."])
	require.True(t, details["No tracked task covers acceptance scenarios."])
	require.True(t, details["No tracked task covers failure UX."])
}

func taskForTest(id, status, priority string, refs []string) *Task {
	if refs == nil {
		refs = []string{"specs/tessariq-v0.1.0.md#tessariq-run-task-path"}
	}
	return &Task{
		Frontmatter: TaskFrontmatter{
			ID:          id,
			Title:       id,
			Status:      status,
			Priority:    priority,
			Milestone:   "v0.1.0",
			SpecVersion: "v0.1.0",
			SpecRefs:    refs,
			Verification: TaskVerification{
				Unit:        VerificationTier{Rationale: "required"},
				Integration: VerificationTier{Rationale: "considered"},
				E2E:         VerificationTier{Rationale: "considered"},
				Mutation:    VerificationTier{Rationale: "considered"},
				ManualTest:  VerificationTier{Rationale: "considered"},
			},
		},
		Body: "## Summary\n\nx\n\n## Acceptance Criteria\n\nx\n\n## Test Expectations\n\nx\n\n## TDD Plan\n\nx\n\n## Notes\n\nx\n",
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}
