package run

import (
	"strings"
	"time"
)

// DurationValue wraps time.Duration to implement pflag.Value with cleaner
// String() output: trailing "0s" and "0m" components are stripped so that
// help text reads "(default 30m)" instead of "(default 30m0s)".
type DurationValue time.Duration

func (d *DurationValue) Set(s string) error {
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = DurationValue(v)
	return nil
}

func (d DurationValue) String() string {
	s := time.Duration(d).String()
	if strings.HasSuffix(s, "0s") && len(s) > 2 && (s[len(s)-3] == 'm' || s[len(s)-3] == 'h') {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "0m") && len(s) > 2 && s[len(s)-3] == 'h' {
		s = s[:len(s)-2]
	}
	return s
}

func (d DurationValue) Type() string {
	return "duration"
}
