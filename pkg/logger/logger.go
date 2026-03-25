package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func LogMsg(msg string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	logDir := filepath.Join(home, ".planner")
	logFile := filepath.Join(logDir, "log.log")

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	timestamp := time.Now().UTC().Format(time.RFC3339)
	logMsg := fmt.Sprintf("%s: %s\n", timestamp, msg)

	_, err = f.WriteString(logMsg)
	if err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	return nil
}

func Log(err error) error {
	return LogMsg(err.Error())
}
