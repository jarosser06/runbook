package server

import (
	"os"
	"testing"

	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/logs"
	"runbookmcp.dev/internal/task"
)

func TestOneShotTruncationEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	})
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	if err := logs.Setup(); err != nil {
		t.Fatal(err)
	}

	manifest := &config.Manifest{
		Version: "1",
		Tasks: map[string]config.Task{
			"bigout": {
				Type:        config.TaskTypeOneShot,
				Description: "200 lines stdout",
				Command:     "seq 1 200",
			},
			"bigboth": {
				Type:        config.TaskTypeOneShot,
				Description: "200 stdout 150 stderr",
				Command:     "bash -c 'seq 1 200; seq 1 150 >&2'",
			},
			"smallout": {
				Type:        config.TaskTypeOneShot,
				Description: "50 lines stdout",
				Command:     "seq 1 50",
			},
		},
	}

	mgr := task.NewManager(manifest, nil)

	t.Run("200-line stdout is truncated to 100", func(t *testing.T) {
		result, err := mgr.ExecuteOneShot("bigout", map[string]interface{}{})
		if err != nil {
			t.Fatalf("ExecuteOneShot: %v", err)
		}
		_, shown, total := truncateToLines(result.Stdout, mcpOutputMaxLines)
		if total != 200 {
			t.Errorf("stdout total=%d, want 200", total)
		}
		if shown != 100 {
			t.Errorf("stdout shown=%d, want 100", shown)
		}
		if total <= shown {
			t.Errorf("expected stdout_truncated=true, total=%d shown=%d", total, shown)
		}
	})

	t.Run("stdout and stderr each truncated independently", func(t *testing.T) {
		result, err := mgr.ExecuteOneShot("bigboth", map[string]interface{}{})
		if err != nil {
			t.Fatalf("ExecuteOneShot: %v", err)
		}
		_, outShown, outTotal := truncateToLines(result.Stdout, mcpOutputMaxLines)
		_, errShown, errTotal := truncateToLines(result.Stderr, mcpOutputMaxLines)

		if outTotal != 200 {
			t.Errorf("stdout total=%d, want 200", outTotal)
		}
		if outShown != 100 {
			t.Errorf("stdout shown=%d, want 100", outShown)
		}
		if errTotal != 150 {
			t.Errorf("stderr total=%d, want 150", errTotal)
		}
		if errShown != 100 {
			t.Errorf("stderr shown=%d, want 100", errShown)
		}
	})

	t.Run("under-limit output is not truncated", func(t *testing.T) {
		result, err := mgr.ExecuteOneShot("smallout", map[string]interface{}{})
		if err != nil {
			t.Fatalf("ExecuteOneShot: %v", err)
		}
		_, shown, total := truncateToLines(result.Stdout, mcpOutputMaxLines)
		if total != shown {
			t.Errorf("expected no truncation: total=%d shown=%d", total, shown)
		}
		if total < 50 {
			t.Errorf("expected at least 50 lines, got %d", total)
		}
	})
}
