package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSpecRefAlias(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ref     string
		version string
		want    string
	}{
		{
			name:    "adapter-contract resolves to agent-and-runtime-contract",
			ref:     "specs/tessariq-v0.1.0.md#adapter-contract",
			version: "v0.1.0",
			want:    "specs/tessariq-v0.1.0.md#agent-and-runtime-contract",
		},
		{
			name:    "non-aliased ref passes through unchanged",
			ref:     "specs/tessariq-v0.1.0.md#release-intent",
			version: "v0.1.0",
			want:    "specs/tessariq-v0.1.0.md#release-intent",
		},
		{
			name:    "unknown version passes through unchanged",
			ref:     "specs/tessariq-v0.1.0.md#adapter-contract",
			version: "v0.2.0",
			want:    "specs/tessariq-v0.1.0.md#adapter-contract",
		},
		{
			name:    "malformed ref passes through unchanged",
			ref:     "no-anchor",
			version: "v0.1.0",
			want:    "no-anchor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveSpecRefAlias(tt.ref, tt.version)
			require.Equal(t, tt.want, got)
		})
	}
}
