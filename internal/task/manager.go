package task

import (
	"fmt"
	"io"

	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/logs"
	"runbookmcp.dev/internal/template"
)

// ProcessManager interface for daemon operations
// This will be implemented by the process package
type ProcessManager interface {
	Start(taskName string, sessionID string, cmd string, env map[string]string, cwd string, logPath string, shell string) error
	Stop(taskName string) error
	Status(taskName string) (bool, int, error)
	GetSessionID(taskName string) (string, error)
	StopAll() error
}

// Manager coordinates task execution
type Manager struct {
	executor         *Executor
	dedupExecutor    *DedupExecutor
	workflowExecutor *WorkflowExecutor
	processManager   ProcessManager
	manifest         *config.Manifest
}

// NewManager creates a new task manager
func NewManager(manifest *config.Manifest, processManager ProcessManager) *Manager {
	executor := NewExecutor(manifest)
	return &Manager{
		executor:         executor,
		dedupExecutor:    NewDedupExecutor(executor),
		workflowExecutor: NewWorkflowExecutor(executor, manifest),
		processManager:   processManager,
		manifest:         manifest,
	}
}

// SetStreaming configures the executor to stream stdout/stderr to the given
// writers in addition to capturing them for logging. Pass os.Stdout/os.Stderr
// for CLI use; leave unset (nil) for MCP server use to avoid polluting the
// stdio protocol stream.
func (m *Manager) SetStreaming(stdout, stderr io.Writer) {
	m.executor.stdout = stdout
	m.executor.stderr = stderr
}

// ExecuteOneShot executes a one-shot task with deduplication.
// If the same task+params is already running, callers wait for
// the existing execution and receive the same result.
func (m *Manager) ExecuteOneShot(taskName string, params map[string]interface{}) (*ExecutionResult, error) {
	return m.dedupExecutor.Execute(taskName, params)
}

// ExecuteWorkflow runs a composite workflow by name with the given parameters.
// Steps execute sequentially using the raw Executor (no dedup).
func (m *Manager) ExecuteWorkflow(workflowName string, params map[string]interface{}) (*WorkflowResult, error) {
	return m.workflowExecutor.Execute(workflowName, params)
}

// StartDaemon starts a daemon task
func (m *Manager) StartDaemon(taskName string, params map[string]interface{}) (*DaemonStartResult, error) {
	// Get task definition
	task, exists := m.manifest.Tasks[taskName]
	if !exists {
		return &DaemonStartResult{
			Success: false,
			Error:   fmt.Sprintf("task '%s' not found", taskName),
		}, nil
	}

	// Verify task type
	if task.Type != config.TaskTypeDaemon {
		return &DaemonStartResult{
			Success: false,
			Error:   fmt.Sprintf("task '%s' is not a daemon", taskName),
		}, nil
	}

	// Check if already running
	running, _, err := m.processManager.Status(taskName)
	if err != nil {
		return &DaemonStartResult{
			Success: false,
			Error:   fmt.Sprintf("failed to check status: %v", err),
		}, nil
	}
	if running {
		return &DaemonStartResult{
			Success: false,
			Error:   fmt.Sprintf("daemon '%s' is already running", taskName),
		}, nil
	}

	params = m.applyDefaults(task, params)

	command, err := template.SubstituteParameters(task.Command, params)
	if err != nil {
		return &DaemonStartResult{
			Success: false,
			Error:   fmt.Sprintf("failed to substitute parameters: %v", err),
		}, nil
	}

	sessionID := logs.GenerateSessionID()

	logPath := logs.GetSessionLogPath(sessionID)

	workingDir := resolveWorkingDirectory(task, params)
	if err := m.processManager.Start(taskName, sessionID, command, task.Env, workingDir, logPath, task.Shell); err != nil {
		return &DaemonStartResult{
			Success: false,
			Error:   fmt.Sprintf("failed to start daemon: %v", err),
		}, nil
	}

	// Get PID
	_, pid, err := m.processManager.Status(taskName)
	if err != nil {
		return &DaemonStartResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get daemon status: %v", err),
		}, nil
	}

	return &DaemonStartResult{
		Success:   true,
		PID:       pid,
		LogPath:   logPath,
		SessionID: sessionID,
	}, nil
}

// StopDaemon stops a daemon task
func (m *Manager) StopDaemon(taskName string) (*DaemonStopResult, error) {
	// Get task definition
	task, exists := m.manifest.Tasks[taskName]
	if !exists {
		return &DaemonStopResult{
			Success: false,
			Error:   fmt.Sprintf("task '%s' not found", taskName),
		}, nil
	}

	// Verify task type
	if task.Type != config.TaskTypeDaemon {
		return &DaemonStopResult{
			Success: false,
			Error:   fmt.Sprintf("task '%s' is not a daemon", taskName),
		}, nil
	}

	// Check if running
	running, _, err := m.processManager.Status(taskName)
	if err != nil {
		return &DaemonStopResult{
			Success: false,
			Error:   fmt.Sprintf("failed to check status: %v", err),
		}, nil
	}
	if !running {
		return &DaemonStopResult{
			Success: false,
			Error:   fmt.Sprintf("daemon '%s' is not running", taskName),
		}, nil
	}

	// Stop daemon
	if err := m.processManager.Stop(taskName); err != nil {
		return &DaemonStopResult{
			Success: false,
			Error:   fmt.Sprintf("failed to stop daemon: %v", err),
		}, nil
	}

	return &DaemonStopResult{
		Success: true,
		Message: fmt.Sprintf("daemon '%s' stopped successfully", taskName),
	}, nil
}

// DaemonStatus returns the status of a daemon task
func (m *Manager) DaemonStatus(taskName string) (*DaemonStatus, error) {
	// Get task definition
	task, exists := m.manifest.Tasks[taskName]
	if !exists {
		return nil, fmt.Errorf("task '%s' not found", taskName)
	}

	// Verify task type
	if task.Type != config.TaskTypeDaemon {
		return nil, fmt.Errorf("task '%s' is not a daemon", taskName)
	}

	// Get status
	running, pid, err := m.processManager.Status(taskName)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Get session ID if running
	sessionID := ""
	if running {
		sessionID, _ = m.processManager.GetSessionID(taskName)
	}

	// Get log path
	logPath := ""
	if sessionID != "" {
		logPath = logs.GetSessionLogPath(sessionID)
	}

	return &DaemonStatus{
		Running:   running,
		PID:       pid,
		LogPath:   logPath,
		SessionID: sessionID,
	}, nil
}

// GetManifest returns the manifest
func (m *Manager) GetManifest() *config.Manifest {
	return m.manifest
}

func (m *Manager) applyDefaults(task config.Task, params map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range params {
		result[k] = v
	}

	for paramName, paramDef := range task.Parameters {
		if _, exists := result[paramName]; !exists && paramDef.Default != nil {
			result[paramName] = *paramDef.Default
		}
	}

	return result
}
