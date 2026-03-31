package prereq

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

type Dependency string

const (
	DependencyGit    Dependency = "git"
	DependencyTmux   Dependency = "tmux"
	DependencyDocker Dependency = "docker"
)

var ErrUnknownCommand = errors.New("unknown command prerequisites")

type Checker struct {
	LookPath func(file string) (string, error)
}

func NewChecker() Checker {
	return Checker{LookPath: exec.LookPath}
}

func RequirementsForCommand(command string) ([]Dependency, error) {
	switch command {
	case "init":
		return []Dependency{DependencyGit}, nil
	case "run":
		return []Dependency{DependencyGit, DependencyTmux, DependencyDocker}, nil
	case "attach":
		return []Dependency{DependencyTmux}, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownCommand, command)
	}
}

func (c Checker) CheckCommand(command string) error {
	requirements, err := RequirementsForCommand(command)
	if err != nil {
		return err
	}

	lookPath := c.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	for _, dep := range requirements {
		if _, err := lookPath(string(dep)); err != nil {
			return fmt.Errorf("required host prerequisite %q is missing or unavailable; install or enable %s, then retry", dep, dep)
		}
	}

	return nil
}

// CheckDockerDaemon verifies the Docker daemon is running and reachable
// by executing docker info.
func (c Checker) CheckDockerDaemon(ctx context.Context) error {
	lookPath := c.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	dockerPath, err := lookPath("docker")
	if err != nil {
		return fmt.Errorf("docker is not installed; install Docker and ensure the daemon is running: %w", err)
	}
	cmd := exec.CommandContext(ctx, dockerPath, "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon is not reachable; ensure Docker is running: %w", err)
	}
	return nil
}
