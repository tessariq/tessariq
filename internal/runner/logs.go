package runner

import (
	"fmt"
	"os"
	"path/filepath"
)

// LogFiles manages durable run.log and runner.log file handles with
// write-time capping.
type LogFiles struct {
	runLogFile    *os.File
	runnerLogFile *os.File
	RunLog        *CappedWriter
	RunnerLog     *CappedWriter
}

// OpenLogs creates run.log and runner.log in the evidence directory,
// wrapped with capped writers. If capBytes is <= 0, DefaultLogCapBytes
// is used.
func OpenLogs(evidenceDir string, capBytes int64) (*LogFiles, error) {
	if capBytes <= 0 {
		capBytes = DefaultLogCapBytes
	}

	runLog, err := os.OpenFile(filepath.Join(evidenceDir, "run.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create run.log: %w", err)
	}

	runnerLog, err := os.OpenFile(filepath.Join(evidenceDir, "runner.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		runLog.Close()
		return nil, fmt.Errorf("create runner.log: %w", err)
	}

	return &LogFiles{
		runLogFile:    runLog,
		runnerLogFile: runnerLog,
		RunLog:        NewCappedWriter(runLog, capBytes),
		RunnerLog:     NewCappedWriter(runnerLog, capBytes),
	}, nil
}

// RunLogPath returns the filesystem path of run.log.
func (l *LogFiles) RunLogPath() string {
	return l.runLogFile.Name()
}

// Close flushes and closes both underlying log files.
func (l *LogFiles) Close() error {
	var firstErr error
	if l.runLogFile != nil {
		if err := l.runLogFile.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		l.runLogFile = nil
		l.RunLog = nil
	}
	if l.runnerLogFile != nil {
		if err := l.runnerLogFile.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		l.runnerLogFile = nil
		l.RunnerLog = nil
	}
	return firstErr
}
