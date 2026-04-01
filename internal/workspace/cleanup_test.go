package workspace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanup_NonExistentPath_ReturnsNil(t *testing.T) {
	t.Parallel()

	err := Cleanup(context.Background(), "/tmp/fake-repo-root", "/nonexistent/workspace/path")
	require.NoError(t, err, "Cleanup must return nil for a non-existent workspace path")
}
