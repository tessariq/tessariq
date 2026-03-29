package workspace

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "lowercase passthrough", in: "tessariq", want: "tessariq"},
		{name: "uppercase to lower", in: "TessariQ", want: "tessariq"},
		{name: "spaces to hyphens", in: "my project", want: "my-project"},
		{name: "special chars to hyphens", in: "my_project.v2", want: "my-project-v2"},
		{name: "consecutive hyphens collapsed", in: "my--project", want: "my-project"},
		{name: "leading hyphens trimmed", in: "--leading", want: "leading"},
		{name: "trailing hyphens trimmed", in: "trailing--", want: "trailing"},
		{name: "mixed special chars", in: "My_Cool..Project!!!", want: "my-cool-project"},
		{name: "single char", in: "a", want: "a"},
		{name: "numbers preserved", in: "project123", want: "project123"},
		{name: "only special chars", in: "___", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, slug(tt.in))
		})
	}
}

func TestShortHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
	}{
		{name: "standard path", in: "/home/user/code/tessariq"},
		{name: "root path", in: "/"},
		{name: "empty string", in: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := shortHash(tt.in)
			require.Len(t, h, 8, "shortHash must be exactly 8 characters")
			require.Regexp(t, `^[0-9a-f]{8}$`, h, "shortHash must be lowercase hex")
		})
	}
}

func TestShortHash_Deterministic(t *testing.T) {
	t.Parallel()

	h1 := shortHash("/home/user/code/tessariq")
	h2 := shortHash("/home/user/code/tessariq")
	require.Equal(t, h1, h2)
}

func TestShortHash_DifferentInputsDifferentOutput(t *testing.T) {
	t.Parallel()

	h1 := shortHash("/home/user/code/tessariq")
	h2 := shortHash("/home/user/code/other-repo")
	require.NotEqual(t, h1, h2)
}

func TestRepoID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		repoRoot string
		wantSlug string
	}{
		{
			name:     "standard path",
			repoRoot: "/home/user/code/tessariq",
			wantSlug: "tessariq",
		},
		{
			name:     "path with special chars in basename",
			repoRoot: "/home/user/My_Project.v2",
			wantSlug: "my-project-v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := RepoID(tt.repoRoot)
			hash := shortHash(tt.repoRoot)
			expected := tt.wantSlug + "-" + hash
			require.Equal(t, expected, id)
		})
	}
}

func TestRepoID_Deterministic(t *testing.T) {
	t.Parallel()

	id1 := RepoID("/home/user/code/tessariq")
	id2 := RepoID("/home/user/code/tessariq")
	require.Equal(t, id1, id2)
}

func TestRepoID_DifferentPathsDifferentIDs(t *testing.T) {
	t.Parallel()

	id1 := RepoID("/home/alice/tessariq")
	id2 := RepoID("/home/bob/tessariq")
	require.NotEqual(t, id1, id2, "same basename at different paths must produce different repo IDs")
}

func TestRepoID_Format(t *testing.T) {
	t.Parallel()

	id := RepoID("/home/user/code/tessariq")
	require.Regexp(t, `^[a-z0-9-]+-[0-9a-f]{8}$`, id)
}
