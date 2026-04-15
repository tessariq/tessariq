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

func TestExtractTaskTitle_FilenameFallbackStripsControlCharacters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		filename string
		want     string
	}{
		{"newline", "Fix: bug\nSigned-off-by: attacker.md", "Fix: bugSigned-off-by: attacker"},
		{"nul", "bad\x00name.md", "badname"},
		{"unit_separator", "bad\x1fname.md", "badname"},
		{"del", "bad\x7fname.md", "badname"},
		{"carriage_return", "bad\rname.md", "badname"},
		{"tab_removed_in_fallback", "a\tb.md", "ab"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ExtractTaskTitle([]byte("no heading"), tc.filename)
			require.Equal(t, tc.want, got)
			for i := 0; i < len(got); i++ {
				b := got[i]
				require.Falsef(t, b <= 0x1f || b == 0x7f, "title leaked control byte %#x", b)
			}
		})
	}
}

func TestExtractTaskTitle_FilenameFallbackKeepsSpaceAndPunctuation(t *testing.T) {
	t.Parallel()

	title := ExtractTaskTitle([]byte("no heading"), "Fix: a bug? (v2).md")
	require.Equal(t, "Fix: a bug? (v2)", title)
}
