package workflow

import "time"

const (
	stateSchemaVersion  = 1
	reportSchemaVersion = 1
)

var (
	validStatuses         = map[string]struct{}{"todo": {}, "in_progress": {}, "done": {}, "blocked": {}, "cancelled": {}}
	validTerminalStatuses = map[string]struct{}{"done": {}, "blocked": {}, "cancelled": {}}
	validPriorities       = map[string]struct{}{"p0": {}, "p1": {}, "p2": {}, "p3": {}}
	validProfiles         = map[string]struct{}{"task": {}, "implemented": {}, "spec": {}}
	validDispositions     = map[string]struct{}{"report": {}, "hybrid": {}}
	validSeverities       = map[string]struct{}{"low": {}, "medium": {}, "high": {}}
)

type StateFrontmatter struct {
	SchemaVersion       int      `yaml:"schema_version"`
	UpdatedAt           string   `yaml:"updated_at"`
	Mode                string   `yaml:"mode"`
	RunID               string   `yaml:"run_id"`
	AgentID             string   `yaml:"agent_id"`
	Model               string   `yaml:"model"`
	ActiveTask          string   `yaml:"active_task"`
	ActiveTaskStartedAt string   `yaml:"active_task_started_at"`
	Attempt             int      `yaml:"attempt"`
	LastCompleted       string   `yaml:"last_completed"`
	NextTasks           []string `yaml:"next_tasks"`
	RepoState           string   `yaml:"repo_state"`
	LastTransition      string   `yaml:"last_transition"`
	LastTransitionAt    string   `yaml:"last_transition_at"`
	SelectionReason     string   `yaml:"selection_reason"`
	ValidationLastRun   string   `yaml:"validation_last_run"`
	ValidationStatus    string   `yaml:"validation_status"`
	ValidationScope     string   `yaml:"validation_scope"`
	ValidationPlan      string   `yaml:"validation_plan"`
	ValidationReport    string   `yaml:"validation_report"`
	ValidationCheckedAt string   `yaml:"validation_checked_at"`
	MilestoneFocus      string   `yaml:"milestone_focus"`
	ActiveSpecVersion   string   `yaml:"active_spec_version"`
	ActiveSpecPath      string   `yaml:"active_spec_path"`
	StaleAfterMinutes   int      `yaml:"stale_after_minutes"`
	MaxRetries          int      `yaml:"max_retries"`
}

type State struct {
	Frontmatter StateFrontmatter
	Body        string
}

type VerificationTier struct {
	Required  bool     `yaml:"required"`
	Commands  []string `yaml:"commands"`
	Rationale string   `yaml:"rationale"`
}

type TaskVerification struct {
	Unit        VerificationTier `yaml:"unit"`
	Integration VerificationTier `yaml:"integration"`
	E2E         VerificationTier `yaml:"e2e"`
	Mutation    VerificationTier `yaml:"mutation"`
	ManualTest  VerificationTier `yaml:"manual_test"`
}

type TaskFrontmatter struct {
	ID           string           `yaml:"id"`
	Title        string           `yaml:"title"`
	Status       string           `yaml:"status"`
	Priority     string           `yaml:"priority"`
	DependsOn    []string         `yaml:"depends_on"`
	Milestone    string           `yaml:"milestone"`
	SpecVersion  string           `yaml:"spec_version"`
	SpecRefs     []string         `yaml:"spec_refs"`
	UpdatedAt    string           `yaml:"updated_at"`
	Areas        []string         `yaml:"areas"`
	Verification TaskVerification `yaml:"verification"`
	BlockedBy    []string         `yaml:"blocked_by,omitempty"`
	Supersedes   []string         `yaml:"supersedes,omitempty"`
}

type Task struct {
	Frontmatter TaskFrontmatter
	Body        string
	Filename    string
}

type Finding struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Severity    string   `json:"severity"`
	Status      string   `json:"status"`
	Details     string   `json:"details"`
	SpecVersion string   `json:"spec_version,omitempty"`
	SpecRefs    []string `json:"spec_refs,omitempty"`
	TaskID      string   `json:"task_id,omitempty"`
}

type VerificationSummary struct {
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
}

type VerificationReport struct {
	SchemaVersion     int                 `json:"schema_version"`
	Profile           string              `json:"profile"`
	Disposition       string              `json:"disposition"`
	GeneratedAt       string              `json:"generated_at"`
	MilestoneFocus    string              `json:"milestone_focus,omitempty"`
	ActiveSpecVersion string              `json:"active_spec_version,omitempty"`
	ActiveSpecPath    string              `json:"active_spec_path,omitempty"`
	ArtifactDir       string              `json:"artifact_dir"`
	PlanPath          string              `json:"plan_path"`
	ReportPath        string              `json:"report_path"`
	Findings          []Finding           `json:"findings"`
	Summary           VerificationSummary `json:"summary"`
}

type ValidationResult struct {
	Valid      bool     `json:"valid"`
	Violations []string `json:"violations"`
}

type SelectionResult struct {
	SelectedTask string   `json:"selected_task"`
	Candidates   []string `json:"candidates"`
	Reason       string   `json:"reason"`
	Recovered    bool     `json:"recovered"`
}

type StartInput struct {
	TaskID  string
	Mode    string
	AgentID string
	Model   string
}

type StartResult struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	RunID     string `json:"run_id"`
}

type FinishInput struct {
	TaskID string
	Status string
	Note   string
}

type FinishResult struct {
	TaskID     string `json:"task_id"`
	Status     string `json:"status"`
	FinishedAt string `json:"finished_at"`
	Note       string `json:"note"`
}

type RefreshResult struct {
	UpdatedAt  string   `json:"updated_at"`
	NextTasks  []string `json:"next_tasks"`
	ActiveTask string   `json:"active_task"`
	RepoState  string   `json:"repo_state"`
}

type VerifyInput struct {
	Profile     string
	TaskID      string
	Disposition string
}

type VerifyResult struct {
	Profile     string              `json:"profile"`
	Disposition string              `json:"disposition"`
	ArtifactDir string              `json:"artifact_dir"`
	PlanPath    string              `json:"plan_path"`
	ReportPath  string              `json:"report_path"`
	Summary     VerificationSummary `json:"summary"`
	Findings    []Finding           `json:"findings"`
}

type FollowupsInput struct {
	Mode        string
	MinSeverity string
}

type FollowupsResult struct {
	CreatedTaskIDs []string `json:"created_task_ids"`
	ReportPath     string   `json:"report_path"`
}

type SkillCheckResult struct {
	Match      bool     `json:"match"`
	Mismatches []string `json:"mismatches"`
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
