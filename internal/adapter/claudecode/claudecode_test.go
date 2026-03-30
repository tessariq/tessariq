package claudecode

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
)

func TestBuildArgs_DefaultNonInteractive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	args := buildArgs(cfg, "implement feature X")

	require.Contains(t, args, "--print")
	require.Contains(t, args, "--dangerously-skip-permissions")
	require.Contains(t, args, "implement feature X")
	require.NotContains(t, args, "--model")
}

func TestBuildArgs_WithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Model = "sonnet"
	args := buildArgs(cfg, "fix bug")

	require.Contains(t, args, "--model")
	require.Contains(t, args, "sonnet")
	require.Contains(t, args, "--print")
	require.Contains(t, args, "--dangerously-skip-permissions")
}

func TestBuildArgs_Interactive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	args := buildArgs(cfg, "review code")

	require.NotContains(t, args, "--print")
	require.NotContains(t, args, "--dangerously-skip-permissions")
	require.NotContains(t, args, "review code",
		"interactive mode should not pass task content as prompt arg")
}

func TestBuildArgs_InteractiveWithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	cfg.Model = "opus"
	args := buildArgs(cfg, "task content")

	require.NotContains(t, args, "--print")
	require.NotContains(t, args, "--dangerously-skip-permissions")
	require.Contains(t, args, "--model")
	require.Contains(t, args, "opus")
}

func TestBuildArgs_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		model       string
		interactive bool
		task        string
		wantPrint   bool
		wantSkip    bool
		wantModel   bool
		wantTask    bool
	}{
		{
			name:      "default autonomous no model",
			task:      "do stuff",
			wantPrint: true, wantSkip: true, wantModel: false, wantTask: true,
		},
		{
			name:      "autonomous with model",
			model:     "sonnet",
			task:      "do stuff",
			wantPrint: true, wantSkip: true, wantModel: true, wantTask: true,
		},
		{
			name:        "interactive no model",
			interactive: true,
			task:        "do stuff",
			wantPrint:   false, wantSkip: false, wantModel: false, wantTask: false,
		},
		{
			name:        "interactive with model",
			interactive: true,
			model:       "opus",
			task:        "do stuff",
			wantPrint:   false, wantSkip: false, wantModel: true, wantTask: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := run.DefaultConfig()
			cfg.Model = tt.model
			cfg.Interactive = tt.interactive
			args := buildArgs(cfg, tt.task)

			if tt.wantPrint {
				require.Contains(t, args, "--print")
			} else {
				require.NotContains(t, args, "--print")
			}

			if tt.wantSkip {
				require.Contains(t, args, "--dangerously-skip-permissions")
			} else {
				require.NotContains(t, args, "--dangerously-skip-permissions")
			}

			if tt.wantModel {
				require.Contains(t, args, "--model")
				require.Contains(t, args, tt.model)
			} else {
				require.NotContains(t, args, "--model")
			}

			if tt.wantTask {
				require.Contains(t, args, tt.task)
			} else {
				require.NotContains(t, args, tt.task)
			}
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

	require.True(t, app["interactive"])
	require.True(t, app["model"])
}

func TestBuildApplied_WithoutModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	app := buildApplied(cfg)

	require.True(t, app["interactive"])
	_, hasModel := app["model"]
	require.False(t, hasModel, "model should be absent when not requested")
}

func TestResolveImage_CustomOverride(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Image = "myregistry/claude:v2"
	img := resolveImage(cfg)

	require.Equal(t, "myregistry/claude:v2", img)
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
	require.True(t, p.Applied()["model"])
	require.True(t, p.Applied()["interactive"])
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
	require.True(t, p.Applied()["interactive"])
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
