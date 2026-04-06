package opencode

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/version"
)

func TestDefaultImage_UsesVersionTag(t *testing.T) {
	t.Parallel()
	img := DefaultImage()
	require.Contains(t, img, "ghcr.io/tessariq/opencode:")
	require.Contains(t, img, version.Version)
}

func TestBuildArgs_DefaultNonInteractive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	args := buildArgs(cfg, "implement feature X")

	require.Equal(t, []string{"run", "--format", "json", "--", "implement feature X"}, args,
		"non-interactive mode uses run subcommand with JSON output")
}

func TestBuildArgs_WithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Model = "sonnet"
	args := buildArgs(cfg, "fix bug")

	require.Equal(t, []string{"--model", "sonnet", "run", "--format", "json", "--", "fix bug"}, args,
		"model is a root-level flag and must precede the run subcommand")
	require.Contains(t, args, "--model")
	require.Contains(t, args, "sonnet")
}

func TestBuildArgs_Interactive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	args := buildArgs(cfg, "review code")

	require.Equal(t, []string{"--", "review code"}, args,
		"interactive mode launches TUI without run subcommand")
}

func TestBuildArgs_InteractiveWithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	cfg.Model = "opus"
	args := buildArgs(cfg, "task content")

	require.Equal(t, []string{"--model", "opus", "--", "task content"}, args,
		"model is forwarded in interactive mode too")
}

func TestBuildArgs_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		model       string
		interactive bool
		task        string
		want        []string
	}{
		{
			name: "default autonomous no model",
			task: "do stuff",
			want: []string{"run", "--format", "json", "--", "do stuff"},
		},
		{
			name:  "autonomous with model",
			model: "sonnet",
			task:  "do stuff",
			want:  []string{"--model", "sonnet", "run", "--format", "json", "--", "do stuff"},
		},
		{
			name:        "interactive no model",
			interactive: true,
			task:        "do stuff",
			want:        []string{"--", "do stuff"},
		},
		{
			name:        "interactive with model",
			interactive: true,
			model:       "opus",
			task:        "do stuff",
			want:        []string{"--model", "opus", "--", "do stuff"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := run.DefaultConfig()
			cfg.Model = tt.model
			cfg.Interactive = tt.interactive
			args := buildArgs(cfg, tt.task)

			require.Equal(t, tt.want, args)
		})
	}
}

func TestBuildRequested_WithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Model = "sonnet"
	req := buildRequested(cfg)

	require.Equal(t, false, req["interactive"])
	require.Equal(t, "sonnet", req["model"])
}

func TestBuildRequested_WithoutModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	req := buildRequested(cfg)

	require.Equal(t, false, req["interactive"])
	_, hasModel := req["model"]
	require.False(t, hasModel, "model should be absent when empty")
}

func TestBuildRequested_Interactive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	req := buildRequested(cfg)

	require.Equal(t, true, req["interactive"])
}

func TestBuildApplied_WithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Model = "sonnet"
	app := buildApplied(cfg)

	require.True(t, app["interactive"], "applied is a capability flag: adapter supports interactive")
	require.True(t, app["model"], "opencode forwards model as-is")
}

func TestBuildApplied_WithoutModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	app := buildApplied(cfg)

	require.True(t, app["interactive"], "applied is a capability flag: adapter supports interactive")
	_, hasModel := app["model"]
	require.False(t, hasModel, "model should be absent when not requested")
}

func TestBuildApplied_InteractiveRequested(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	app := buildApplied(cfg)

	require.True(t, app["interactive"],
		"applied is a capability flag: adapter supports interactive")
}

func TestBuildApplied_InteractiveNotRequested(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	app := buildApplied(cfg)

	require.True(t, app["interactive"],
		"applied is a capability flag: true even when interactive is not requested")
}

func TestResolveImage_CustomOverride(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Image = "myregistry/opencode:v2"
	img := resolveImage(cfg)

	require.Equal(t, "myregistry/opencode:v2", img)
}

func TestResolveImage_Default(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	img := resolveImage(cfg)

	require.Equal(t, DefaultImage(), img)
	require.NotEmpty(t, img)
}

func TestNew_ReturnsConfigWithMetadata(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Model = "sonnet"
	a := New(cfg, "implement X", nil)

	require.Equal(t, DefaultImage(), a.Image())
	require.Equal(t, "sonnet", a.Requested()["model"])
	require.Equal(t, false, a.Requested()["interactive"])
	require.True(t, a.Applied()["model"], "opencode forwards model as-is")
	require.True(t, a.Applied()["interactive"], "applied is a capability flag: adapter supports interactive")
}

func TestNew_CustomImage(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Image = "custom/img:v3"
	a := New(cfg, "task", nil)

	require.Equal(t, "custom/img:v3", a.Image())
}

func TestNew_InteractiveMode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	a := New(cfg, "task", nil)

	require.Equal(t, true, a.Requested()["interactive"])
	require.True(t, a.Applied()["interactive"],
		"opencode applies interactive when requested")
}

func TestNew_NoModelOmitsFromMetadata(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	_, hasModel := a.Requested()["model"]
	require.False(t, hasModel)
	_, hasModelApplied := a.Applied()["model"]
	require.False(t, hasModelApplied)
}

func TestNew_Args(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "do the thing", nil)

	require.Equal(t, []string{"run", "--format", "json", "--", "do the thing"}, a.Args())
}

func TestBuildArgs_YAMLFrontmatterNotParsedAsFlag(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	task := "---\nid: TASK-001\ntitle: Fix the bug\n---\n\nDo the thing."
	args := buildArgs(cfg, task)

	require.Equal(t, []string{"run", "--format", "json", "--", task}, args,
		"YAML frontmatter starting with --- must not be parsed as a CLI flag")
}

func TestNew_WithEnvVars(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	envVars := map[string]string{"SOME_VAR": "value"}
	a := New(cfg, "task", envVars)

	require.Equal(t, "value", a.EnvVars()["SOME_VAR"])
}

func TestAgentConfig_Name(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	require.Equal(t, "opencode", a.Name())
}

func TestAgentConfig_BinaryName(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	require.Equal(t, "opencode", a.BinaryName())
}

func TestNew_NilEnvVars(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	require.Nil(t, a.EnvVars())
}
