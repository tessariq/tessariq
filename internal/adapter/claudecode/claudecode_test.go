package claudecode

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/version"
)

func TestDefaultImage_UsesVersionTag(t *testing.T) {
	t.Parallel()
	img := DefaultImage()
	require.Contains(t, img, "ghcr.io/tessariq/claude-code:")
	require.Contains(t, img, version.Version)
}

func TestBuildArgs_DefaultNonInteractive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	args := buildArgs(cfg, "implement feature X")

	require.Contains(t, args, "--print")
	require.Contains(t, args, "--dangerously-skip-permissions")
	require.Contains(t, args, "--verbose")
	require.Contains(t, args, "--output-format")
	require.Contains(t, args, "stream-json")
	require.Contains(t, args, "--")
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
	require.Contains(t, args, "--verbose")
	require.Contains(t, args, "--output-format")
	require.Contains(t, args, "stream-json")
}

func TestBuildArgs_Interactive(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Interactive = true
	args := buildArgs(cfg, "review code")

	require.NotContains(t, args, "--print")
	require.NotContains(t, args, "--dangerously-skip-permissions")
	require.NotContains(t, args, "--verbose")
	require.NotContains(t, args, "--output-format")
	require.Contains(t, args, "--")
	require.Contains(t, args, "review code",
		"interactive mode should pass task content as initial prompt")
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
		name           string
		model          string
		interactive    bool
		task           string
		wantPrint      bool
		wantSkip       bool
		wantVerbose    bool
		wantStreamJSON bool
		wantModel      bool
		wantTask       bool
	}{
		{
			name:      "default autonomous no model",
			task:      "do stuff",
			wantPrint: true, wantSkip: true, wantVerbose: true, wantStreamJSON: true, wantModel: false, wantTask: true,
		},
		{
			name:      "autonomous with model",
			model:     "sonnet",
			task:      "do stuff",
			wantPrint: true, wantSkip: true, wantVerbose: true, wantStreamJSON: true, wantModel: true, wantTask: true,
		},
		{
			name:        "interactive no model",
			interactive: true,
			task:        "do stuff",
			wantPrint:   false, wantSkip: false, wantVerbose: false, wantStreamJSON: false, wantModel: false, wantTask: true,
		},
		{
			name:        "interactive with model",
			interactive: true,
			model:       "opus",
			task:        "do stuff",
			wantPrint:   false, wantSkip: false, wantVerbose: false, wantStreamJSON: false, wantModel: true, wantTask: true,
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

			if tt.wantVerbose {
				require.Contains(t, args, "--verbose")
			} else {
				require.NotContains(t, args, "--verbose")
			}

			if tt.wantStreamJSON {
				require.Contains(t, args, "--output-format")
				require.Contains(t, args, "stream-json")
			} else {
				require.NotContains(t, args, "--output-format")
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

func TestBuildArgs_YAMLFrontmatterNotParsedAsFlag(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	task := "---\nid: TASK-001\ntitle: Fix the bug\n---\n\nDo the thing."
	args := buildArgs(cfg, task)

	require.Contains(t, args, "--")
	require.Contains(t, args, task)
	// Verify end-of-options marker appears before the task content.
	dashIdx := -1
	taskIdx := -1
	for i, a := range args {
		if a == "--" {
			dashIdx = i
		}
		if a == task {
			taskIdx = i
		}
	}
	require.Greater(t, dashIdx, -1, "end-of-options marker must be present")
	require.Greater(t, taskIdx, dashIdx, "task content must follow end-of-options marker")
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

func TestBuildSupported_WithModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	cfg.Model = "sonnet"
	app := buildSupported(cfg)

	require.True(t, app["interactive"])
	require.True(t, app["model"])
}

func TestBuildSupported_WithoutModel(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	app := buildSupported(cfg)

	require.True(t, app["interactive"],
		"supported is a capability flag: adapter supports interactive even when false was requested")
	_, hasModel := app["model"]
	require.False(t, hasModel, "model should be absent when not requested")
}

func TestBuildSupported_InteractiveNotRequested(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	app := buildSupported(cfg)

	require.False(t, cfg.Interactive, "default config does not request interactive mode")
	require.True(t, app["interactive"],
		"supported is a capability flag: true even when interactive is not requested")
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
	require.True(t, a.Supported()["model"])
	require.True(t, a.Supported()["interactive"],
		"supported is a capability flag: adapter supports interactive even when false was requested")
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
	require.True(t, a.Supported()["interactive"])
}

func TestNew_NoModelOmitsFromMetadata(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	_, hasModel := a.Requested()["model"]
	require.False(t, hasModel)
	_, hasModelSupported := a.Supported()["model"]
	require.False(t, hasModelSupported)
}

func TestNew_Args(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "do the thing", nil)

	require.Contains(t, a.Args(), "--print")
	require.Contains(t, a.Args(), "do the thing")
}

func TestBinaryName_IsClaudeString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "claude", BinaryName)
}

func TestAgentConfig_Name(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	require.Equal(t, "claude-code", a.Name())
}

func TestAgentConfig_BinaryName(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	require.Equal(t, "claude", a.BinaryName())
}

func TestNew_WithEnvVars(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	envVars := map[string]string{"CLAUDE_CONFIG_DIR": "/home/tessariq/.claude"}
	a := New(cfg, "task", envVars)

	require.Equal(t, DefaultImage(), a.Image())
	require.Equal(t, "/home/tessariq/.claude", a.EnvVars()["CLAUDE_CONFIG_DIR"])
}

func TestNew_NilEnvVars(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	require.Nil(t, a.EnvVars())
	require.Equal(t, DefaultImage(), a.Image())
}

func TestAgentConfig_UpdateCommand(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	require.Equal(t, []string{"npm", "install", "--global", "--prefix", "/cache", "@anthropic-ai/claude-code@latest"}, a.UpdateCommand("/cache"))
}

func TestAgentConfig_UpdateCommand_PrefixSubstitution(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	cmd := a.UpdateCommand("/custom/prefix")
	require.Equal(t, "/custom/prefix", cmd[4])
}

func TestAgentConfig_VersionCommand(t *testing.T) {
	t.Parallel()

	cfg := run.DefaultConfig()
	a := New(cfg, "task", nil)

	require.Equal(t, []string{"claude", "--version"}, a.VersionCommand())
}
