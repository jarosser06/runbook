package process

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/jarosser06/dev-toolkit-mcp/internal/logs"
)

// ProcessInfo holds information about a running process
type ProcessInfo struct {
	PID       int
	Cmd       *exec.Cmd
	StartTime time.Time
	LogFile   string
	SessionID string
	done      chan struct{} // Closed when process exits
}

// Manager manages daemon processes
type Manager struct {
	processes map[string]*ProcessInfo
	mu        sync.RWMutex
}

// NewManager creates a new process manager
func NewManager() *Manager {
	return &Manager{
		processes: make(map[string]*ProcessInfo),
	}
}

// Start starts a new daemon process
func (pm *Manager) Start(taskName string, sessionID string, cmd string, env map[string]string, cwd string, logPath string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if already running
	if proc, exists := pm.processes[taskName]; exists {
		// Check if process is actually alive
		if isProcessAlive(proc.PID) {
			return fmt.Errorf("daemon '%s' is already running with PID %d", taskName, proc.PID)
		}
		// Process is dead, remove it
		delete(pm.processes, taskName)
	}

	// Create command
	command := exec.Command("/bin/bash", "-c", cmd)

	// Set working directory
	if cwd != "" {
		command.Dir = cwd
	}

	// Set environment variables
	command.Env = os.Environ()
	for key, value := range env {
		command.Env = append(command.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Create session directory
	if err := logs.CreateSessionDirectory(sessionID); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Get current working directory for metadata
	workingDir, _ := os.Getwd()
	if cwd != "" {
		workingDir = cwd
	}

	// Create session metadata
	startTime := time.Now()
	metadata := &logs.SessionMetadata{
		SessionID:  sessionID,
		TaskName:   taskName,
		TaskType:   "daemon",
		StartTime:  startTime,
		Command:    cmd,
		WorkingDir: workingDir,
	}

	// Write initial session metadata
	if err := logs.WriteSessionMetadata(sessionID, metadata); err != nil {
		return fmt.Errorf("failed to write session metadata: %w", err)
	}

	// Create latest symlink
	if err := logs.CreateLatestLink(taskName, sessionID); err != nil {
		// Non-fatal error
		fmt.Fprintf(os.Stderr, "Warning: failed to create latest symlink: %v\n", err)
	}

	// Open log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Set stdout and stderr to log file
	command.Stdout = logFile
	command.Stderr = logFile

	// Start the process
	if err := command.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Store process info
	doneChan := make(chan struct{})
	pm.processes[taskName] = &ProcessInfo{
		PID:       command.Process.Pid,
		Cmd:       command,
		StartTime: startTime,
		LogFile:   logPath,
		SessionID: sessionID,
		done:      doneChan,
	}

	// Monitor process in background
	go func() {
		exitErr := command.Wait() // Capture exit status for metadata
		_ = logFile.Close()        // Ignore close errors during cleanup

		// Update session metadata with end time and exit code
		endTime := time.Now()
		duration := endTime.Sub(startTime)
		exitCode := 0
		success := true

		if exitErr != nil {
			if exitStatus, ok := command.ProcessState.Sys().(syscall.WaitStatus); ok {
				exitCode = exitStatus.ExitStatus()
			}
			success = false
		}

		updates := map[string]interface{}{
			"end_time":  endTime,
			"duration":  duration,
			"exit_code": exitCode,
			"success":   success,
		}

		if err := logs.UpdateSessionMetadata(sessionID, updates); err != nil {
			// Non-fatal error
			fmt.Fprintf(os.Stderr, "Warning: failed to update session metadata: %v\n", err)
		}

		close(doneChan) // Signal that Wait() has completed
		pm.mu.Lock()
		delete(pm.processes, taskName)
		pm.mu.Unlock()
	}()

	return nil
}

// Stop stops a running daemon process
func (pm *Manager) Stop(taskName string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	proc, exists := pm.processes[taskName]
	if !exists {
		return fmt.Errorf("daemon '%s' is not running", taskName)
	}

	// Check if process is actually alive
	if !isProcessAlive(proc.PID) {
		delete(pm.processes, taskName)
		return fmt.Errorf("daemon '%s' is not running", taskName)
	}

	// Send SIGTERM
	if err := proc.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for graceful shutdown (5 seconds)
	// Wait on the done channel instead of calling Wait() again to avoid race
	select {
	case <-time.After(5 * time.Second):
		// Graceful shutdown timeout, send SIGKILL
		if err := proc.Cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		// Wait for monitoring goroutine to finish
		<-proc.done
	case <-proc.done:
		// Process terminated gracefully
	}

	// Clean up (monitoring goroutine already deleted from map)
	// But we still hold the lock, so make sure it's gone
	delete(pm.processes, taskName)

	return nil
}

// Status returns the status of a daemon process
func (pm *Manager) Status(taskName string) (bool, int, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	proc, exists := pm.processes[taskName]
	if !exists {
		return false, 0, nil
	}

	// Check if process is actually alive
	if !isProcessAlive(proc.PID) {
		return false, 0, nil
	}

	return true, proc.PID, nil
}

// StopAll stops all running daemon processes
func (pm *Manager) StopAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var errors []string

	for taskName := range pm.processes {
		// Unlock temporarily to call Stop (which locks)
		pm.mu.Unlock()
		if err := pm.Stop(taskName); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", taskName, err))
		}
		pm.mu.Lock()
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to stop some daemons: %v", errors)
	}

	return nil
}

// GetProcessInfo returns process information
func (pm *Manager) GetProcessInfo(taskName string) (*ProcessInfo, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	proc, exists := pm.processes[taskName]
	if !exists {
		return nil, fmt.Errorf("daemon '%s' is not running", taskName)
	}

	return proc, nil
}

// GetSessionID returns the session ID for a running daemon
func (pm *Manager) GetSessionID(taskName string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	proc, exists := pm.processes[taskName]
	if !exists {
		return "", fmt.Errorf("daemon '%s' is not running", taskName)
	}

	return proc.SessionID, nil
}

// isProcessAlive checks if a process is alive
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
