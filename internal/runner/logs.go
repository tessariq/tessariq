package runner

import (
	"fmt"
	"os"
	"path/filepath"
)

// LogFiles manages durable run.log and runner.log file handles.
type LogFiles struct {
	RunLog    *os.File
	RunnerLog *os.File
}

// OpenLogs creates run.log and runner.log in the evidence directory.
func OpenLogs(evidenceDir string) (*LogFiles, error) {
	runLog, err := os.OpenFile(filepath.Join(evidenceDir, "run.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create run.log: %w", err)
	}

	runnerLog, err := os.OpenFile(filepath.Join(evidenceDir, "runner.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		runLog.Close()
		return nil, fmt.Errorf("create runner.log: %w", err)
	}

	return &LogFiles{RunLog: runLog, RunnerLog: runnerLog}, nil
}

// Close flushes and closes both log files.
func (l *LogFiles) Close() error {
	var firstErr error
	if l.RunLog != nil {
		if err := l.RunLog.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		l.RunLog = nil
	}
	if l.RunnerLog != nil {
		if err := l.RunnerLog.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		l.RunnerLog = nil
	}
	return firstErr
}
