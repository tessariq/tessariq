package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFrontmatterRoundTrip(t *testing.T) {
	t.Parallel()

	value := TaskFrontmatter{
		ID:          "TASK-001-example",
		Title:       "Example",
		Status:      "todo",
		Priority:    "p1",
		Milestone:   "v0.1.0",
		SpecVersion: "v0.1.0",
		SpecRefs:    []string{"specs/tessariq-v0.1.0.md#tessariq-run-task-path"},
	}

	encoded, err := marshalFrontmatter(value, "## Summary\n\nBody\n")
	require.NoError(t, err)

	decoded, body, err := parseFrontmatter[TaskFrontmatter](encoded)
	require.NoError(t, err)
	require.Equal(t, value.ID, decoded.ID)
	require.Equal(t, value.Title, decoded.Title)
	require.Contains(t, body, "## Summary")
}

func TestParseFrontmatterRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	_, _, err := parseFrontmatter[TaskFrontmatter]([]byte("title: missing fences"))
	require.Error(t, err)

	_, _, err = parseFrontmatter[TaskFrontmatter]([]byte("---\ntitle: bad\n"))
	require.Error(t, err)
}
