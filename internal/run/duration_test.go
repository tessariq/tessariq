package run

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDurationValue_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "30m0s becomes 30m", d: 30 * time.Minute, want: "30m"},
		{name: "1h0m0s becomes 1h", d: 1 * time.Hour, want: "1h"},
		{name: "1h30m0s becomes 1h30m", d: 1*time.Hour + 30*time.Minute, want: "1h30m"},
		{name: "5m30s stays 5m30s", d: 5*time.Minute + 30*time.Second, want: "5m30s"},
		{name: "90s becomes 1m30s", d: 90 * time.Second, want: "1m30s"},
		{name: "500ms stays 500ms", d: 500 * time.Millisecond, want: "500ms"},
		{name: "0s stays 0s", d: 0, want: "0s"},
		{name: "30s stays 30s", d: 30 * time.Second, want: "30s"},
		{name: "2h0m0s becomes 2h", d: 2 * time.Hour, want: "2h"},
		{name: "1h0m30s becomes 1h0m30s", d: 1*time.Hour + 30*time.Second, want: "1h0m30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dv := DurationValue(tt.d)
			require.Equal(t, tt.want, dv.String())
		})
	}
}

func TestDurationValue_Set(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "parse 30m", input: "30m", want: 30 * time.Minute},
		{name: "parse 1h", input: "1h", want: 1 * time.Hour},
		{name: "parse 1h30m", input: "1h30m", want: 1*time.Hour + 30*time.Minute},
		{name: "parse 5m30s", input: "5m30s", want: 5*time.Minute + 30*time.Second},
		{name: "parse 500ms", input: "500ms", want: 500 * time.Millisecond},
		{name: "invalid input", input: "not-a-duration", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var dv DurationValue
			err := dv.Set(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, time.Duration(dv))
		})
	}
}

func TestDurationValue_Type(t *testing.T) {
	t.Parallel()

	var dv DurationValue
	require.Equal(t, "duration", dv.Type())
}

func TestDurationValue_SetStringRoundTrip(t *testing.T) {
	t.Parallel()

	durations := []time.Duration{
		30 * time.Minute,
		1 * time.Hour,
		5*time.Minute + 30*time.Second,
		90 * time.Second,
	}

	for _, d := range durations {
		dv := DurationValue(d)
		s := dv.String()

		var parsed DurationValue
		require.NoError(t, parsed.Set(s))
		require.Equal(t, d, time.Duration(parsed), "round-trip failed for %s", s)
	}
}
