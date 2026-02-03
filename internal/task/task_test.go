package task

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jarosser06/dev-toolkit-mcp/internal/config"
	"github.com/jarosser06/dev-toolkit-mcp/internal/logs"
)

// Mock ProcessManager for testing
type MockProcessManager struct {
	processes map[string]*mockProcess
}

type mockProcess struct {
	pid       int
	running   bool
	sessionID string
}

func NewMockProcessManager() *MockProcessManager {
	return &MockProcessManager{
		processes: make(map[string]*mockProcess),
	}
}

func (m *MockProcessManager) Start(taskName string, sessionID string, cmd string, env map[string]string, cwd string, logPath string) error {
	if _, exists := m.processes[taskName]; exists && m.processes[taskName].running {
		return fmt.Errorf("process already running")
	}
	m.processes[taskName] = &mockProcess{
		pid:       12345,
		running:   true,
		sessionID: sessionID,
	}
	return nil
}

func (m *MockProcessManager) Stop(taskName string) error {
	if proc, exists := m.processes[taskName]; exists && proc.running {
		proc.running = false
		return nil
	}
	return fmt.Errorf("process not running")
}

func (m *MockProcessManager) Status(taskName string) (bool, int, error) {
	if proc, exists := m.processes[taskName]; exists {
		return proc.running, proc.pid, nil
	}
	return false, 0, nil
}

func (m *MockProcessManager) StopAll() error {
	for _, proc := range m.processes {
		proc.running = false
	}
	return nil
}

func (m *MockProcessManager) GetSessionID(taskName string) (string, error) {
	if proc, exists := m.processes[taskName]; exists {
		return proc.sessionID, nil
	}
	return "", fmt.Errorf("process not found")
}

func TestExecutorExecute(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	tests := []struct {
		name       string
		manifest   *config.Manifest
		taskName   string
		params     map[string]interface{}
		wantError  bool
		checkFunc  func(*testing.T, *ExecutionResult)
	}{
		{
			name: "successful command",
			manifest: &config.Manifest{
				Version: "1.0",
				Tasks: map[string]config.Task{
					"test": {
						Description: "Echo test",
						Command:     "echo 'hello world'",
						Type:        config.TaskTypeOneShot,
					},
				},
			},
			taskName:  "test",
			params:    map[string]interface{}{},
			wantError: false,
			checkFunc: func(t *testing.T, result *ExecutionResult) {
				if !result.Success {
					t.Errorf("expected success, got failure: %s", result.Error)
				}
				if !strings.Contains(result.Stdout, "hello world") {
					t.Errorf("expected stdout to contain 'hello world', got: %s", result.Stdout)
				}
				if result.ExitCode != 0 {
					t.Errorf("expected exit code 0, got %d", result.ExitCode)
				}
			},
		},
		{
			name: "failed command",
			manifest: &config.Manifest{
				Version: "1.0",
				Tasks: map[string]config.Task{
					"test": {
						Description: "Exit with error",
						Command:     "exit 1",
						Type:        config.TaskTypeOneShot,
					},
				},
			},
			taskName:  "test",
			params:    map[string]interface{}{},
			wantError: false,
			checkFunc: func(t *testing.T, result *ExecutionResult) {
				if result.Success {
					t.Errorf("expected failure, got success")
				}
				if result.ExitCode != 1 {
					t.Errorf("expected exit code 1, got %d", result.ExitCode)
				}
			},
		},
		{
			name: "command with parameters",
			manifest: &config.Manifest{
				Version: "1.0",
				Tasks: map[string]config.Task{
					"test": {
						Description: "Echo parameter",
						Command:     "echo {{.message}}",
						Type:        config.TaskTypeOneShot,
					},
				},
			},
			taskName: "test",
			params:   map[string]interface{}{"message": "test message"},
			checkFunc: func(t *testing.T, result *ExecutionResult) {
				if !result.Success {
					t.Errorf("expected success, got failure: %s", result.Error)
				}
				if !strings.Contains(result.Stdout, "test message") {
					t.Errorf("expected stdout to contain 'test message', got: %s", result.Stdout)
				}
			},
		},
		{
			name: "command with timeout",
			manifest: &config.Manifest{
				Version: "1.0",
				Tasks: map[string]config.Task{
					"test": {
						Description: "Sleep command",
						Command:     "sleep 5",
						Type:        config.TaskTypeOneShot,
						Timeout:     1,
					},
				},
			},
			taskName: "test",
			params:   map[string]interface{}{},
			checkFunc: func(t *testing.T, result *ExecutionResult) {
				if result.Success {
					t.Errorf("expected failure due to timeout, got success")
				}
				if !result.TimedOut {
					t.Errorf("expected TimedOut to be true")
				}
			},
		},
		{
			name: "missing task",
			manifest: &config.Manifest{
				Version: "1.0",
				Tasks:   map[string]config.Task{},
			},
			taskName:  "nonexistent",
			params:    map[string]interface{}{},
			wantError: true,
		},
		{
			name: "daemon task",
			manifest: &config.Manifest{
				Version: "1.0",
				Tasks: map[string]config.Task{
					"daemon": {
						Description: "Daemon task",
						Command:     "sleep 100",
						Type:        config.TaskTypeDaemon,
					},
				},
			},
			taskName:  "daemon",
			params:    map[string]interface{}{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecutor(tt.manifest)
			result, err := executor.Execute(tt.taskName, tt.params)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestManagerExecuteOneShot(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"test": {
				Description: "Echo test",
				Command:     "echo 'hello'",
				Type:        config.TaskTypeOneShot,
			},
		},
	}

	manager := NewManager(manifest, NewMockProcessManager())
	result, err := manager.ExecuteOneShot("test", map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}
}

func TestManagerStartDaemon(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	tests := []struct {
		name      string
		manifest  *config.Manifest
		taskName  string
		wantError bool
		checkFunc func(*testing.T, *DaemonStartResult)
	}{
		{
			name: "start daemon successfully",
			manifest: &config.Manifest{
				Version: "1.0",
				Tasks: map[string]config.Task{
					"daemon": {
						Description: "Daemon task",
						Command:     "sleep 100",
						Type:        config.TaskTypeDaemon,
					},
				},
			},
			taskName: "daemon",
			checkFunc: func(t *testing.T, result *DaemonStartResult) {
				if !result.Success {
					t.Errorf("expected success, got failure: %s", result.Error)
				}
				if result.PID == 0 {
					t.Errorf("expected non-zero PID")
				}
			},
		},
		{
			name: "start non-daemon task",
			manifest: &config.Manifest{
				Version: "1.0",
				Tasks: map[string]config.Task{
					"oneshot": {
						Description: "One-shot task",
						Command:     "echo test",
						Type:        config.TaskTypeOneShot,
					},
				},
			},
			taskName: "oneshot",
			checkFunc: func(t *testing.T, result *DaemonStartResult) {
				if result.Success {
					t.Errorf("expected failure, got success")
				}
			},
		},
		{
			name: "start nonexistent task",
			manifest: &config.Manifest{
				Version: "1.0",
				Tasks:   map[string]config.Task{},
			},
			taskName: "nonexistent",
			checkFunc: func(t *testing.T, result *DaemonStartResult) {
				if result.Success {
					t.Errorf("expected failure, got success")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.manifest, NewMockProcessManager())
			result, err := manager.StartDaemon(tt.taskName, map[string]interface{}{})

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestManagerStopDaemon(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"daemon": {
				Description: "Daemon task",
				Command:     "sleep 100",
				Type:        config.TaskTypeDaemon,
			},
		},
	}

	manager := NewManager(manifest, NewMockProcessManager())

	// Start daemon first
	_, err = manager.StartDaemon("daemon", map[string]interface{}{})
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	// Stop daemon
	result, err := manager.StopDaemon("daemon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}

	// Try to stop again (should fail)
	result, err = manager.StopDaemon("daemon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Errorf("expected failure when stopping already stopped daemon")
	}
}

func TestManagerDaemonStatus(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"daemon": {
				Description: "Daemon task",
				Command:     "sleep 100",
				Type:        config.TaskTypeDaemon,
			},
		},
	}

	manager := NewManager(manifest, NewMockProcessManager())

	// Check status before starting
	status, err := manager.DaemonStatus("daemon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Running {
		t.Errorf("expected daemon to not be running")
	}

	// Start daemon
	_, err = manager.StartDaemon("daemon", map[string]interface{}{})
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	// Check status after starting
	status, err = manager.DaemonStatus("daemon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Running {
		t.Errorf("expected daemon to be running")
	}
	if status.PID == 0 {
		t.Errorf("expected non-zero PID")
	}
}

func TestExecutionResultTiming(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"sleep": {
				Description: "Sleep briefly",
				Command:     "sleep 0.1",
				Type:        config.TaskTypeOneShot,
			},
		},
	}

	executor := NewExecutor(manifest)
	result, err := executor.Execute("sleep", map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Duration < 100*time.Millisecond {
		t.Errorf("expected duration >= 100ms, got %v", result.Duration)
	}
}
