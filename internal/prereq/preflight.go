package prereq

import (
	"errors"
	"fmt"
	"os/exec"
)

type Dependency string

const (
	DependencyGit  Dependency = "git"
	DependencyTmux Dependency = "tmux"
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
		return []Dependency{DependencyGit, DependencyTmux}, nil
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
