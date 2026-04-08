package adapter

// UpdateResult holds the outcome of an agent auto-update attempt.
// A zero-value UpdateResult means no update was attempted.
type UpdateResult struct {
	Attempted     bool
	Success       bool
	CachedVersion string
	BakedVersion  string
	ElapsedMs     int64
	Error         string
	CacheHostPath string // host path to cache dir; empty if no cache
}
