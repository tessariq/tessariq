package opencode

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
)

func TestBuildArgs_DefaultNonInteractive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	args := buildArgs(cfg, "implement feature X")

	require.Equal(t, []string{"implement feature X"}, args,
		"opencode takes only the task as a positional arg, no flags")
}

func TestBuildArgs_WithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Model = "sonnet"
	args := buildArgs(cfg, "fix bug")

	require.Equal(t, []string{"fix bug"}, args,
		"model is not forwarded as a CLI flag")
	require.NotContains(t, args, "--model")
	require.NotContains(t, args, "sonnet")
}

func TestBuildArgs_Interactive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	args := buildArgs(cfg, "review code")

	require.Equal(t, []string{"review code"}, args,
		"interactive mode does not change CLI args")
}

func TestBuildArgs_InteractiveWithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	cfg.Model = "opus"
	args := buildArgs(cfg, "task content")

	require.Equal(t, []string{"task content"}, args,
		"neither model nor interactive affect CLI args")
}

func TestBuildArgs_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		model       string
		interactive bool
		task        string
	}{
		{name: "default autonomous no model", task: "do stuff"},
		{name: "autonomous with model", model: "sonnet", task: "do stuff"},
		{name: "interactive no model", interactive: true, task: "do stuff"},
		{name: "interactive with model", interactive: true, model: "opus", task: "do stuff"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := run.DefaultConfig()
			cfg.Model = tt.model
			cfg.Interactive = tt.interactive
			args := buildArgs(cfg, tt.task)

			require.Equal(t, []string{tt.task}, args,
				"opencode always produces a single positional arg regardless of config")
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

	require.False(t, app["interactive"], "opencode does not support interactive toggle")
	require.False(t, app["model"], "opencode does not support model selection")
}

func TestBuildApplied_WithoutModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	app := buildApplied(cfg)

	require.False(t, app["interactive"], "opencode does not support interactive toggle")
	_, hasModel := app["model"]
	require.False(t, hasModel, "model should be absent when not requested")
}

func TestBuildApplied_Interactive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	app := buildApplied(cfg)

	require.False(t, app["interactive"],
		"opencode cannot apply interactive mode, so applied must be false")
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

	require.Equal(t, DefaultImage, img)
	require.NotEmpty(t, img)
}

func TestNew_ReturnsProcessWithMetadata(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Model = "sonnet"
	p := New(cfg, "implement X")

	require.Equal(t, DefaultImage, p.Image())
	require.Equal(t, "sonnet", p.Requested()["model"])
	require.Equal(t, false, p.Requested()["interactive"])
	require.False(t, p.Applied()["model"], "opencode does not apply model")
	require.False(t, p.Applied()["interactive"], "opencode does not apply interactive")
}

func TestNew_CustomImage(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Image = "custom/img:v3"
	p := New(cfg, "task")

	require.Equal(t, "custom/img:v3", p.Image())
}

func TestNew_InteractiveMode(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	p := New(cfg, "task")

	require.Equal(t, true, p.Requested()["interactive"])
	require.False(t, p.Applied()["interactive"],
		"opencode does not support interactive toggle")
}

func TestNew_NoModelOmitsFromMetadata(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	p := New(cfg, "task")

	_, hasModel := p.Requested()["model"]
	require.False(t, hasModel)
	_, hasModelApplied := p.Applied()["model"]
	require.False(t, hasModelApplied)
}

func TestStart_BinaryNotFound_UserGuidance(t *testing.T) {
	// Not parallel: t.Setenv modifies process environment.
	t.Setenv("PATH", t.TempDir())

	cfg := run.DefaultConfig()
	p := New(cfg, "task")

	err := p.Start(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, exec.ErrNotFound)
	require.Contains(t, err.Error(), `adapter binary "opencode"`)
	require.Contains(t, err.Error(), "container image")
	require.Contains(t, err.Error(), "--image")
}
