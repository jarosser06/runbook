package task

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jarosser06/runbook/internal/config"
	"github.com/jarosser06/runbook/internal/logs"
)

type MockProcessManager struct {
	processes   map[string]*mockProcess
	capturedCwd string
}

type mockProcess struct {
	pid       int
	running   bool
	sessionID string
	command   string
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
	m.capturedCwd = cwd
	m.processes[taskName] = &mockProcess{
		pid:       12345,
		running:   true,
		sessionID: sessionID,
		command:   cmd,
	}
	return nil
}

func (m *MockProcessManager) GetCommand(taskName string) (string, error) {
	if proc, exists := m.processes[taskName]; exists {
		return proc.command, nil
	}
	return "", fmt.Errorf("process not found")
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

func TestDaemonParameterSubstitution(t *testing.T) {
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
				Description: "Daemon with parameters",
				Command:     "echo {{.port}}",
				Type:        config.TaskTypeDaemon,
			},
		},
	}

	mockPM := NewMockProcessManager()
	manager := NewManager(manifest, mockPM)

	params := map[string]interface{}{"port": "8080"}
	_, err = manager.StartDaemon("daemon", params)
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	cmd, err := mockPM.GetCommand("daemon")
	if err != nil {
		t.Fatalf("failed to get command: %v", err)
	}

	expected := "echo 8080"
	if cmd != expected {
		t.Errorf("expected command %q, got %q", expected, cmd)
	}
}

func TestApplyDefaultsWithEmptyString(t *testing.T) {
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

	emptyString := ""
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"test": {
				Description: "Task with empty string default",
				Command:     "echo 'port:[{{.port}}]'",
				Type:        config.TaskTypeOneShot,
				Parameters: map[string]config.Param{
					"port": {
						Type:        "string",
						Description: "Port number",
						Default:     &emptyString,
					},
				},
			},
		},
	}

	executor := NewExecutor(manifest)
	result, err := executor.Execute("test", map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}

	expected := "port:[]\n"
	if result.Stdout != expected {
		t.Errorf("expected stdout to be %q (empty string applied), got: %q", expected, result.Stdout)
	}
}

func TestApplyDefaultsWithValue(t *testing.T) {
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

	defaultValue := "8080"
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"test": {
				Description: "Task with default value",
				Command:     "echo 'port:[{{.port}}]'",
				Type:        config.TaskTypeOneShot,
				Parameters: map[string]config.Param{
					"port": {
						Type:        "string",
						Description: "Port number",
						Default:     &defaultValue,
					},
				},
			},
		},
	}

	executor := NewExecutor(manifest)
	result, err := executor.Execute("test", map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}

	expected := "port:[8080]\n"
	if result.Stdout != expected {
		t.Errorf("expected stdout to be %q (default value applied), got: %q", expected, result.Stdout)
	}
}

func TestWorkingDirectoryResolution(t *testing.T) {
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

	// Create subdirectories for testing
	subDir1 := tmpDir + "/subdir1"
	subDir2 := tmpDir + "/subdir2"
	if err := os.MkdirAll(subDir1, 0755); err != nil {
		t.Fatalf("failed to create subdir1: %v", err)
	}
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatalf("failed to create subdir2: %v", err)
	}

	tests := []struct {
		name                     string
		taskWorkingDirectory     string
		exposeWorkingDirectory   bool
		providedParameter        interface{}
		expectedWorkingDirectory string
	}{
		{
			name:                     "static working_directory only",
			taskWorkingDirectory:     subDir1,
			exposeWorkingDirectory:   false,
			providedParameter:        nil,
			expectedWorkingDirectory: subDir1,
		},
		{
			name:                     "exposed with parameter provided",
			taskWorkingDirectory:     subDir1,
			exposeWorkingDirectory:   true,
			providedParameter:        subDir2,
			expectedWorkingDirectory: subDir2,
		},
		{
			name:                     "exposed without parameter",
			taskWorkingDirectory:     subDir1,
			exposeWorkingDirectory:   true,
			providedParameter:        nil,
			expectedWorkingDirectory: subDir1,
		},
		{
			name:                     "exposed with empty string",
			taskWorkingDirectory:     subDir1,
			exposeWorkingDirectory:   true,
			providedParameter:        "",
			expectedWorkingDirectory: subDir1,
		},
		{
			name:                     "not exposed, parameter provided",
			taskWorkingDirectory:     subDir1,
			exposeWorkingDirectory:   false,
			providedParameter:        subDir2,
			expectedWorkingDirectory: subDir1,
		},
		{
			name:                     "parameter type validation - non-string",
			taskWorkingDirectory:     subDir1,
			exposeWorkingDirectory:   true,
			providedParameter:        12345,
			expectedWorkingDirectory: subDir1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := &config.Manifest{
				Version: "1.0",
				Tasks: map[string]config.Task{
					"test": {
						Description:            "Test working directory",
						Command:                "pwd",
						Type:                   config.TaskTypeOneShot,
						WorkingDirectory:       tt.taskWorkingDirectory,
						ExposeWorkingDirectory: tt.exposeWorkingDirectory,
					},
				},
			}

			executor := NewExecutor(manifest)
			params := map[string]interface{}{}
			if tt.providedParameter != nil {
				params["working_directory"] = tt.providedParameter
			}

			result, err := executor.Execute("test", params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.Success {
				t.Errorf("expected success, got failure: %s", result.Error)
			}

			// Verify the command ran in the expected directory
			// Note: macOS may resolve paths differently (e.g., /private prefix)
			// so we use filepath.EvalSymlinks to normalize both paths
			actualDir := strings.TrimSpace(result.Stdout)

			// Resolve both paths to handle symlink differences
			expectedResolved, err := os.Readlink(tt.expectedWorkingDirectory)
			if err != nil {
				expectedResolved = tt.expectedWorkingDirectory
			}
			actualResolved, err := os.Readlink(actualDir)
			if err != nil {
				actualResolved = actualDir
			}

			// Compare the resolved paths or fall back to direct comparison
			// if they match after resolution
			if actualResolved != expectedResolved && actualDir != tt.expectedWorkingDirectory {
				// One final check - see if they're the same inode
				expectedStat, err1 := os.Stat(tt.expectedWorkingDirectory)
				actualStat, err2 := os.Stat(actualDir)
				if err1 == nil && err2 == nil && os.SameFile(expectedStat, actualStat) {
					// Same file, test passes
					return
				}
				t.Errorf("expected working directory %q, got %q", tt.expectedWorkingDirectory, actualDir)
			}
		})
	}
}

func TestWorkingDirectoryResolutionDaemon(t *testing.T) {
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

	subDir1 := tmpDir + "/subdir1"
	subDir2 := tmpDir + "/subdir2"
	if err := os.MkdirAll(subDir1, 0755); err != nil {
		t.Fatalf("failed to create subdir1: %v", err)
	}
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatalf("failed to create subdir2: %v", err)
	}

	tests := []struct {
		name                     string
		taskWorkingDirectory     string
		exposeWorkingDirectory   bool
		providedParameter        interface{}
		expectedWorkingDirectory string
	}{
		{
			name:                     "daemon - static working_directory",
			taskWorkingDirectory:     subDir1,
			exposeWorkingDirectory:   false,
			providedParameter:        nil,
			expectedWorkingDirectory: subDir1,
		},
		{
			name:                     "daemon - exposed with parameter",
			taskWorkingDirectory:     subDir1,
			exposeWorkingDirectory:   true,
			providedParameter:        subDir2,
			expectedWorkingDirectory: subDir2,
		},
		{
			name:                     "daemon - exposed without parameter",
			taskWorkingDirectory:     subDir1,
			exposeWorkingDirectory:   true,
			providedParameter:        nil,
			expectedWorkingDirectory: subDir1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := &config.Manifest{
				Version: "1.0",
				Tasks: map[string]config.Task{
					"daemon": {
						Description:            "Daemon with working directory",
						Command:                "sleep 100",
						Type:                   config.TaskTypeDaemon,
						WorkingDirectory:       tt.taskWorkingDirectory,
						ExposeWorkingDirectory: tt.exposeWorkingDirectory,
					},
				},
			}

			mockPM := NewMockProcessManager()
			manager := NewManager(manifest, mockPM)
			params := map[string]interface{}{}
			if tt.providedParameter != nil {
				params["working_directory"] = tt.providedParameter
			}

			_, err := manager.StartDaemon("daemon", params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if mockPM.capturedCwd != tt.expectedWorkingDirectory {
				t.Errorf("expected working directory %q passed to process manager, got %q", tt.expectedWorkingDirectory, mockPM.capturedCwd)
			}
		})
	}
}
