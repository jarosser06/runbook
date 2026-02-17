package task

import (
	"time"
)

// ExecutionResult represents the result of a task execution
type ExecutionResult struct {
	Success      bool          `json:"success"`
	ExitCode     int           `json:"exit_code"`
	Stdout       string        `json:"stdout,omitempty"`
	Stderr       string        `json:"stderr,omitempty"`
	Duration     time.Duration `json:"duration"`
	Error        string        `json:"error,omitempty"`
	TaskName     string        `json:"task_name"`
	LogPath      string        `json:"log_path,omitempty"`
	TimedOut     bool          `json:"timed_out"`
	SessionID    string        `json:"session_id,omitempty"`
}

// DaemonStatus represents the status of a daemon task
type DaemonStatus struct {
	Running   bool      `json:"running"`
	PID       int       `json:"pid,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	Uptime    string    `json:"uptime,omitempty"`
	LogPath   string    `json:"log_path"`
	SessionID string    `json:"session_id,omitempty"`
}

// DaemonStartResult represents the result of starting a daemon
type DaemonStartResult struct {
	Success   bool   `json:"success"`
	PID       int    `json:"pid"`
	LogPath   string `json:"log_path"`
	Error     string `json:"error,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// DaemonStopResult represents the result of stopping a daemon
type DaemonStopResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// WorkflowStepResult represents the result of a single workflow step
type WorkflowStepResult struct {
	StepIndex int              `json:"step_index"`
	TaskName  string           `json:"task_name"`
	Result    *ExecutionResult `json:"result,omitempty"`
	Skipped   bool             `json:"skipped"`
}

// WorkflowResult represents the aggregated result of a workflow execution
type WorkflowResult struct {
	Success      bool                 `json:"success"`
	WorkflowName string              `json:"workflow_name"`
	Steps        []WorkflowStepResult `json:"steps"`
	Duration     time.Duration        `json:"duration"`
	Error        string               `json:"error,omitempty"`
	StepsRun     int                  `json:"steps_run"`
	StepsFailed  int                  `json:"steps_failed"`
}
