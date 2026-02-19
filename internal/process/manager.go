package process

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"runbookmcp.dev/internal/logs"
)

// ProcessInfo holds information about a running process
type ProcessInfo struct {
	PID       int
	OwnerID   string    // UUID of the Manager that started this daemon
	Cmd       *exec.Cmd // nil for daemons restored from PID files
	StartTime time.Time
	LogFile   string
	SessionID string
	done      chan struct{} // Closed when process exits
}

// Manager manages daemon processes
type Manager struct {
	ownerID   string // unique ID for this Manager instance
	processes map[string]*ProcessInfo
	mu        sync.RWMutex
}

// NewManager creates a new process manager with a unique owner ID and restores
// any daemons that are still running from previous invocations.
func NewManager() *Manager {
	pm := &Manager{
		ownerID:   uuid.New().String(),
		processes: make(map[string]*ProcessInfo),
	}
	pm.restoreFromPIDFiles()
	return pm
}

// restoreFromPIDFiles scans the pids directory on startup. For each PID file:
//   - Dead daemon process: remove the stale file.
//   - Alive daemon, dead owner process: adopt the orphan (assign to this Manager)
//     so it can be stopped by the current invocation.
//   - Alive daemon, alive owner process: restore with the original OwnerID so
//     that only the owning process can stop it; other Managers can still read
//     status and logs.
func (pm *Manager) restoreFromPIDFiles() {
	files, err := scanPIDFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to scan PID files: %v\n", err)
		return
	}

	for _, data := range files {
		if !isProcessAlive(data.PID) {
			deletePIDFile(data.TaskName)
			continue
		}

		// Determine effective owner. If the process that originally started this
		// daemon is no longer alive, the daemon is an orphan — adopt it so it
		// can be managed (stopped) by the current invocation.
		effectiveOwnerID := data.OwnerID
		if !isProcessAlive(data.OwnerPID) {
			effectiveOwnerID = pm.ownerID
		}

		doneChan := make(chan struct{})
		pm.processes[data.TaskName] = &ProcessInfo{
			PID:       data.PID,
			OwnerID:   effectiveOwnerID,
			Cmd:       nil,
			StartTime: data.StartTime,
			LogFile:   data.LogFile,
			SessionID: data.SessionID,
			done:      doneChan,
		}

		// Poll until the process exits so the map entry and PID file are
		// cleaned up automatically even if no one explicitly stops it.
		taskName := data.TaskName
		pid := data.PID
		go func() {
			for isProcessAlive(pid) {
				time.Sleep(500 * time.Millisecond)
			}
			deletePIDFile(taskName)
			close(doneChan)
			pm.mu.Lock()
			delete(pm.processes, taskName)
			pm.mu.Unlock()
		}()
	}
}

// Start starts a new daemon process
func (pm *Manager) Start(taskName string, sessionID string, cmd string, env map[string]string, cwd string, logPath string, shell string) error {
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

	if shell == "" {
		shell = "/bin/bash"
	}

	// Create command
	command := exec.Command(shell, "-c", cmd)

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

	// Set process group attributes for proper daemon isolation
	// This creates a new process group with the daemon as leader (PGID == PID)
	// All children spawned by the daemon will inherit this PGID
	// This allows us to terminate the entire process tree with kill(-pgid, signal)
	command.SysProcAttr = getProcAttrs()

	// Start the process
	if err := command.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Persist PID so subsequent CLI invocations can discover this daemon
	if err := writePIDFile(pidFileData{
		PID:       command.Process.Pid,
		OwnerID:   pm.ownerID,
		OwnerPID:  os.Getpid(),
		SessionID: sessionID,
		TaskName:  taskName,
		StartTime: startTime,
		LogFile:   logPath,
	}); err != nil {
		// Non-fatal: in-process tracking still works; warn and continue
		fmt.Fprintf(os.Stderr, "Warning: failed to write PID file: %v\n", err)
	}

	// Store process info
	doneChan := make(chan struct{})
	pm.processes[taskName] = &ProcessInfo{
		PID:       command.Process.Pid,
		OwnerID:   pm.ownerID,
		Cmd:       command,
		StartTime: startTime,
		LogFile:   logPath,
		SessionID: sessionID,
		done:      doneChan,
	}

	// Monitor process in background
	go func() {
		exitErr := command.Wait() // Capture exit status for metadata
		_ = logFile.Close()       // Ignore close errors during cleanup

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

		deletePIDFile(taskName)
		close(doneChan) // Signal that Wait() has completed
		pm.mu.Lock()
		delete(pm.processes, taskName)
		pm.mu.Unlock()
	}()

	return nil
}

// Stop stops a running daemon process. Returns an error if the daemon is not
// running or was started by a different Manager instance (ownership check).
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

	// Ownership check: only the Manager that started the daemon can stop it.
	// Other standalone instances can observe (status/logs) but not modify.
	if proc.OwnerID != pm.ownerID {
		return fmt.Errorf("daemon '%s' is owned by another runbook process and cannot be stopped from here", taskName)
	}

	// Send SIGTERM to entire process group
	// The daemon's PID equals its PGID (because we set Setpgid=true)
	// Negative PID means send to all processes in that process group
	// This terminates the daemon AND all its children
	if err := killProcessGroup(proc.PID, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process group: %w", err)
	}

	// Wait for graceful shutdown (5 seconds)
	// Wait on the done channel instead of calling Wait() again to avoid race
	select {
	case <-time.After(5 * time.Second):
		// Graceful shutdown timeout, send SIGKILL to entire process group
		// This force-kills the daemon and all children that didn't exit gracefully
		if err := killProcessGroup(proc.PID, syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to kill process group: %w", err)
		}
		// Wait for monitoring goroutine to finish
		<-proc.done
	case <-proc.done:
		// Process terminated gracefully
	}

	// Clean up (monitoring goroutine already deleted from map)
	// But we still hold the lock, so make sure it's gone
	delete(pm.processes, taskName)
	deletePIDFile(taskName)

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

// StopAll stops all daemon processes owned by this Manager instance.
// Daemons started by other Manager instances are left running.
func (pm *Manager) StopAll() error {
	pm.mu.Lock()
	names := make([]string, 0, len(pm.processes))
	for name, proc := range pm.processes {
		if proc.OwnerID == pm.ownerID {
			names = append(names, name)
		}
	}
	pm.mu.Unlock()

	var errors []string
	for _, name := range names {
		if err := pm.Stop(name); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
		}
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
