package task

import (
	"sync"
	"testing"

	"github.com/jarosser06/dev-workflow-mcp/internal/config"
)

func TestDedupKey(t *testing.T) {
	// Same task+params should produce same key
	key1 := dedupKey("test", map[string]interface{}{"a": "1", "b": "2"})
	key2 := dedupKey("test", map[string]interface{}{"b": "2", "a": "1"})
	if key1 != key2 {
		t.Errorf("expected same key for same params in different order, got %s != %s", key1, key2)
	}

	// Different task names should produce different keys
	key3 := dedupKey("other", map[string]interface{}{"a": "1", "b": "2"})
	if key1 == key3 {
		t.Errorf("expected different keys for different task names")
	}

	// Different params should produce different keys
	key4 := dedupKey("test", map[string]interface{}{"a": "1", "b": "3"})
	if key1 == key4 {
		t.Errorf("expected different keys for different params")
	}

	// Nil and empty params should produce same key
	key5 := dedupKey("test", nil)
	key6 := dedupKey("test", map[string]interface{}{})
	if key5 != key6 {
		t.Errorf("expected same key for nil and empty params, got %s != %s", key5, key6)
	}
}

func TestDedupExecutor(t *testing.T) {
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"echo": {
				Description: "Echo test",
				Command:     "echo hello",
				Type:        config.TaskTypeOneShot,
			},
		},
	}

	executor := NewExecutor(manifest)
	dedup := NewDedupExecutor(executor)

	// Basic execution should work
	result, err := dedup.Execute("echo", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}
}

func TestDedupExecutorConcurrent(t *testing.T) {
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"slow": {
				Description: "Slow task",
				Command:     "sleep 0.2 && echo done",
				Type:        config.TaskTypeOneShot,
			},
		},
	}

	executor := NewExecutor(manifest)
	dedup := NewDedupExecutor(executor)

	// Launch multiple concurrent requests for the same task
	const concurrency = 5
	var wg sync.WaitGroup
	results := make([]*ExecutionResult, concurrency)
	errors := make([]error, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errors[idx] = dedup.Execute("slow", nil)
		}(i)
	}
	wg.Wait()

	// All should succeed
	for i, err := range errors {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, err)
		}
	}
	for i, r := range results {
		if r == nil {
			t.Errorf("goroutine %d: nil result", i)
			continue
		}
		if !r.Success {
			t.Errorf("goroutine %d: expected success, got: %s", i, r.Error)
		}
	}

	// All results should share the same session ID (deduped)
	sessionID := results[0].SessionID
	for i := 1; i < concurrency; i++ {
		if results[i] != nil && results[i].SessionID != sessionID {
			t.Errorf("goroutine %d: expected session ID %s, got %s (not deduped)", i, sessionID, results[i].SessionID)
		}
	}
}

func TestDedupExecutorDifferentParams(t *testing.T) {
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"echo": {
				Description: "Echo test",
				Command:     "echo {{.message}}",
				Type:        config.TaskTypeOneShot,
				Parameters: map[string]config.Param{
					"message": {
						Type:        "string",
						Required:    true,
						Description: "Message",
					},
				},
			},
		},
	}

	executor := NewExecutor(manifest)
	dedup := NewDedupExecutor(executor)

	// Different params should not be deduped
	var wg sync.WaitGroup
	var result1, result2 *ExecutionResult

	wg.Add(2)
	go func() {
		defer wg.Done()
		result1, _ = dedup.Execute("echo", map[string]interface{}{"message": "hello"})
	}()
	go func() {
		defer wg.Done()
		result2, _ = dedup.Execute("echo", map[string]interface{}{"message": "world"})
	}()
	wg.Wait()

	// Should have different session IDs (separate executions)
	if result1 != nil && result2 != nil && result1.SessionID == result2.SessionID {
		t.Errorf("expected different session IDs for different params, both got %s", result1.SessionID)
	}
}
