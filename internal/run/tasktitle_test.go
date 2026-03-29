package run

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTaskTitle_H1Present(t *testing.T) {
	t.Parallel()

	content := []byte("# My Task\n\nSome content")
	title := ExtractTaskTitle(content, "task.md")
	require.Equal(t, "My Task", title)
}

func TestExtractTaskTitle_H1WithLeadingWhitespace(t *testing.T) {
	t.Parallel()

	content := []byte("  # Indented Title\n\nContent")
	title := ExtractTaskTitle(content, "task.md")
	require.Equal(t, "Indented Title", title)
}

func TestExtractTaskTitle_MultipleH1_FirstWins(t *testing.T) {
	t.Parallel()

	content := []byte("# First Title\n\n## Section\n\n# Second Title")
	title := ExtractTaskTitle(content, "task.md")
	require.Equal(t, "First Title", title)
}

func TestExtractTaskTitle_H1WithBoldFormatting(t *testing.T) {
	t.Parallel()

	content := []byte("# **Bold** Task Title")
	title := ExtractTaskTitle(content, "task.md")
	require.Equal(t, "**Bold** Task Title", title)
}

func TestExtractTaskTitle_H1WithInlineCode(t *testing.T) {
	t.Parallel()

	content := []byte("# Fix `parseConfig` bug")
	title := ExtractTaskTitle(content, "task.md")
	require.Equal(t, "Fix `parseConfig` bug", title)
}

func TestExtractTaskTitle_H1WithSpecialCharacters(t *testing.T) {
	t.Parallel()

	content := []byte("# Add <script> & \"quotes\" handling")
	title := ExtractTaskTitle(content, "task.md")
	require.Equal(t, "Add <script> & \"quotes\" handling", title)
}

func TestExtractTaskTitle_H1WithTrailingHashes(t *testing.T) {
	t.Parallel()

	content := []byte("# Title ###")
	title := ExtractTaskTitle(content, "task.md")
	require.Equal(t, "Title", title)
}

func TestExtractTaskTitle_NoH1_FallsBackToBasename(t *testing.T) {
	t.Parallel()

	content := []byte("No heading here\n\nJust content")
	title := ExtractTaskTitle(content, "specs/feature-xyz.md")
	require.Equal(t, "feature-xyz", title)
}

func TestExtractTaskTitle_EmptyContent_FallsBackToBasename(t *testing.T) {
	t.Parallel()

	content := []byte("")
	title := ExtractTaskTitle(content, "my-task.md")
	require.Equal(t, "my-task", title)
}

func TestExtractTaskTitle_H2Only_FallsBackToBasename(t *testing.T) {
	t.Parallel()

	content := []byte("## Not H1\n\n### Also not H1")
	title := ExtractTaskTitle(content, "subtask.md")
	require.Equal(t, "subtask", title)
}

func TestExtractTaskTitle_H1WithOnlyWhitespace(t *testing.T) {
	t.Parallel()

	content := []byte("#   \n\nContent after blank H1")
	title := ExtractTaskTitle(content, "blank.md")
	require.Equal(t, "blank", title)
}

func TestExtractTaskTitle_H1AfterFrontmatter(t *testing.T) {
	t.Parallel()

	content := []byte("---\ntitle: frontmatter\n---\n\n# Real Title\n\nContent")
	title := ExtractTaskTitle(content, "task.md")
	require.Equal(t, "Real Title", title)
}

func TestExtractTaskTitle_DoubleHashNoSpace_IsNotH1(t *testing.T) {
	t.Parallel()

	content := []byte("##Not A Heading\n\n## Real H2")
	title := ExtractTaskTitle(content, "fallback.md")
	require.Equal(t, "fallback", title)
}

func TestExtractTaskTitle_H1WithTabAfterHash_FallsBack(t *testing.T) {
	t.Parallel()

	content := []byte("#\tTab Title\n\nContent")
	title := ExtractTaskTitle(content, "tab-task.md")
	require.Equal(t, "tab-task", title)
}
