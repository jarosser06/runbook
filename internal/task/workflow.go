package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jarosser06/runbook/internal/config"
)

// WorkflowExecutor handles execution of composite workflows
type WorkflowExecutor struct {
	executor *Executor
	manifest *config.Manifest
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(executor *Executor, manifest *config.Manifest) *WorkflowExecutor {
	return &WorkflowExecutor{
		executor: executor,
		manifest: manifest,
	}
}

// Execute runs a workflow by name with the given parameters
func (we *WorkflowExecutor) Execute(workflowName string, params map[string]interface{}) (*WorkflowResult, error) {
	workflow, exists := we.manifest.Workflows[workflowName]
	if !exists {
		return nil, fmt.Errorf("workflow '%s' not found", workflowName)
	}

	startTime := time.Now()

	// Apply workflow-level parameter defaults
	resolvedParams := applyWorkflowDefaults(workflow, params)

	// Create workflow-level timeout context if configured
	var ctx context.Context
	var cancel context.CancelFunc
	if workflow.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(workflow.Timeout)*time.Second)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	result := &WorkflowResult{
		WorkflowName: workflowName,
		Steps:        make([]WorkflowStepResult, len(workflow.Steps)),
	}

	allSuccess := true

	for i, step := range workflow.Steps {
		// Check if workflow timeout has expired
		select {
		case <-ctx.Done():
			// Mark remaining steps as skipped
			for j := i; j < len(workflow.Steps); j++ {
				result.Steps[j] = WorkflowStepResult{
					StepIndex: j,
					TaskName:  workflow.Steps[j].Task,
					Skipped:   true,
				}
			}
			result.Error = fmt.Sprintf("workflow timed out after %d seconds at step %d (%s)", workflow.Timeout, i, step.Task)
			result.Success = false
			result.Duration = time.Since(startTime)
			result.StepsRun = i
			result.StepsFailed = countFailed(result.Steps)
			return result, nil
		default:
		}

		// Resolve step params by substituting workflow param values
		stepParams := resolveStepParams(step.Params, resolvedParams)

		// Execute the step task
		execResult, err := we.executor.Execute(step.Task, stepParams)

		stepResult := WorkflowStepResult{
			StepIndex: i,
			TaskName:  step.Task,
		}

		if err != nil {
			stepResult.Result = &ExecutionResult{
				Success:  false,
				TaskName: step.Task,
				Error:    err.Error(),
			}
			allSuccess = false
			result.Steps[i] = stepResult
			result.StepsRun = i + 1
			result.StepsFailed = countFailed(result.Steps)

			if !step.ContinueOnFailure {
				// Mark remaining steps as skipped
				for j := i + 1; j < len(workflow.Steps); j++ {
					result.Steps[j] = WorkflowStepResult{
						StepIndex: j,
						TaskName:  workflow.Steps[j].Task,
						Skipped:   true,
					}
				}
				result.Success = false
				result.Error = fmt.Sprintf("step %d (%s) failed: %s", i, step.Task, err.Error())
				result.Duration = time.Since(startTime)
				return result, nil
			}
			continue
		}

		stepResult.Result = execResult
		result.Steps[i] = stepResult

		if !execResult.Success {
			allSuccess = false
			if !step.ContinueOnFailure {
				// Mark remaining steps as skipped
				for j := i + 1; j < len(workflow.Steps); j++ {
					result.Steps[j] = WorkflowStepResult{
						StepIndex: j,
						TaskName:  workflow.Steps[j].Task,
						Skipped:   true,
					}
				}
				result.Success = false
				result.Error = fmt.Sprintf("step %d (%s) failed: %s", i, step.Task, execResult.Error)
				result.Duration = time.Since(startTime)
				result.StepsRun = i + 1
				result.StepsFailed = countFailed(result.Steps)
				return result, nil
			}
		}
	}

	result.Success = allSuccess
	result.Duration = time.Since(startTime)
	result.StepsRun = len(workflow.Steps)
	result.StepsFailed = countFailed(result.Steps)
	return result, nil
}

// applyWorkflowDefaults merges default workflow parameter values into the provided params
func applyWorkflowDefaults(workflow config.Workflow, params map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range params {
		result[k] = v
	}

	for paramName, paramDef := range workflow.Parameters {
		if _, exists := result[paramName]; !exists && paramDef.Default != nil {
			result[paramName] = *paramDef.Default
		}
	}

	return result
}

// resolveStepParams substitutes workflow parameter values into step param templates.
// Step params use {{.param_name}} syntax to reference workflow-level parameters.
func resolveStepParams(stepParams map[string]string, workflowParams map[string]interface{}) map[string]interface{} {
	resolved := make(map[string]interface{})

	for key, tmpl := range stepParams {
		resolved[key] = substituteTemplate(tmpl, workflowParams)
	}

	return resolved
}

// substituteTemplate performs simple {{.key}} substitution in a template string
func substituteTemplate(tmpl string, params map[string]interface{}) string {
	result := tmpl
	for key, value := range params {
		placeholder := "{{." + key + "}}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result
}

// countFailed counts the number of failed (non-skipped, non-success) steps
func countFailed(steps []WorkflowStepResult) int {
	count := 0
	for _, step := range steps {
		if !step.Skipped && step.Result != nil && !step.Result.Success {
			count++
		}
	}
	return count
}
