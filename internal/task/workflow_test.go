package task

import (
	"os"
	"testing"

	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/logs"
)

func setupWorkflowTest(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}
	return func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}
}

func TestWorkflowExecutorBasic(t *testing.T) {
	cleanup := setupWorkflowTest(t)
	defer cleanup()

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"lint": {
				Description: "Run linter",
				Command:     "echo lint-ok",
				Type:        config.TaskTypeOneShot,
			},
			"test": {
				Description: "Run tests",
				Command:     "echo test-ok",
				Type:        config.TaskTypeOneShot,
			},
			"build": {
				Description: "Build project",
				Command:     "echo build-ok",
				Type:        config.TaskTypeOneShot,
			},
		},
		Workflows: map[string]config.Workflow{
			"ci": {
				Description: "Run full CI pipeline",
				Steps: []config.WorkflowStep{
					{Task: "lint"},
					{Task: "test"},
					{Task: "build"},
				},
			},
		},
	}

	executor := NewExecutor(manifest)
	we := NewWorkflowExecutor(executor, manifest)

	result, err := we.Execute("ci", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}
	if result.WorkflowName != "ci" {
		t.Errorf("expected workflow name 'ci', got %q", result.WorkflowName)
	}
	if result.StepsRun != 3 {
		t.Errorf("expected 3 steps run, got %d", result.StepsRun)
	}
	if result.StepsFailed != 0 {
		t.Errorf("expected 0 steps failed, got %d", result.StepsFailed)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 step results, got %d", len(result.Steps))
	}
	for i, step := range result.Steps {
		if step.Skipped {
			t.Errorf("step %d should not be skipped", i)
		}
		if step.Result == nil {
			t.Errorf("step %d result should not be nil", i)
		} else if !step.Result.Success {
			t.Errorf("step %d should have succeeded", i)
		}
	}
}

func TestWorkflowExecutorStopsOnFailure(t *testing.T) {
	cleanup := setupWorkflowTest(t)
	defer cleanup()

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"lint": {
				Description: "Run linter",
				Command:     "echo lint-ok",
				Type:        config.TaskTypeOneShot,
			},
			"test": {
				Description: "Run failing tests",
				Command:     "exit 1",
				Type:        config.TaskTypeOneShot,
			},
			"build": {
				Description: "Build project",
				Command:     "echo build-ok",
				Type:        config.TaskTypeOneShot,
			},
		},
		Workflows: map[string]config.Workflow{
			"ci": {
				Description: "Run full CI pipeline",
				Steps: []config.WorkflowStep{
					{Task: "lint"},
					{Task: "test"},
					{Task: "build"},
				},
			},
		},
	}

	executor := NewExecutor(manifest)
	we := NewWorkflowExecutor(executor, manifest)

	result, err := we.Execute("ci", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Errorf("expected failure, got success")
	}
	if result.StepsRun != 2 {
		t.Errorf("expected 2 steps run, got %d", result.StepsRun)
	}
	if result.StepsFailed != 1 {
		t.Errorf("expected 1 step failed, got %d", result.StepsFailed)
	}

	// First step should succeed
	if !result.Steps[0].Result.Success {
		t.Errorf("step 0 (lint) should have succeeded")
	}

	// Second step should fail
	if result.Steps[1].Result.Success {
		t.Errorf("step 1 (test) should have failed")
	}

	// Third step should be skipped
	if !result.Steps[2].Skipped {
		t.Errorf("step 2 (build) should be skipped")
	}
}

func TestWorkflowExecutorContinueOnFailure(t *testing.T) {
	cleanup := setupWorkflowTest(t)
	defer cleanup()

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"lint": {
				Description: "Run linter",
				Command:     "exit 1",
				Type:        config.TaskTypeOneShot,
			},
			"test": {
				Description: "Run tests",
				Command:     "echo test-ok",
				Type:        config.TaskTypeOneShot,
			},
		},
		Workflows: map[string]config.Workflow{
			"ci": {
				Description: "CI with continue",
				Steps: []config.WorkflowStep{
					{Task: "lint", ContinueOnFailure: true},
					{Task: "test"},
				},
			},
		},
	}

	executor := NewExecutor(manifest)
	we := NewWorkflowExecutor(executor, manifest)

	result, err := we.Execute("ci", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Overall should fail since a step failed
	if result.Success {
		t.Errorf("expected overall failure since lint failed")
	}
	if result.StepsRun != 2 {
		t.Errorf("expected 2 steps run (continue_on_failure), got %d", result.StepsRun)
	}
	if result.StepsFailed != 1 {
		t.Errorf("expected 1 step failed, got %d", result.StepsFailed)
	}

	// Lint step should fail but not skip subsequent
	if result.Steps[0].Result.Success {
		t.Errorf("step 0 (lint) should have failed")
	}
	if result.Steps[0].Skipped {
		t.Errorf("step 0 should not be skipped")
	}

	// Test step should still run and succeed
	if !result.Steps[1].Result.Success {
		t.Errorf("step 1 (test) should have succeeded")
	}
}

func TestWorkflowExecutorWithParams(t *testing.T) {
	cleanup := setupWorkflowTest(t)
	defer cleanup()

	defaultFlags := "-v"
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"test": {
				Description: "Run tests",
				Command:     "echo {{.flags}}",
				Type:        config.TaskTypeOneShot,
			},
		},
		Workflows: map[string]config.Workflow{
			"ci": {
				Description: "CI with params",
				Parameters: map[string]config.Param{
					"test_flags": {
						Type:        "string",
						Description: "Flags for test step",
						Default:     &defaultFlags,
					},
				},
				Steps: []config.WorkflowStep{
					{
						Task: "test",
						Params: map[string]string{
							"flags": "{{.test_flags}}",
						},
					},
				},
			},
		},
	}

	executor := NewExecutor(manifest)
	we := NewWorkflowExecutor(executor, manifest)

	// Test with default params
	result, err := we.Execute("ci", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}
	if result.Steps[0].Result == nil {
		t.Fatalf("step 0 result should not be nil")
	}
	if result.Steps[0].Result.Stdout != "-v\n" {
		t.Errorf("expected stdout '-v\\n', got %q", result.Steps[0].Result.Stdout)
	}

	// Test with overridden params
	result, err = we.Execute("ci", map[string]interface{}{"test_flags": "-race"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}
	if result.Steps[0].Result.Stdout != "-race\n" {
		t.Errorf("expected stdout '-race\\n', got %q", result.Steps[0].Result.Stdout)
	}
}

func TestWorkflowExecutorNotFound(t *testing.T) {
	cleanup := setupWorkflowTest(t)
	defer cleanup()

	manifest := &config.Manifest{
		Version:   "1.0",
		Tasks:     map[string]config.Task{},
		Workflows: map[string]config.Workflow{},
	}

	executor := NewExecutor(manifest)
	we := NewWorkflowExecutor(executor, manifest)

	_, err := we.Execute("nonexistent", map[string]interface{}{})
	if err == nil {
		t.Errorf("expected error for nonexistent workflow")
	}
}

func TestWorkflowExecutorTimeout(t *testing.T) {
	cleanup := setupWorkflowTest(t)
	defer cleanup()

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"slow": {
				Description: "Slow task",
				Command:     "sleep 5",
				Type:        config.TaskTypeOneShot,
			},
			"fast": {
				Description: "Fast task",
				Command:     "echo done",
				Type:        config.TaskTypeOneShot,
			},
		},
		Workflows: map[string]config.Workflow{
			"ci": {
				Description: "CI with timeout",
				Timeout:     1,
				Steps: []config.WorkflowStep{
					{Task: "slow"},
					{Task: "fast"},
				},
			},
		},
	}

	executor := NewExecutor(manifest)
	we := NewWorkflowExecutor(executor, manifest)

	result, err := we.Execute("ci", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Errorf("expected failure due to timeout")
	}
	// The slow task runs but the workflow timeout should eventually cause skipping
	// Since the slow task has no per-task timeout, it will run to the workflow timeout
}

func TestWorkflowManagerExecuteWorkflow(t *testing.T) {
	cleanup := setupWorkflowTest(t)
	defer cleanup()

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"lint": {
				Description: "Run linter",
				Command:     "echo lint-ok",
				Type:        config.TaskTypeOneShot,
			},
			"test": {
				Description: "Run tests",
				Command:     "echo test-ok",
				Type:        config.TaskTypeOneShot,
			},
		},
		Workflows: map[string]config.Workflow{
			"ci": {
				Description: "Run CI",
				Steps: []config.WorkflowStep{
					{Task: "lint"},
					{Task: "test"},
				},
			},
		},
	}

	manager := NewManager(manifest, NewMockProcessManager())
	result, err := manager.ExecuteWorkflow("ci", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}
	if result.StepsRun != 2 {
		t.Errorf("expected 2 steps run, got %d", result.StepsRun)
	}
}
