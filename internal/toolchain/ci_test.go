// Package toolchain holds assertions about this repository's own build and CI
// configuration. It has no production code: the tests here parse the committed
// workflow, Taskfile, and toolchain files and fail when an invariant that keeps
// CI cheap and reproducible is broken.
//
// These tests read committed, immutable repository configuration. They create no
// temporary files, touch no network, and start no containers, so they stay inside
// the unit-test tier despite reading from disk.
package toolchain

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const repoRoot = "../.."

var (
	ciWorkflow       = filepath.Join(repoRoot, ".github", "workflows", "ci.yml")
	planningWorkflow = filepath.Join(repoRoot, ".github", "workflows", "planning.yml")
	mutationWorkflow = filepath.Join(repoRoot, ".github", "workflows", "mutation.yml")
	setupAction      = filepath.Join(repoRoot, ".github", "actions", "setup", "action.yml")
)

// TestWorkflowPathLanesAreExactMirrors guards the two-lane split: ci.yml ignores
// the doc/planning paths that planning.yml claims, so every changed file lands in
// exactly one lane. If the two lists drift, files fall between the lanes and are
// silently validated by nothing at all — a failure mode with no other symptom.
func TestWorkflowPathLanesAreExactMirrors(t *testing.T) {
	t.Parallel()

	ci := loadYAML(t, ciWorkflow)
	planning := loadYAML(t, planningWorkflow)

	lanes := map[string][]string{
		"ci.yml pull_request paths-ignore": stringSlice(nodeAt(ci, "on", "pull_request", "paths-ignore")),
		"ci.yml push paths-ignore":         stringSlice(nodeAt(ci, "on", "push", "paths-ignore")),
		"planning.yml pull_request paths":  stringSlice(nodeAt(planning, "on", "pull_request", "paths")),
		"planning.yml push paths":          stringSlice(nodeAt(planning, "on", "push", "paths")),
	}

	want := sorted(lanes["ci.yml pull_request paths-ignore"])
	require.NotEmpty(t, want, "ci.yml pull_request must declare a paths-ignore lane")

	for name, got := range lanes {
		require.Equal(t, want, sorted(got),
			"%s must mirror the other path lanes exactly; update every list together", name)
	}
}

// TestCIDoesNotRunMutationTests keeps mutation testing off the pull-request path.
// gremlins re-runs the whole suite once per mutant, so reintroducing it here would
// quietly restore the slowest job in the repository.
func TestCIDoesNotRunMutationTests(t *testing.T) {
	t.Parallel()

	for _, cmd := range stepValues(t, loadYAML(t, ciWorkflow), "run") {
		require.NotContains(t, cmd, "test:mutate",
			"mutation testing belongs in mutation.yml (nightly), not in the CI lane")
		require.NotContains(t, cmd, "gremlins",
			"mutation testing belongs in mutation.yml (nightly), not in the CI lane")
	}
}

// TestMutationWorkflowIsScheduledOnly pins the nightly-only policy from the other
// side: mutation.yml must never grow a push or pull_request trigger.
func TestMutationWorkflowIsScheduledOnly(t *testing.T) {
	t.Parallel()

	triggers := mappingKeys(nodeAt(loadYAML(t, mutationWorkflow), "on"))
	require.Equal(t, []string{"schedule", "workflow_dispatch"}, sorted(triggers),
		"mutation.yml must run on a schedule and manual dispatch only")
}

// TestCIDelegatesGoToTask keeps one command surface for local and CI: every build
// or test step goes through a Taskfile target, so reproducing a CI failure locally
// is always the same command that failed.
func TestCIDelegatesGoToTask(t *testing.T) {
	t.Parallel()

	rawGo := regexp.MustCompile(`(^|[\s;&|])go\s+(build|test|vet|run|install|generate)\b`)

	for _, path := range []string{ciWorkflow, planningWorkflow, mutationWorkflow} {
		for _, cmd := range stepValues(t, loadYAML(t, path), "run") {
			require.NotRegexp(t, rawGo, cmd,
				"%s must invoke Go through a `task` target, not directly", path)
		}
	}
}

// TestWorkflowsProvisionToolchainViaSharedSetup keeps toolchain provisioning and
// the Go module/build cache in one place. A job that calls mise-action directly
// skips the cache and recompiles the dependency tree from scratch.
func TestWorkflowsProvisionToolchainViaSharedSetup(t *testing.T) {
	t.Parallel()

	workflows, err := filepath.Glob(filepath.Join(repoRoot, ".github", "workflows", "*.yml"))
	require.NoError(t, err)
	require.NotEmpty(t, workflows)

	for _, path := range workflows {
		for _, uses := range stepValues(t, loadYAML(t, path), "uses") {
			require.NotContains(t, uses, "jdx/mise-action",
				"%s must provision the toolchain via ./.github/actions/setup", path)
			require.NotContains(t, uses, "actions/setup-go",
				"%s must not pin a second Go version alongside mise.toml", path)
		}
	}
}

// TestSetupActionPinsMiseActionBySHA pins the composite action's one third-party
// dependency to an immutable commit: a moving tag can change what provisions the
// toolchain without any change landing in this repository.
func TestSetupActionPinsMiseActionBySHA(t *testing.T) {
	t.Parallel()

	sha := regexp.MustCompile(`^jdx/mise-action@[0-9a-f]{40}$`)

	var found bool
	for _, uses := range compositeUses(t, loadYAML(t, setupAction)) {
		if !regexp.MustCompile(`^jdx/mise-action@`).MatchString(uses) {
			continue
		}
		found = true
		require.Regexp(t, sha, uses, "pin jdx/mise-action to a full commit SHA")
	}
	require.True(t, found, "the setup action must provision the toolchain via jdx/mise-action")
}

// --- YAML helpers ---------------------------------------------------------
//
// Traversal works on yaml.Node rather than a decoded map because GitHub's `on:`
// key is resolved as a boolean by some YAML schemas; comparing the raw scalar
// Value sidesteps that entirely.

func loadYAML(t *testing.T, path string) *yaml.Node {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err, "read %s", path)

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &doc), "parse %s", path)
	require.Equal(t, yaml.DocumentNode, doc.Kind, "%s must hold a single YAML document", path)
	require.Len(t, doc.Content, 1, "%s must hold a single YAML document", path)
	require.Equal(t, yaml.MappingNode, doc.Content[0].Kind, "%s must start with a mapping", path)

	return doc.Content[0]
}

// child returns the value node for key in a mapping, or nil when absent.
func child(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// nodeAt walks a chain of mapping keys, returning nil if any link is missing.
func nodeAt(node *yaml.Node, keys ...string) *yaml.Node {
	for _, key := range keys {
		node = child(node, key)
	}
	return node
}

func stringSlice(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		out = append(out, item.Value)
	}
	return out
}

func mappingKeys(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	out := make([]string, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		out = append(out, node.Content[i].Value)
	}
	return out
}

// stepValues collects one field from every step of every job in a workflow.
func stepValues(t *testing.T, root *yaml.Node, field string) []string {
	t.Helper()

	jobs := child(root, "jobs")
	if jobs == nil {
		return nil
	}

	var out []string
	for i := 1; i < len(jobs.Content); i += 2 {
		steps := child(jobs.Content[i], "steps")
		if steps == nil {
			continue
		}
		for _, step := range steps.Content {
			if value := child(step, field); value != nil {
				out = append(out, value.Value)
			}
		}
	}
	return out
}

// compositeUses collects the `uses:` of every step in a composite action.
func compositeUses(t *testing.T, root *yaml.Node) []string {
	t.Helper()

	steps := nodeAt(root, "runs", "steps")
	require.NotNil(t, steps, "composite action must declare runs.steps")

	var out []string
	for _, step := range steps.Content {
		if value := child(step, "uses"); value != nil {
			out = append(out, value.Value)
		}
	}
	return out
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
