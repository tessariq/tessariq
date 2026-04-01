package workflow

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	paths          Paths
	changedFilesFn func(repoRoot string) ([]string, error)
}

func NewService(start string) (*Service, error) {
	paths, err := DiscoverPaths(start)
	if err != nil {
		return nil, err
	}
	return &Service{paths: paths, changedFilesFn: changedFilesFromGit}, nil
}

func (s *Service) ValidateState() (ValidationResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return ValidationResult{}, err
	}

	violations := validateStateAndTasks(state, tasks, nowUTC(), s.paths.RepoRoot)
	return ValidationResult{
		Valid:      len(violations) == 0,
		Violations: violations,
	}, nil
}

func (s *Service) Next() (SelectionResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return SelectionResult{}, err
	}

	now := nowUTC()
	result, updatedState, updatedTasks, err := selectNextTask(state, tasks, now)
	if err != nil {
		return SelectionResult{}, err
	}

	if err := s.saveAll(updatedState, updatedTasks); err != nil {
		return SelectionResult{}, err
	}

	return result, nil
}

func (s *Service) Start(input StartInput) (StartResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return StartResult{}, err
	}

	task, err := findTask(tasks, input.TaskID)
	if err != nil {
		return StartResult{}, err
	}
	if task.Frontmatter.Status != "todo" {
		return StartResult{}, fmt.Errorf("task %s is not todo", input.TaskID)
	}
	if state.Frontmatter.ActiveTask != "" {
		return StartResult{}, fmt.Errorf("active task %s already in progress", state.Frontmatter.ActiveTask)
	}
	if unresolvedDependency(task, tasks) {
		return StartResult{}, fmt.Errorf("task %s has unresolved dependencies", input.TaskID)
	}

	now := nowUTC().Format(timeLayout)
	task.Frontmatter.Status = "in_progress"
	task.Frontmatter.UpdatedAt = now

	state.Frontmatter.UpdatedAt = now
	state.Frontmatter.Mode = input.Mode
	state.Frontmatter.AgentID = input.AgentID
	state.Frontmatter.Model = input.Model
	state.Frontmatter.RunID = newWorkflowRunID()
	state.Frontmatter.ActiveTask = input.TaskID
	state.Frontmatter.ActiveTaskStartedAt = now
	state.Frontmatter.Attempt = 0
	state.Frontmatter.LastTransition = "start"
	state.Frontmatter.LastTransitionAt = now
	state.Frontmatter.RepoState = "active"
	state.Frontmatter.SelectionReason = "explicit start"

	if err := s.saveAll(state, tasks); err != nil {
		return StartResult{}, err
	}

	return StartResult{
		TaskID:    input.TaskID,
		Status:    task.Frontmatter.Status,
		StartedAt: now,
		RunID:     state.Frontmatter.RunID,
	}, nil
}

func (s *Service) Finish(input FinishInput) (FinishResult, error) {
	if _, ok := validTerminalStatuses[input.Status]; !ok {
		return FinishResult{}, fmt.Errorf("invalid finish status %q", input.Status)
	}
	if strings.TrimSpace(input.Note) == "" {
		return FinishResult{}, errors.New("finish note must not be empty")
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return FinishResult{}, err
	}

	taskID := input.TaskID
	if taskID == "" {
		taskID = state.Frontmatter.ActiveTask
	}
	if taskID == "" {
		return FinishResult{}, errors.New("no active task to finish")
	}

	task, err := findTask(tasks, taskID)
	if err != nil {
		return FinishResult{}, err
	}
	if task.Frontmatter.Status != "in_progress" {
		return FinishResult{}, fmt.Errorf("task %s is not in progress", taskID)
	}

	if input.Status == "done" {
		if err := checkManualTestArtifacts(s.paths.ArtifactsDir, taskID); err != nil {
			return FinishResult{}, err
		}
	}

	now := nowUTC().Format(timeLayout)
	task.Frontmatter.Status = input.Status
	task.Frontmatter.UpdatedAt = now
	appendTaskNote(task, fmt.Sprintf("- %s: %s", now, strings.TrimSpace(input.Note)))

	state.Frontmatter.UpdatedAt = now
	state.Frontmatter.LastCompleted = taskID
	state.Frontmatter.ActiveTask = ""
	state.Frontmatter.ActiveTaskStartedAt = ""
	state.Frontmatter.Attempt = 0
	state.Frontmatter.LastTransition = "finish"
	state.Frontmatter.LastTransitionAt = now
	state.Frontmatter.RepoState = "idle"

	result, updatedState, updatedTasks, err := selectNextTask(state, tasks, nowUTC())
	if err != nil {
		return FinishResult{}, err
	}
	state = updatedState
	tasks = updatedTasks
	state.Frontmatter.SelectionReason = result.Reason

	if err := s.saveAll(state, tasks); err != nil {
		return FinishResult{}, err
	}

	return FinishResult{
		TaskID:     taskID,
		Status:     input.Status,
		FinishedAt: now,
		Note:       strings.TrimSpace(input.Note),
	}, nil
}

func checkManualTestArtifacts(artifactsDir, taskID string) error {
	taskDir := filepath.Join(artifactsDir, "manual-test", taskID)
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return fmt.Errorf("manual test artifacts missing for task %s; run manual testing before finishing as done", taskID)
	}

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsDir() {
			continue
		}
		tsDir := filepath.Join(taskDir, entry.Name())
		planExists := fileExists(filepath.Join(tsDir, "plan.md"))
		reportExists := fileExists(filepath.Join(tsDir, "report.md"))
		if planExists && reportExists {
			return nil
		}
		if planExists && !reportExists {
			return fmt.Errorf("manual test artifacts incomplete for task %s: plan.md exists but report.md is missing", taskID)
		}
	}

	return fmt.Errorf("manual test artifacts missing for task %s; run manual testing before finishing as done", taskID)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *Service) RefreshState() (RefreshResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return RefreshResult{}, err
	}

	now := nowUTC().Format(timeLayout)
	candidates := candidateTasks(tasks)
	state.Frontmatter.UpdatedAt = now
	state.Frontmatter.NextTasks = candidates
	state.Frontmatter.RepoState = repoState(state.Frontmatter.ActiveTask)
	state.Body = renderStateSnapshot(state.Frontmatter, tasks)

	if err := s.saveState(state); err != nil {
		return RefreshResult{}, err
	}

	return RefreshResult{
		UpdatedAt:  now,
		NextTasks:  candidates,
		ActiveTask: state.Frontmatter.ActiveTask,
		RepoState:  state.Frontmatter.RepoState,
	}, nil
}

func (s *Service) Verify(input VerifyInput) (VerifyResult, error) {
	if _, ok := validProfiles[input.Profile]; !ok {
		return VerifyResult{}, fmt.Errorf("invalid profile %q", input.Profile)
	}
	if _, ok := validDispositions[input.Disposition]; !ok {
		return VerifyResult{}, fmt.Errorf("invalid disposition %q", input.Disposition)
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return VerifyResult{}, err
	}

	report, err := s.buildVerificationReport(state, tasks, input)
	if err != nil {
		return VerifyResult{}, err
	}

	if err := s.writeVerificationArtifacts(report); err != nil {
		return VerifyResult{}, err
	}

	now := nowUTC().Format(timeLayout)
	state.Frontmatter.UpdatedAt = now
	state.Frontmatter.ValidationLastRun = report.GeneratedAt
	state.Frontmatter.ValidationStatus = validationStatus(report.Summary)
	state.Frontmatter.ValidationScope = verificationScopeLabel(report)
	state.Frontmatter.ValidationCheckedAt = report.GeneratedAt
	state.Body = renderStateSnapshot(state.Frontmatter, tasks)

	if err := s.saveState(state); err != nil {
		return VerifyResult{}, err
	}

	return VerifyResult{
		Profile:           report.Profile,
		Disposition:       report.Disposition,
		MilestoneFocus:    report.MilestoneFocus,
		ActiveSpecVersion: report.ActiveSpecVersion,
		ActiveSpecPath:    report.ActiveSpecPath,
		ArtifactDir:       relPath(s.paths.RepoRoot, report.ArtifactDir),
		PlanPath:          relPath(s.paths.RepoRoot, report.PlanPath),
		ReportPath:        relPath(s.paths.RepoRoot, report.ReportPath),
		Summary:           report.Summary,
		Findings:          report.Findings,
	}, nil
}

func (s *Service) CreateFollowups(input FollowupsInput) (FollowupsResult, error) {
	if input.Mode != "create" {
		return FollowupsResult{}, fmt.Errorf("unsupported follow-up mode %q", input.Mode)
	}
	if _, ok := validSeverities[input.MinSeverity]; !ok {
		return FollowupsResult{}, fmt.Errorf("invalid minimum severity %q", input.MinSeverity)
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return FollowupsResult{}, err
	}
	if strings.TrimSpace(state.Frontmatter.ValidationLastRun) == "" {
		return FollowupsResult{}, errors.New("no validation run recorded in state")
	}

	report, reportPath, err := s.loadVerificationReport(state.Frontmatter.ValidationLastRun)
	if err != nil {
		return FollowupsResult{}, err
	}

	created := make([]string, 0)
	nextNumber := nextTaskNumber(tasks)
	for _, finding := range report.Findings {
		if finding.Status != "open" || severityRank(finding.Severity) < severityRank(input.MinSeverity) {
			continue
		}

		task := newFollowupTask(nextNumber, finding, state.Frontmatter.MilestoneFocus)
		task.Filename = filepath.Join(s.paths.TasksDir, task.Frontmatter.ID+".md")
		tasks = append(tasks, task)
		created = append(created, task.Frontmatter.ID)
		nextNumber++
	}

	if len(created) == 0 {
		return FollowupsResult{
			CreatedTaskIDs: nil,
			ReportPath:     relPath(s.paths.RepoRoot, reportPath),
		}, nil
	}

	if err := s.saveAll(state, tasks); err != nil {
		return FollowupsResult{}, err
	}

	return FollowupsResult{
		CreatedTaskIDs: created,
		ReportPath:     relPath(s.paths.RepoRoot, reportPath),
	}, nil
}

func (s *Service) CheckSkills() (SkillCheckResult, error) {
	left, err := skillTree(s.paths.AgentSkills)
	if err != nil {
		return SkillCheckResult{}, err
	}
	right, err := skillTree(s.paths.ClaudeSkills)
	if err != nil {
		return SkillCheckResult{}, err
	}

	mismatches := compareSkillTrees(left, right)
	return SkillCheckResult{
		Match:      len(mismatches) == 0,
		Mismatches: mismatches,
	}, nil
}

func (s *Service) loadStateAndTasks() (*State, []*Task, error) {
	state, err := s.loadState()
	if err != nil {
		return nil, nil, err
	}
	tasks, err := s.loadTasks()
	if err != nil {
		return nil, nil, err
	}
	return state, tasks, nil
}

func (s *Service) loadState() (*State, error) {
	data, err := os.ReadFile(s.paths.StateFile)
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}
	frontmatter, body, err := parseFrontmatter[StateFrontmatter](data)
	if err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	return &State{Frontmatter: frontmatter, Body: body}, nil
}

func (s *Service) loadTasks() ([]*Task, error) {
	entries, err := os.ReadDir(s.paths.TasksDir)
	if err != nil {
		return nil, fmt.Errorf("read tasks dir: %w", err)
	}

	tasks := make([]*Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filename := filepath.Join(s.paths.TasksDir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read task %s: %w", entry.Name(), err)
		}
		frontmatter, body, err := parseFrontmatter[TaskFrontmatter](data)
		if err != nil {
			return nil, fmt.Errorf("parse task %s: %w", entry.Name(), err)
		}
		tasks = append(tasks, &Task{
			Frontmatter: frontmatter,
			Body:        body,
			Filename:    filename,
		})
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Frontmatter.ID < tasks[j].Frontmatter.ID
	})
	return tasks, nil
}

func (s *Service) saveAll(state *State, tasks []*Task) error {
	state.Body = renderStateSnapshot(state.Frontmatter, tasks)
	if err := s.saveState(state); err != nil {
		return err
	}
	return s.saveTasks(tasks)
}

func (s *Service) saveState(state *State) error {
	encoded, err := marshalFrontmatter(state.Frontmatter, state.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(s.paths.StateFile, encoded, 0o644)
}

func (s *Service) saveTasks(tasks []*Task) error {
	for _, task := range tasks {
		encoded, err := marshalFrontmatter(task.Frontmatter, task.Body)
		if err != nil {
			return err
		}
		if err := os.WriteFile(task.Filename, encoded, 0o644); err != nil {
			return fmt.Errorf("write task %s: %w", task.Frontmatter.ID, err)
		}
	}
	return nil
}

func (s *Service) buildVerificationReport(state *State, tasks []*Task, input VerifyInput) (VerificationReport, error) {
	timestamp := nowUTC().Format("20060102T150405Z")
	artifactDir := filepath.Join(s.paths.ArtifactsDir, "verify", input.Profile, verifyTarget(input, state), timestamp)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return VerificationReport{}, fmt.Errorf("create verification artifacts dir: %w", err)
	}

	scope, scopeViolations := resolveSpecScope(state)
	specDocs, specDocViolations := loadReferencedSpecDocs(s.paths.RepoRoot, tasks)
	scopeViolations = append(scopeViolations, specDocViolations...)
	if len(scopeViolations) == 0 {
		if _, err := loadSpecDocument(s.paths.RepoRoot, scope); err != nil {
			scopeViolations = append(scopeViolations, err.Error())
		}
	}

	findings := make([]Finding, 0)
	changedFiles := make([]string, 0)
	if input.Profile == "task" && s.changedFilesFn != nil {
		files, err := s.changedFilesFn(s.paths.RepoRoot)
		if err == nil {
			changedFiles = files
		}
	}

	findings = append(findings, buildTaskFindings(state, tasks, input, scope, specDocs, scopeViolations, changedFiles)...)
	if input.Profile == "spec" {
		findings = append(findings, buildSpecFindings(tasks, scope, scopeViolations)...)
	}
	if input.Profile == "implemented" {
		findings = append(findings, buildImplementedFindings(tasks)...)
	}

	reportPath := filepath.Join(artifactDir, "report.json")
	planPath := filepath.Join(artifactDir, "plan.md")
	report := VerificationReport{
		SchemaVersion:     reportSchemaVersion,
		Profile:           input.Profile,
		Disposition:       input.Disposition,
		GeneratedAt:       nowUTC().Format(timeLayout),
		MilestoneFocus:    scope.Milestone,
		ActiveSpecVersion: scope.Version,
		ActiveSpecPath:    scope.Path,
		ArtifactDir:       artifactDir,
		PlanPath:          planPath,
		ReportPath:        reportPath,
		Findings:          findings,
		Summary:           summarizeFindings(findings),
	}

	return report, nil
}

func (s *Service) writeVerificationArtifacts(report VerificationReport) error {
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal verification report: %w", err)
	}
	if err := os.WriteFile(report.ReportPath, reportJSON, 0o644); err != nil {
		return fmt.Errorf("write verification report: %w", err)
	}

	plan := renderVerificationPlan(report)
	if err := os.WriteFile(report.PlanPath, []byte(plan), 0o644); err != nil {
		return fmt.Errorf("write verification plan: %w", err)
	}
	return nil
}

func (s *Service) loadVerificationReport(generatedAt string) (VerificationReport, string, error) {
	verifyDir := filepath.Join(s.paths.ArtifactsDir, "verify")
	if _, err := os.Stat(verifyDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return VerificationReport{}, "", fmt.Errorf("no local verification artifacts found for validation run %s; rerun verify before followups", generatedAt)
		}
		return VerificationReport{}, "", fmt.Errorf("stat verification artifacts dir: %w", err)
	}

	var matchedPath string
	var matchedReport VerificationReport
	err := filepath.WalkDir(verifyDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "report.json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read verification report %s: %w", path, err)
		}

		var report VerificationReport
		if err := json.Unmarshal(data, &report); err != nil {
			return fmt.Errorf("parse verification report %s: %w", path, err)
		}
		if report.GeneratedAt != generatedAt {
			return nil
		}

		matchedPath = path
		matchedReport = report
		return fs.SkipAll
	})
	if err != nil {
		return VerificationReport{}, "", fmt.Errorf("load verification report: %w", err)
	}
	if matchedPath == "" {
		return VerificationReport{}, "", fmt.Errorf("no local verification artifacts found for validation run %s; rerun verify before followups", generatedAt)
	}

	return matchedReport, matchedPath, nil
}

func validateStateAndTasks(state *State, tasks []*Task, now time.Time, repoRoot string) []string {
	var violations []string
	if state.Frontmatter.SchemaVersion != stateSchemaVersion {
		violations = append(violations, fmt.Sprintf("state schema_version must be %d", stateSchemaVersion))
	}
	if state.Frontmatter.StaleAfterMinutes <= 0 {
		violations = append(violations, "state stale_after_minutes must be positive")
	}
	if state.Frontmatter.MaxRetries <= 0 {
		violations = append(violations, "state max_retries must be positive")
	}

	_, scopeViolations := resolveSpecScope(state)
	violations = append(violations, scopeViolations...)
	specDocs, specDocViolations := loadReferencedSpecDocs(repoRoot, tasks)
	violations = append(violations, specDocViolations...)

	byID := make(map[string]*Task, len(tasks))
	inProgress := 0
	for _, task := range tasks {
		if task.Frontmatter.ID == "" {
			violations = append(violations, "task id must not be empty")
			continue
		}
		if _, ok := validStatuses[task.Frontmatter.Status]; !ok {
			violations = append(violations, fmt.Sprintf("task %s has invalid status %q", task.Frontmatter.ID, task.Frontmatter.Status))
		}
		if _, ok := validPriorities[task.Frontmatter.Priority]; !ok {
			violations = append(violations, fmt.Sprintf("task %s has invalid priority %q", task.Frontmatter.ID, task.Frontmatter.Priority))
		}
		if task.Frontmatter.SpecVersion == "" {
			violations = append(violations, fmt.Sprintf("task %s missing spec_version", task.Frontmatter.ID))
		}
		if len(task.Frontmatter.SpecRefs) == 0 {
			violations = append(violations, fmt.Sprintf("task %s missing spec_refs", task.Frontmatter.ID))
		}
		for _, ref := range task.Frontmatter.SpecRefs {
			path, anchor, err := splitSpecRef(ref)
			if err != nil {
				violations = append(violations, fmt.Sprintf("task %s has invalid spec_ref %q: %v", task.Frontmatter.ID, ref, err))
				continue
			}
			refVersion := specVersionFromPath(path)
			if refVersion != "" && task.Frontmatter.SpecVersion != refVersion {
				violations = append(violations, fmt.Sprintf("task %s spec_version %q does not match spec_ref %q", task.Frontmatter.ID, task.Frontmatter.SpecVersion, ref))
			}
			if specDoc := specDocs[path]; specDoc != nil {
				if _, ok := specDoc.Anchors[anchor]; !ok {
					violations = append(violations, fmt.Sprintf("task %s spec_ref %q points to unknown heading anchor", task.Frontmatter.ID, ref))
				}
			}
		}
		if missing := missingTaskSections(task.Body); len(missing) > 0 {
			violations = append(violations, fmt.Sprintf("task %s missing sections: %s", task.Frontmatter.ID, strings.Join(missing, ", ")))
		}
		if bad := missingVerificationRationales(task.Frontmatter.Verification); len(bad) > 0 {
			violations = append(violations, fmt.Sprintf("task %s missing verification rationale for: %s", task.Frontmatter.ID, strings.Join(bad, ", ")))
		}
		byID[task.Frontmatter.ID] = task
		if task.Frontmatter.Status == "in_progress" {
			inProgress++
		}
	}

	for _, task := range tasks {
		for _, dep := range task.Frontmatter.DependsOn {
			dependency, ok := byID[dep]
			if !ok {
				violations = append(violations, fmt.Sprintf("task %s depends on unknown task %s", task.Frontmatter.ID, dep))
				continue
			}
			if dependency.Frontmatter.Status != "done" && task.Frontmatter.Status == "in_progress" {
				violations = append(violations, fmt.Sprintf("task %s is in progress with unresolved dependency %s", task.Frontmatter.ID, dep))
			}
		}
	}

	if inProgress > 1 {
		violations = append(violations, "more than one task is in progress")
	}
	if state.Frontmatter.ActiveTask != "" {
		active, ok := byID[state.Frontmatter.ActiveTask]
		if !ok {
			violations = append(violations, fmt.Sprintf("active task %s not found", state.Frontmatter.ActiveTask))
		} else if active.Frontmatter.Status != "in_progress" {
			violations = append(violations, fmt.Sprintf("active task %s is not marked in_progress", state.Frontmatter.ActiveTask))
		}
		if state.Frontmatter.ActiveTaskStartedAt == "" {
			violations = append(violations, "active task is missing active_task_started_at")
		} else if startedAt, err := time.Parse(timeLayout, state.Frontmatter.ActiveTaskStartedAt); err == nil {
			if now.Sub(startedAt) > time.Duration(state.Frontmatter.StaleAfterMinutes)*time.Minute {
				violations = append(violations, fmt.Sprintf("active task %s is stale", state.Frontmatter.ActiveTask))
			}
		}
	} else if inProgress == 1 {
		violations = append(violations, "one task is in progress but state.active_task is empty")
	}

	return violations
}

const timeLayout = time.RFC3339

func selectNextTask(state *State, tasks []*Task, now time.Time) (SelectionResult, *State, []*Task, error) {
	recovered := false
	if state.Frontmatter.ActiveTask != "" && state.Frontmatter.ActiveTaskStartedAt != "" {
		startedAt, err := time.Parse(timeLayout, state.Frontmatter.ActiveTaskStartedAt)
		if err == nil && now.Sub(startedAt) > time.Duration(state.Frontmatter.StaleAfterMinutes)*time.Minute {
			activeTask, err := findTask(tasks, state.Frontmatter.ActiveTask)
			if err != nil {
				return SelectionResult{}, nil, nil, err
			}
			recovered = true
			if state.Frontmatter.Attempt+1 >= state.Frontmatter.MaxRetries {
				activeTask.Frontmatter.Status = "blocked"
				appendTaskNote(activeTask, fmt.Sprintf("- %s: automatically blocked after repeated stale recovery", now.Format(timeLayout)))
			} else {
				activeTask.Frontmatter.Status = "todo"
				appendTaskNote(activeTask, fmt.Sprintf("- %s: automatically requeued after stale recovery", now.Format(timeLayout)))
			}
			activeTask.Frontmatter.UpdatedAt = now.Format(timeLayout)
			state.Frontmatter.Attempt++
			state.Frontmatter.ActiveTask = ""
			state.Frontmatter.ActiveTaskStartedAt = ""
			state.Frontmatter.LastTransition = "recover"
			state.Frontmatter.LastTransitionAt = now.Format(timeLayout)
		}
	}

	if state.Frontmatter.ActiveTask != "" {
		candidates := candidateTasks(tasks)
		state.Frontmatter.NextTasks = candidates
		state.Frontmatter.SelectionReason = "continue active task"
		state.Frontmatter.UpdatedAt = now.Format(timeLayout)
		return SelectionResult{
			SelectedTask: state.Frontmatter.ActiveTask,
			Candidates:   candidates,
			Reason:       "continue active task",
			Recovered:    recovered,
		}, state, tasks, nil
	}

	eligible := eligibleTasks(tasks)
	candidates := make([]string, 0, len(eligible))
	for _, task := range eligible {
		candidates = append(candidates, task.Frontmatter.ID)
	}

	state.Frontmatter.NextTasks = candidates
	state.Frontmatter.SelectionReason = "next eligible todo by priority"
	state.Frontmatter.UpdatedAt = now.Format(timeLayout)
	state.Frontmatter.RepoState = "idle"

	selected := ""
	if len(candidates) > 0 {
		selected = candidates[0]
	}

	return SelectionResult{
		SelectedTask: selected,
		Candidates:   candidates,
		Reason:       state.Frontmatter.SelectionReason,
		Recovered:    recovered,
	}, state, tasks, nil
}

func eligibleTasks(tasks []*Task) []*Task {
	result := make([]*Task, 0)
	for _, task := range tasks {
		if task.Frontmatter.Status != "todo" || unresolvedDependency(task, tasks) {
			continue
		}
		result = append(result, task)
	}

	sort.Slice(result, func(i, j int) bool {
		left := result[i]
		right := result[j]
		if priorityRank(left.Frontmatter.Priority) == priorityRank(right.Frontmatter.Priority) {
			return left.Frontmatter.ID < right.Frontmatter.ID
		}
		return priorityRank(left.Frontmatter.Priority) < priorityRank(right.Frontmatter.Priority)
	})
	return result
}

func candidateTasks(tasks []*Task) []string {
	eligible := eligibleTasks(tasks)
	out := make([]string, 0, min(5, len(eligible)))
	for i, task := range eligible {
		if i == 5 {
			break
		}
		out = append(out, task.Frontmatter.ID)
	}
	return out
}

func unresolvedDependency(task *Task, tasks []*Task) bool {
	for _, dep := range task.Frontmatter.DependsOn {
		dependency, err := findTask(tasks, dep)
		if err != nil {
			return true
		}
		if dependency.Frontmatter.Status != "done" {
			return true
		}
	}
	return false
}

func findTask(tasks []*Task, id string) (*Task, error) {
	for _, task := range tasks {
		if task.Frontmatter.ID == id {
			return task, nil
		}
	}
	return nil, fmt.Errorf("task %s not found", id)
}

func renderStateSnapshot(frontmatter StateFrontmatter, tasks []*Task) string {
	var out strings.Builder
	out.WriteString("## Machine Snapshot\n\n")
	out.WriteString(fmt.Sprintf("- Active task: %s\n", nonEmpty(frontmatter.ActiveTask, "none")))
	out.WriteString(fmt.Sprintf("- Last completed: %s\n", nonEmpty(frontmatter.LastCompleted, "none")))
	out.WriteString(fmt.Sprintf("- Validation status: %s\n", nonEmpty(frontmatter.ValidationStatus, "not_run")))
	if frontmatter.ActiveSpecVersion != "" || frontmatter.ActiveSpecPath != "" {
		out.WriteString(fmt.Sprintf("- Active spec: %s (%s)\n", nonEmpty(frontmatter.ActiveSpecVersion, "unknown"), nonEmpty(frontmatter.ActiveSpecPath, "unknown")))
	}
	out.WriteString("- Next tasks:\n")
	if len(frontmatter.NextTasks) == 0 {
		out.WriteString("  - none\n")
	} else {
		for _, taskID := range frontmatter.NextTasks {
			out.WriteString(fmt.Sprintf("  - %s\n", taskID))
		}
	}
	out.WriteString("\n## Task Counts\n\n")
	counts := map[string]int{}
	for _, task := range tasks {
		counts[task.Frontmatter.Status]++
	}
	for _, status := range []string{"todo", "in_progress", "done", "blocked", "cancelled"} {
		out.WriteString(fmt.Sprintf("- %s: %d\n", status, counts[status]))
	}
	return out.String()
}

func missingTaskSections(body string) []string {
	required := []string{"## Summary", "## Acceptance Criteria", "## Test Expectations", "## TDD Plan", "## Notes"}
	var missing []string
	for _, heading := range required {
		if !strings.Contains(body, heading) {
			missing = append(missing, heading)
		}
	}
	return missing
}

func missingVerificationRationales(v TaskVerification) []string {
	type namedTier struct {
		name string
		tier VerificationTier
	}
	tiers := []namedTier{
		{name: "unit", tier: v.Unit},
		{name: "integration", tier: v.Integration},
		{name: "e2e", tier: v.E2E},
		{name: "mutation", tier: v.Mutation},
		{name: "manual_test", tier: v.ManualTest},
	}
	var missing []string
	for _, tier := range tiers {
		if strings.TrimSpace(tier.tier.Rationale) == "" {
			missing = append(missing, tier.name)
		}
	}
	return missing
}

func appendTaskNote(task *Task, note string) {
	if strings.TrimSpace(note) == "" {
		return
	}
	section := "## Notes\n"
	if !strings.Contains(task.Body, section) {
		task.Body += "\n\n" + section + "\n"
	}
	task.Body = strings.TrimRight(task.Body, "\n") + "\n" + note + "\n"
}

func loadReferencedSpecDocs(repoRoot string, tasks []*Task) (map[string]*specDocument, []string) {
	docs := make(map[string]*specDocument)
	var violations []string

	for _, task := range tasks {
		for _, ref := range task.Frontmatter.SpecRefs {
			path, _, err := splitSpecRef(ref)
			if err != nil {
				continue
			}
			if _, ok := docs[path]; ok {
				continue
			}
			doc, err := loadSpecDocumentAtPath(repoRoot, path)
			if err != nil {
				violations = append(violations, err.Error())
				continue
			}
			docs[path] = doc
		}
	}

	return docs, violations
}

func buildTaskFindings(state *State, tasks []*Task, input VerifyInput, scope specScope, specDocs map[string]*specDocument, scopeViolations []string, changedFiles []string) []Finding {
	findings := make([]Finding, 0)
	targets := tasks
	if input.Profile == "task" {
		targets = nil
		for _, task := range tasks {
			if task.Frontmatter.ID == input.TaskID {
				targets = append(targets, task)
				break
			}
		}
	}

	for _, violation := range scopeViolations {
		findings = append(findings, Finding{
			ID:          slugify("scope-" + violation),
			Title:       "invalid planning spec scope",
			Severity:    "high",
			Status:      "open",
			Details:     violation,
			SpecVersion: state.Frontmatter.ActiveSpecVersion,
		})
	}

	for _, task := range targets {
		if len(task.Frontmatter.SpecRefs) == 0 {
			findings = append(findings, Finding{
				ID:       task.Frontmatter.ID + "-spec-refs",
				Title:    "task missing spec refs",
				Severity: "high",
				Status:   "open",
				Details:  "Task must include exact spec_refs.",
				TaskID:   task.Frontmatter.ID,
			})
		}
		for _, ref := range task.Frontmatter.SpecRefs {
			path, anchor, err := splitSpecRef(ref)
			if err != nil {
				findings = append(findings, Finding{
					ID:       task.Frontmatter.ID + "-bad-spec-ref",
					Title:    "task has invalid spec ref",
					Severity: "high",
					Status:   "open",
					Details:  err.Error(),
					TaskID:   task.Frontmatter.ID,
					SpecRefs: []string{ref},
				})
				continue
			}
			refVersion := specVersionFromPath(path)
			if refVersion != "" && task.Frontmatter.SpecVersion != refVersion {
				findings = append(findings, Finding{
					ID:          task.Frontmatter.ID + "-spec-version",
					Title:       "task spec version does not match its spec refs",
					Severity:    "high",
					Status:      "open",
					Details:     fmt.Sprintf("Task spec_version %q does not match spec ref %q.", task.Frontmatter.SpecVersion, ref),
					TaskID:      task.Frontmatter.ID,
					SpecVersion: task.Frontmatter.SpecVersion,
					SpecRefs:    []string{ref},
				})
			}
			if specDoc := specDocs[path]; specDoc != nil {
				if _, ok := specDoc.Anchors[anchor]; !ok {
					findings = append(findings, Finding{
						ID:          task.Frontmatter.ID + "-" + slugify(anchor),
						Title:       "task spec ref points to an unknown heading",
						Severity:    "high",
						Status:      "open",
						Details:     fmt.Sprintf("Spec ref %q does not resolve to a heading in %s.", ref, path),
						TaskID:      task.Frontmatter.ID,
						SpecVersion: task.Frontmatter.SpecVersion,
						SpecRefs:    []string{ref},
					})
				}
			}
		}
		if missing := missingTaskSections(task.Body); len(missing) > 0 {
			findings = append(findings, Finding{
				ID:       task.Frontmatter.ID + "-sections",
				Title:    "task missing required sections",
				Severity: "high",
				Status:   "open",
				Details:  strings.Join(missing, ", "),
				TaskID:   task.Frontmatter.ID,
			})
		}
		if strings.TrimSpace(task.Frontmatter.Verification.Unit.Rationale) == "" {
			findings = append(findings, Finding{
				ID:       task.Frontmatter.ID + "-verification",
				Title:    "task missing explicit test-tier consideration",
				Severity: "medium",
				Status:   "open",
				Details:  "Each task must explicitly consider unit, integration, e2e, and mutation testing.",
				TaskID:   task.Frontmatter.ID,
			})
		}
	}
	if input.Profile == "task" && len(targets) == 0 {
		findings = append(findings, Finding{
			ID:       input.TaskID + "-missing",
			Title:    "task not found",
			Severity: "high",
			Status:   "open",
			Details:  "Requested task-scoped verification target was not found.",
			TaskID:   input.TaskID,
		})
	}

	if input.Profile == "task" {
		targetTaskID := strings.TrimSpace(input.TaskID)
		if targetTaskID == "" {
			targetTaskID = strings.TrimSpace(state.Frontmatter.ActiveTask)
		}
		if targetTaskID == "" {
			targetTaskID = "sweep"
		}
		findings = append(findings, buildChangelogFindings(targetTaskID, changedFiles)...)
	}

	return findings
}

func buildChangelogFindings(taskID string, changedFiles []string) []Finding {
	required, evidence := requiresChangelogUpdate(changedFiles)
	if !required {
		return nil
	}

	return []Finding{{
		ID:       taskID + "-changelog",
		Title:    "user-visible changes missing changelog update",
		Severity: "medium",
		Status:   "open",
		Details:  fmt.Sprintf("User-visible code changes detected (%s) without updating CHANGELOG.md. Add a user-facing entry under CHANGELOG.md before finishing the task.", strings.Join(evidence, ", ")),
		TaskID:   taskID,
	}}
}

func requiresChangelogUpdate(changedFiles []string) (bool, []string) {
	if len(changedFiles) == 0 {
		return false, nil
	}

	touchedChangelog := false
	evidence := make([]string, 0)
	seen := make(map[string]struct{})

	for _, path := range changedFiles {
		norm := filepath.ToSlash(strings.TrimSpace(path))
		if norm == "" {
			continue
		}
		if norm == "CHANGELOG.md" {
			touchedChangelog = true
			continue
		}
		if !isUserVisibleCodePath(norm) {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		evidence = append(evidence, norm)
	}

	if touchedChangelog || len(evidence) == 0 {
		return false, nil
	}

	sort.Strings(evidence)
	return true, evidence
}

func isUserVisibleCodePath(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	if strings.HasSuffix(path, "_test.go") {
		return false
	}
	if isWorkflowToolPath(path) {
		return false
	}
	if strings.HasPrefix(path, "cmd/tessariq/") {
		return true
	}
	if strings.HasPrefix(path, "internal/") && !strings.HasPrefix(path, "internal/testutil/") {
		return true
	}
	return false
}

func isWorkflowToolPath(path string) bool {
	return strings.HasPrefix(path, "cmd/tessariq-workflow/") || strings.HasPrefix(path, "internal/workflow/")
}

func changedFilesFromGit(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list changed files: %w", err)
	}
	return parseGitStatusPorcelain(string(out)), nil
}

func parseGitStatusPorcelain(output string) []string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	paths := make([]string, 0)
	seen := make(map[string]struct{})

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}

		path := strings.TrimSpace(line[3:])
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = strings.TrimSpace(path[idx+4:])
		}
		if strings.HasPrefix(path, "\"") && strings.HasSuffix(path, "\"") {
			if unquoted, err := strconv.Unquote(path); err == nil {
				path = unquoted
			}
		}

		norm := filepath.ToSlash(strings.TrimSpace(path))
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		paths = append(paths, norm)
	}

	sort.Strings(paths)
	return paths
}

func buildSpecFindings(tasks []*Task, scope specScope, scopeViolations []string) []Finding {
	if len(scopeViolations) > 0 {
		return nil
	}

	covered := map[string]bool{}
	for _, task := range tasks {
		for _, ref := range task.Frontmatter.SpecRefs {
			covered[ref] = true
			if resolved := resolveSpecRefAlias(ref, scope.Version); resolved != ref {
				covered[resolved] = true
			}
		}
	}

	findings := make([]Finding, 0)
	for _, required := range requiredSpecCoverageByVersion[scope.Version] {
		if covered[required.Ref] {
			continue
		}
		findings = append(findings, Finding{
			ID:          slugify(required.Title),
			Title:       "missing spec coverage",
			Severity:    "high",
			Status:      "open",
			Details:     fmt.Sprintf("No tracked task covers %s.", required.Title),
			SpecVersion: "v0.1.0",
			SpecRefs:    []string{required.Ref},
		})
	}
	return findings
}

func buildImplementedFindings(tasks []*Task) []Finding {
	findings := make([]Finding, 0)
	for _, task := range tasks {
		if task.Frontmatter.Status == "done" && strings.TrimSpace(task.Frontmatter.Verification.Mutation.Rationale) == "" {
			findings = append(findings, Finding{
				ID:       task.Frontmatter.ID + "-mutation-rationale",
				Title:    "implemented task missing mutation-testing consideration",
				Severity: "medium",
				Status:   "open",
				Details:  "Completed tasks should retain mutation-test rationale for auditability.",
				TaskID:   task.Frontmatter.ID,
			})
		}
	}
	return findings
}

func summarizeFindings(findings []Finding) VerificationSummary {
	var summary VerificationSummary
	for _, finding := range findings {
		switch finding.Severity {
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		case "low":
			summary.Low++
		}
	}
	return summary
}

func validationStatus(summary VerificationSummary) string {
	if summary.High > 0 {
		return "failed"
	}
	if summary.Medium > 0 || summary.Low > 0 {
		return "warnings"
	}
	return "passed"
}

func verificationScopeLabel(report VerificationReport) string {
	if report.ActiveSpecVersion == "" {
		return report.Profile
	}
	return fmt.Sprintf("%s:%s", report.Profile, report.ActiveSpecVersion)
}

func renderVerificationPlan(report VerificationReport) string {
	var out strings.Builder
	out.WriteString("# Verification Plan\n\n")
	out.WriteString(fmt.Sprintf("- Profile: `%s`\n", report.Profile))
	out.WriteString(fmt.Sprintf("- Disposition: `%s`\n", report.Disposition))
	out.WriteString(fmt.Sprintf("- Generated at: `%s`\n", report.GeneratedAt))
	if report.MilestoneFocus != "" {
		out.WriteString(fmt.Sprintf("- Milestone: `%s`\n", report.MilestoneFocus))
	}
	if report.ActiveSpecVersion != "" {
		out.WriteString(fmt.Sprintf("- Active spec version: `%s`\n", report.ActiveSpecVersion))
	}
	if report.ActiveSpecPath != "" {
		out.WriteString(fmt.Sprintf("- Active spec path: `%s`\n", report.ActiveSpecPath))
	}
	out.WriteString(fmt.Sprintf("- Report: `%s`\n", relPath(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(report.PlanPath)))), report.ReportPath)))
	out.WriteString("\n## Checks\n\n")
	switch report.Profile {
	case "task":
		out.WriteString("- Validate task metadata, scope alignment, and required sections.\n")
		out.WriteString("- Remind for changelog updates when user-visible code changes are present (excluding workflow-tooling-only changes).\n")
	case "implemented":
		out.WriteString("- Validate completed-task verification metadata.\n")
	case "spec":
		out.WriteString("- Validate seeded tasks cover the active milestone spec, not every versioned spec.\n")
		out.WriteString("- Validate task spec_refs resolve to live headings in the active spec.\n")
	}
	out.WriteString("- Confirm TDD and test-tier expectations remain explicit.\n")
	out.WriteString("- Confirm mutation threshold policy remains documented.\n")
	return out.String()
}

func verifyTarget(input VerifyInput, state *State) string {
	if input.TaskID != "" {
		return input.TaskID
	}
	if input.Profile == "task" && state.Frontmatter.ActiveTask != "" {
		return state.Frontmatter.ActiveTask
	}
	return "sweep"
}

func priorityRank(priority string) int {
	switch priority {
	case "p0":
		return 0
	case "p1":
		return 1
	case "p2":
		return 2
	default:
		return 3
	}
}

func severityRank(severity string) int {
	switch severity {
	case "low":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

func newWorkflowRunID() string {
	return "wf-" + nowUTC().Format("20060102T150405Z")
}

func nextTaskNumber(tasks []*Task) int {
	maxNumber := 0
	for _, task := range tasks {
		if n := parseTaskNumber(task.Frontmatter.ID); n > maxNumber {
			maxNumber = n
		}
	}
	return maxNumber + 1
}

func parseTaskNumber(id string) int {
	var number int
	_, _ = fmt.Sscanf(id, "TASK-%03d-", &number)
	return number
}

func newFollowupTask(number int, finding Finding, milestone string) *Task {
	id := fmt.Sprintf("TASK-%03d-%s", number, slugify(finding.Title))
	timestamp := nowUTC().Format(timeLayout)
	priority := "p1"
	if finding.Severity == "high" {
		priority = "p0"
	}
	if finding.Severity == "low" {
		priority = "p2"
	}

	task := &Task{
		Frontmatter: TaskFrontmatter{
			ID:          id,
			Title:       finding.Title,
			Status:      "todo",
			Priority:    priority,
			DependsOn:   nil,
			Milestone:   milestone,
			SpecVersion: nonEmpty(finding.SpecVersion, "v0.1.0"),
			SpecRefs:    finding.SpecRefs,
			UpdatedAt:   timestamp,
			Areas:       []string{"workflow", "verification"},
			Verification: TaskVerification{
				Unit: VerificationTier{
					Required:  true,
					Commands:  []string{"go test ./..."},
					Rationale: "Follow-up items start by adding the smallest failing unit test possible.",
				},
				Integration: VerificationTier{
					Required:  false,
					Commands:  []string{"go test -tags=integration ./..."},
					Rationale: "Add only if the follow-up crosses a real process boundary and use Testcontainers only.",
				},
				E2E: VerificationTier{
					Required:  false,
					Commands:  []string{"go test -tags=e2e ./..."},
					Rationale: "Add only if the fix changes a critical CLI workflow end to end.",
				},
				Mutation: VerificationTier{
					Required:  false,
					Commands:  []string{"gremlins unleash --exclude-files 'cmd/.*|internal/testutil/.*' --threshold-efficacy 70"},
					Rationale: "Use when the follow-up changes non-trivial logic.",
				},
			},
		},
		Body: fmt.Sprintf("## Summary\n\nAddress verification finding `%s`.\n\n## Acceptance Criteria\n\n- Finding is resolved or explicitly downgraded with evidence.\n\n## Test Expectations\n\n- Re-evaluate unit, integration, e2e, and mutation test needs before implementation.\n\n## TDD Plan\n\n- Start with the smallest failing test that reproduces the finding.\n\n## Notes\n\n- Source report finding: `%s`\n", finding.ID, finding.Details),
	}
	return task
}

func skillTree(root string) (map[string]string, error) {
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	return result, err
}

func compareSkillTrees(left, right map[string]string) []string {
	seen := map[string]struct{}{}
	var mismatches []string
	for path, leftContent := range left {
		seen[path] = struct{}{}
		rightContent, ok := right[path]
		if !ok {
			mismatches = append(mismatches, fmt.Sprintf("missing in .claude/skills: %s", path))
			continue
		}
		if rightContent != leftContent {
			mismatches = append(mismatches, fmt.Sprintf("content mismatch: %s", path))
		}
	}
	for path := range right {
		if _, ok := seen[path]; ok {
			continue
		}
		mismatches = append(mismatches, fmt.Sprintf("missing in .agents/skills: %s", path))
	}
	sort.Strings(mismatches)
	return mismatches
}

func relPath(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}

func repoState(activeTask string) string {
	if activeTask == "" {
		return "idle"
	}
	return "active"
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "/", "-", ":", "", ",", "", ".", "", "`", "", "'", "", "\"", "")
	value = replacer.Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "item"
	}
	return value
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
