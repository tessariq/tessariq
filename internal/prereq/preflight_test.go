package prereq

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequirementsForCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		want    []Dependency
	}{
		{
			name:    "init requires git",
			command: "init",
			want:    []Dependency{DependencyGit},
		},
		{
			name:    "run requires git and tmux",
			command: "run",
			want:    []Dependency{DependencyGit, DependencyTmux},
		},
		{
			name:    "attach requires tmux",
			command: "attach",
			want:    []Dependency{DependencyTmux},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := RequirementsForCommand(tt.command)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRequirementsForCommand_UnknownCommand(t *testing.T) {
	t.Parallel()

	_, err := RequirementsForCommand("unknown")
	require.ErrorIs(t, err, ErrUnknownCommand)
}

func TestChecker_CheckCommand_AllDependenciesAvailable(t *testing.T) {
	t.Parallel()

	checker := Checker{
		LookPath: func(name string) (string, error) {
			return "/usr/bin/" + name, nil
		},
	}

	err := checker.CheckCommand("run")
	require.NoError(t, err)
}

func TestChecker_CheckCommand_MissingGit(t *testing.T) {
	t.Parallel()

	checker := Checker{
		LookPath: func(name string) (string, error) {
			if name == string(DependencyGit) {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + name, nil
		},
	}

	err := checker.CheckCommand("run")
	require.Error(t, err)
	require.Contains(t, err.Error(), "required host prerequisite \"git\" is missing or unavailable")
	require.Contains(t, err.Error(), "install or enable git, then retry")
}

func TestChecker_CheckCommand_MissingTmux(t *testing.T) {
	t.Parallel()

	checker := Checker{
		LookPath: func(name string) (string, error) {
			if name == string(DependencyTmux) {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + name, nil
		},
	}

	err := checker.CheckCommand("attach")
	require.Error(t, err)
	require.Contains(t, err.Error(), "required host prerequisite \"tmux\" is missing or unavailable")
	require.Contains(t, err.Error(), "install or enable tmux, then retry")
}
