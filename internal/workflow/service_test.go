package workflow

import (
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
			SpecRefs:    []string{"specs/tessariq-v0.1.0.md#cli-run"},
			Verification: TaskVerification{
				Unit:        VerificationTier{Rationale: "required"},
				Integration: VerificationTier{Rationale: "considered"},
				E2E:         VerificationTier{Rationale: "considered"},
				Mutation:    VerificationTier{Rationale: "considered"},
			},
		},
		Body: "## Summary\n",
	}

	violations := validateStateAndTasks(state, []*Task{task}, time.Now().UTC())
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
			StaleAfterMinutes:   180,
			MaxRetries:          0,
		},
	}
	tasks := []*Task{
		taskForTest("TASK-001-a", "todo", "p1", nil),
		taskForTest("TASK-002-b", "in_progress", "p2", nil),
	}
	tasks[1].Frontmatter.DependsOn = []string{"TASK-999-missing"}

	violations := validateStateAndTasks(state, tasks, now)
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
		taskForTest("TASK-001-a", "todo", "p1", []string{"specs/tessariq-v0.1.0.md#cli-run"}),
	}

	findings := buildSpecFindings(tasks)
	require.NotEmpty(t, findings)
	require.Equal(t, "high", findings[0].Severity)
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

	findings := buildTaskFindings([]*Task{task}, VerifyInput{Profile: "task", TaskID: "TASK-001-a"})
	require.Len(t, findings, 3)

	missing := buildTaskFindings([]*Task{}, VerifyInput{Profile: "task", TaskID: "TASK-404"})
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
		Profile:     "spec",
		Disposition: "report",
		GeneratedAt: "2026-03-29T00:00:00Z",
		PlanPath:    "/tmp/plan.md",
		ReportPath:  "/tmp/report.json",
	}
	plan := renderVerificationPlan(report)
	require.Contains(t, plan, "Validate seeded tasks cover all normative and acceptance references.")

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
		SpecRefs:    []string{"specs/tessariq-v0.1.0.md#cli-run"},
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
	require.ElementsMatch(t, []string{"unit", "integration", "e2e", "mutation"}, missing)
}

func TestPriorityAndSeverityRank(t *testing.T) {
	t.Parallel()

	require.Less(t, priorityRank("p0"), priorityRank("p2"))
	require.Less(t, severityRank("low"), severityRank("high"))
}

func taskForTest(id, status, priority string, refs []string) *Task {
	if refs == nil {
		refs = []string{"specs/tessariq-v0.1.0.md#cli-run"}
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
			},
		},
		Body: "## Summary\n\nx\n\n## Acceptance Criteria\n\nx\n\n## Test Expectations\n\nx\n\n## TDD Plan\n\nx\n\n## Notes\n\nx\n",
	}
}
