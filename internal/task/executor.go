package task

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/jarosser06/runbook/internal/config"
	"github.com/jarosser06/runbook/internal/logs"
	"github.com/jarosser06/runbook/internal/template"
)

// Executor handles execution of one-shot tasks
type Executor struct {
	manifest *config.Manifest
}

// NewExecutor creates a new task executor
func NewExecutor(manifest *config.Manifest) *Executor {
	return &Executor{
		manifest: manifest,
	}
}

// resolveWorkingDirectory determines the working directory for a task
// Priority: 1) parameter if exposed and provided, 2) static task field
func resolveWorkingDirectory(task config.Task, params map[string]interface{}) string {
	if task.ExposeWorkingDirectory {
		if wd, ok := params["working_directory"].(string); ok && wd != "" {
			return wd
		}
	}
	return task.WorkingDirectory
}

// applyDefaults merges default parameter values into the provided params map
// Returns a new map with defaults applied for missing parameters
func (e *Executor) applyDefaults(task config.Task, params map[string]interface{}) map[string]interface{} {
	// Create a new map to avoid modifying the original
	result := make(map[string]interface{})

	// Copy provided params
	for k, v := range params {
		result[k] = v
	}

	// Apply defaults for missing parameters
	for paramName, paramDef := range task.Parameters {
		if _, exists := result[paramName]; !exists && paramDef.Default != nil {
			result[paramName] = *paramDef.Default
		}
	}

	return result
}

// Execute runs a one-shot task with the given parameters
func (e *Executor) Execute(taskName string, params map[string]interface{}) (*ExecutionResult, error) {
	// Get task definition
	task, exists := e.manifest.Tasks[taskName]
	if !exists {
		return nil, fmt.Errorf("task '%s' not found", taskName)
	}

	// Verify task type
	if task.Type == config.TaskTypeDaemon {
		return nil, fmt.Errorf("task '%s' is a daemon, use daemon operations instead", taskName)
	}

	// Generate session ID
	sessionID := logs.GenerateSessionID()
	startTime := time.Now()

	// Apply default parameter values
	params = e.applyDefaults(task, params)

	// Substitute parameters in command
	command, err := template.SubstituteParameters(task.Command, params)
	if err != nil {
		return &ExecutionResult{
			Success:  false,
			TaskName: taskName,
			Error:    fmt.Sprintf("parameter substitution failed: %v", err),
			Duration: time.Since(startTime),
		}, nil
	}

	// Determine shell
	shell := task.Shell
	if shell == "" {
		shell = "/bin/bash"
	}

	// Create command
	cmd := exec.Command(shell, "-c", command)

	// Set working directory
	workingDir := resolveWorkingDirectory(task, params)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range task.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Create buffers for output
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Get current working directory for metadata
	cwd, _ := os.Getwd()
	if workingDir != "" {
		cwd = workingDir
	}

	// Create session metadata
	metadata := &logs.SessionMetadata{
		SessionID:  sessionID,
		TaskName:   taskName,
		TaskType:   "oneshot",
		StartTime:  startTime,
		Parameters: params,
		Command:    command,
		WorkingDir: cwd,
	}

	// Create log writer
	logWriter, err := logs.NewWriter(sessionID, metadata)
	if err != nil {
		return &ExecutionResult{
			Success:   false,
			TaskName:  taskName,
			Error:     fmt.Sprintf("failed to create log writer: %v", err),
			Duration:  time.Since(startTime),
			SessionID: sessionID,
		}, nil
	}
	defer logWriter.Close()

	// Handle timeout
	var ctx context.Context
	var cancel context.CancelFunc
	timedOut := false

	if task.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(task.Timeout)*time.Second)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return &ExecutionResult{
			Success:  false,
			TaskName: taskName,
			Error:    fmt.Sprintf("failed to start command: %v", err),
			Duration: time.Since(startTime),
		}, nil
	}

	// Wait for command to complete or timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Timeout occurred
		if cmd.Process != nil {
			if killErr := cmd.Process.Kill(); killErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to kill process: %v\n", killErr)
			}
		}
		timedOut = true
		// Wait for Wait() to complete after kill
		<-done
	case <-done:
		// Command completed (error is captured in ProcessState)
	}

	duration := time.Since(startTime)

	// Get output - safe now because cmd.Wait() has returned
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// Write to log file
	logContent := stdout
	if stderr != "" {
		logContent += "\n" + stderr
	}
	if _, err := logWriter.Write([]byte(logContent)); err != nil {
		// Log write error but don't fail the task
		fmt.Fprintf(os.Stderr, "Warning: failed to write to log: %v\n", err)
	}

	// Determine success
	exitCode := 0
	success := true
	errorMsg := ""

	if timedOut {
		success = false
		exitCode = -1
		errorMsg = fmt.Sprintf("command timed out after %d seconds", task.Timeout)
	} else if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
		if exitCode != 0 {
			success = false
			errorMsg = fmt.Sprintf("command exited with code %d", exitCode)
		}
	}

	// Update writer metadata with execution results
	logWriter.UpdateMetadata(map[string]interface{}{
		"exit_code": exitCode,
		"success":   success,
		"timed_out": timedOut,
	})

	return &ExecutionResult{
		Success:   success,
		ExitCode:  exitCode,
		Stdout:    stdout,
		Stderr:    stderr,
		Duration:  duration,
		Error:     errorMsg,
		TaskName:  taskName,
		LogPath:   logWriter.GetLogPath(),
		TimedOut:  timedOut,
		SessionID: sessionID,
	}, nil
}
