package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestParseRawParams(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{
			name: "empty",
			args: nil,
			want: map[string]any{},
		},
		{
			name: "equals form",
			args: []string{"--key=value"},
			want: map[string]any{"key": "value"},
		},
		{
			name: "single dash equals",
			args: []string{"-key=value"},
			want: map[string]any{"key": "value"},
		},
		{
			name: "space separated",
			args: []string{"--key", "value"},
			want: map[string]any{"key": "value"},
		},
		{
			name: "multiple params",
			args: []string{"--a=1", "--b=2"},
			want: map[string]any{"a": "1", "b": "2"},
		},
		{
			name: "flag followed by another flag (no value)",
			args: []string{"--flag", "--other=x"},
			want: map[string]any{"other": "x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRawParams(tt.args)
			if len(got) != len(tt.want) {
				t.Errorf("len = %d, want %d (got=%v, want=%v)", len(got), len(tt.want), got, tt.want)
				return
			}
			for k, wv := range tt.want {
				if got[k] != wv {
					t.Errorf("param[%q] = %v, want %v", k, got[k], wv)
				}
			}
		})
	}
}

// captureOutput redirects os.Stdout and os.Stderr during fn, returning (stdout, stderr).
func captureOutput(fn func()) (string, string) {
	// Capture stdout
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	// Capture stderr
	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	fn()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)
	return string(outBytes), string(errBytes)
}

func TestPrintRemoteResult_OneShotSuccess(t *testing.T) {
	input := `{"success":true,"exit_code":0,"duration":"100ms","stdout":"hello\n","task_name":"test","session_id":"abc"}`
	stdout, stderr := captureOutput(func() {
		printRemoteResult("run_test", input)
	})

	if !strings.Contains(stdout, "hello") {
		t.Errorf("stdout %q does not contain 'hello'", stdout)
	}
	if !strings.Contains(stderr, "[OK]") {
		t.Errorf("stderr %q does not contain '[OK]'", stderr)
	}
	if strings.Contains(stdout, `{"success"`) || strings.Contains(stderr, `{"success"`) {
		t.Errorf("output contains raw JSON; stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestPrintRemoteResult_OneShotFailure(t *testing.T) {
	input := `{"success":false,"exit_code":1,"duration":"50ms","task_name":"test"}`
	_, stderr := captureOutput(func() {
		printRemoteResult("run_test", input)
	})

	if !strings.Contains(stderr, "[FAIL]") {
		t.Errorf("stderr %q does not contain '[FAIL]'", stderr)
	}
	if !strings.Contains(stderr, "exit code 1") {
		t.Errorf("stderr %q does not contain 'exit code 1'", stderr)
	}
}

func TestPrintRemoteResult_OneShotTimeout(t *testing.T) {
	input := `{"success":false,"timed_out":true,"duration":"5s","task_name":"test"}`
	_, stderr := captureOutput(func() {
		printRemoteResult("run_test", input)
	})

	if !strings.Contains(stderr, "[TIMEOUT]") {
		t.Errorf("stderr %q does not contain '[TIMEOUT]'", stderr)
	}
}

func TestPrintRemoteResult_OneShotNoRawJSON(t *testing.T) {
	input := `{"success":true,"exit_code":0,"duration":"10ms","stdout":"output\n","task_name":"test"}`
	stdout, stderr := captureOutput(func() {
		printRemoteResult("run_test", input)
	})

	combined := stdout + stderr
	if strings.HasPrefix(strings.TrimSpace(combined), "{") {
		t.Errorf("output starts with '{' (raw JSON leak): %q", combined)
	}
}

func TestPrintRemoteResult_DaemonStart(t *testing.T) {
	input := `{"success":true,"pid":1234,"log_path":"/tmp/log"}`
	_, stderr := captureOutput(func() {
		printRemoteResult("start_mydaemon", input)
	})

	if !strings.Contains(stderr, "[STARTED]") {
		t.Errorf("stderr %q does not contain '[STARTED]'", stderr)
	}
	if !strings.Contains(stderr, "1234") {
		t.Errorf("stderr %q does not contain '1234'", stderr)
	}
}

func TestPrintRemoteResult_DaemonStop(t *testing.T) {
	input := `{"success":true}`
	_, stderr := captureOutput(func() {
		printRemoteResult("stop_mydaemon", input)
	})

	if !strings.Contains(stderr, "[STOPPED]") {
		t.Errorf("stderr %q does not contain '[STOPPED]'", stderr)
	}
}

func TestPrintRemoteResult_DaemonStatus(t *testing.T) {
	input := `{"running":true,"pid":1234}`
	_, stderr := captureOutput(func() {
		printRemoteResult("status_mydaemon", input)
	})

	if !strings.Contains(stderr, "[RUNNING]") {
		t.Errorf("stderr %q does not contain '[RUNNING]'", stderr)
	}
	if !strings.Contains(stderr, "1234") {
		t.Errorf("stderr %q does not contain '1234'", stderr)
	}
}
