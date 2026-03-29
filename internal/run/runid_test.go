package run

import (
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestNewRunID_ValidULID(t *testing.T) {
	t.Parallel()

	now := time.Now()
	id, err := NewRunID(now)
	require.NoError(t, err)
	require.Len(t, id, 26)

	parsed, err := ulid.Parse(id)
	require.NoError(t, err)
	require.WithinDuration(t, now, ulid.Time(parsed.Time()), time.Second)
}

func TestNewRunID_Monotonicity(t *testing.T) {
	t.Parallel()

	now := time.Now()
	id1, err := NewRunID(now)
	require.NoError(t, err)

	id2, err := NewRunID(now.Add(time.Millisecond))
	require.NoError(t, err)

	require.NotEqual(t, id1, id2)
	require.True(t, id2 > id1, "later run ID should sort after earlier")
}

func TestNewRunID_AllCrockfordChars(t *testing.T) {
	t.Parallel()

	id, err := NewRunID(time.Now())
	require.NoError(t, err)

	for _, c := range id {
		require.True(t, strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", c),
			"character %q is not valid Crockford Base32", c)
	}
}

func TestIsValidRunID_AcceptsGenerated(t *testing.T) {
	t.Parallel()

	id, err := NewRunID(time.Now())
	require.NoError(t, err)
	require.True(t, IsValidRunID(id))
}

func TestIsValidRunID_RejectsWrongLength(t *testing.T) {
	t.Parallel()

	require.False(t, IsValidRunID(""))
	require.False(t, IsValidRunID("01H"))
	require.False(t, IsValidRunID("01HXG5F2ZZZZZZZZZZZZZZZZZextra"))
}

func TestIsValidRunID_RejectsInvalidChars(t *testing.T) {
	t.Parallel()

	require.False(t, IsValidRunID("01HXG5F2ZZZZZZZZZZZZZZZZI"))
	require.False(t, IsValidRunID("01HXG5F2ZZZZZZZZZZZZZZZZL"))
	require.False(t, IsValidRunID("01HXG5F2ZZZZZZZZZZZZZZZZO"))
	require.False(t, IsValidRunID("01HXG5F2ZZZZZZZZZZZZZZZZU"))
	require.False(t, IsValidRunID("01HXG5F2!!!!!!!!!!!!!!!!!!"))
}

func TestIsValidRunID_RejectsLowercase(t *testing.T) {
	t.Parallel()

	require.False(t, IsValidRunID("01hxg5f2zzzzzzzzzzzzzzzza"))
}

func TestIsValidRunID_AcceptsKnownGoodULID(t *testing.T) {
	t.Parallel()

	require.True(t, IsValidRunID("01ARZ3NDEKTSV4RRFFQ69G5FAV"))
}
