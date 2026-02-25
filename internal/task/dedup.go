package task

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// inflight tracks a single in-progress execution.
type inflight struct {
	done   chan struct{}
	result *ExecutionResult
	err    error
}

// DedupExecutor wraps an Executor and deduplicates concurrent identical
// one-shot task executions. If the same task+params is already running,
// subsequent callers wait for the original execution to complete and
// receive the same result.
type DedupExecutor struct {
	executor *Executor
	mu       sync.Mutex
	flights  map[string]*inflight
}

// NewDedupExecutor creates a DedupExecutor wrapping the given Executor.
func NewDedupExecutor(executor *Executor) *DedupExecutor {
	return &DedupExecutor{
		executor: executor,
		flights:  make(map[string]*inflight),
	}
}

// Execute runs a one-shot task, deduplicating concurrent identical requests.
// If the same taskName+params combination is already in flight, the caller
// waits for that execution to complete and receives the same result.
func (d *DedupExecutor) Execute(taskName string, params map[string]interface{}) (*ExecutionResult, error) {
	key := dedupKey(taskName, params)

	d.mu.Lock()
	if f, ok := d.flights[key]; ok {
		// Already in flight — wait for it
		d.mu.Unlock()
		<-f.done
		return f.result, f.err
	}

	// Register new in-flight entry
	f := &inflight{done: make(chan struct{})}
	d.flights[key] = f
	d.mu.Unlock()

	// Execute the task
	f.result, f.err = d.executor.Execute(taskName, params)

	// Signal completion and clean up
	close(f.done)

	d.mu.Lock()
	delete(d.flights, key)
	d.mu.Unlock()

	return f.result, f.err
}

// dedupKey computes a deterministic key from a task name and its parameters.
func dedupKey(taskName string, params map[string]interface{}) string {
	// Sort parameter keys for deterministic ordering
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sorted := make([]interface{}, 0, len(keys)*2)
	for _, k := range keys {
		sorted = append(sorted, k, params[k])
	}

	paramBytes, _ := json.Marshal(sorted)
	h := sha256.Sum256([]byte(taskName + "|" + string(paramBytes)))
	return fmt.Sprintf("%x", h)
}
