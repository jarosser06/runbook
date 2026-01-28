package logs

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
)

// ReadOptions contains options for reading log files
type ReadOptions struct {
	Lines  int    // Number of lines to tail (0 means all)
	Filter string // Regex pattern to filter lines (empty means no filter)
}

// ReadLog reads the log file for a task with optional tailing and filtering
func ReadLog(taskName string, opts ReadOptions) ([]string, error) {
	logPath := GetLogPath(taskName)

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return []string{}, nil // No log file yet
	}

	// Open log file
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	// Apply filter if specified
	if opts.Filter != "" {
		lines, err = filterLines(lines, opts.Filter)
		if err != nil {
			return nil, fmt.Errorf("failed to filter lines: %w", err)
		}
	}

	// Apply tail if specified
	if opts.Lines > 0 && len(lines) > opts.Lines {
		lines = lines[len(lines)-opts.Lines:]
	}

	return lines, nil
}

// filterLines filters lines using a regex pattern
func filterLines(lines []string, pattern string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var filtered []string
	for _, line := range lines {
		if re.MatchString(line) {
			filtered = append(filtered, line)
		}
	}

	return filtered, nil
}

// TailLog returns the last N lines from a log file
func TailLog(taskName string, lines int) ([]string, error) {
	return ReadLog(taskName, ReadOptions{Lines: lines})
}

// FilterLog returns all lines matching a regex pattern
func FilterLog(taskName string, pattern string) ([]string, error) {
	return ReadLog(taskName, ReadOptions{Filter: pattern})
}
