package container

import "testing"

func TestIsNotFoundError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "docker_desktop_inspect_capitalized",
			text: "Error: No such object: tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV",
			want: true,
		},
		{
			name: "docker_desktop_rm_capitalized",
			text: "Error response from daemon: No such container: tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV",
			want: true,
		},
		{
			name: "alpine_cli_inspect_lowercase",
			text: "error: no such object: tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV",
			want: true,
		},
		{
			name: "alpine_cli_rm_lowercase",
			text: "error: no such container: tessariq-01ARZ3NDEKTSV4RRFFQ69G5FAV",
			want: true,
		},
		{
			name: "mixed_case",
			text: "ERROR: No Such Object: tessariq-x",
			want: true,
		},
		{
			name: "unrelated_error_not_matched",
			text: "Error response from daemon: container tessariq-x is already in use",
			want: false,
		},
		{
			name: "empty_string",
			text: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isNotFoundError(tt.text)
			if got != tt.want {
				t.Errorf("isNotFoundError(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}
