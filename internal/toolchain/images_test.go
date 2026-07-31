package toolchain

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/proxy"
)

var (
	pullScript       = filepath.Join(repoRoot, "scripts", "pull-test-images.sh")
	referenceDockerf = filepath.Join(repoRoot, "runtime", "reference", "Dockerfile")
)

// TestDockerJobsPrePullTestImages keeps the registry pull in one cheap, retrying
// step ahead of each container-backed suite. Without it a Docker Hub outage is
// paid once per test: every container start burns its own pull timeout before
// failing, so a single registry blip turns into a full-length red job (see the
// 111 identical `registry-1.docker.io: context deadline exceeded` failures on
// 2026-07-31).
func TestDockerJobsPrePullTestImages(t *testing.T) {
	t.Parallel()

	jobs := []struct {
		name string
		pull string
		run  string
	}{
		{name: "integration-tests", pull: "test:images:pull:integration", run: "test:integration"},
		{name: "e2e-tests", pull: "test:images:pull:e2e", run: "test:e2e"},
	}

	ci := loadYAML(t, ciWorkflow)

	for _, job := range jobs {
		steps := jobSteps(t, ci, job.name)
		require.NotEmpty(t, steps, "ci.yml must define a %s job", job.name)

		pull, run := -1, -1
		for i, step := range steps {
			cmd := child(step, "run")
			if cmd == nil {
				continue
			}
			switch {
			case strings.Contains(cmd.Value, job.pull):
				pull = i
			case strings.Contains(cmd.Value, job.run):
				run = i
			}
		}

		require.NotEqual(t, -1, pull, "%s must pre-pull images via `task %s`", job.name, job.pull)
		require.NotEqual(t, -1, run, "%s must run the suite via `task %s`", job.name, job.run)
		require.Less(t, pull, run, "%s must pre-pull images before running the suite", job.name)
	}
}

// TestTestImagePullListCoversPinnedImages ties the pull list to the digests the
// code actually uses. A bumped digest that the list does not follow is silent:
// the suite still passes, it just pulls the real image lazily again and loses
// the protection this step exists to provide.
func TestTestImagePullListCoversPinnedImages(t *testing.T) {
	t.Parallel()

	list := readFile(t, pullScript)

	require.Contains(t, list, container.RepairImage,
		"pull list must track container.RepairImage")
	require.Contains(t, list, proxy.DefaultSquidImage,
		"pull list must track proxy.DefaultSquidImage")
	require.Contains(t, list, referenceRuntimeBase(t),
		"pull list must track the reference runtime base image")
}

// referenceRuntimeBase returns the image in the reference runtime Dockerfile's
// FROM line, which the runtime image integration tests build from.
func referenceRuntimeBase(t *testing.T) string {
	t.Helper()

	scanner := bufio.NewScanner(strings.NewReader(readFile(t, referenceDockerf)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && strings.EqualFold(fields[0], "FROM") {
			return fields[1]
		}
	}
	require.NoError(t, scanner.Err())

	t.Fatalf("no FROM instruction in %s", referenceDockerf)
	return ""
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err, "read %s", path)
	return string(data)
}

// jobSteps returns the step nodes of one named job.
func jobSteps(t *testing.T, root *yaml.Node, job string) []*yaml.Node {
	t.Helper()

	steps := nodeAt(root, "jobs", job, "steps")
	if steps == nil {
		return nil
	}
	return steps.Content
}
