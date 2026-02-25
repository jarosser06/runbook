//go:build windows

package process

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
)

// getProcAttrs returns Windows-specific process attributes.
// CREATE_NEW_PROCESS_GROUP makes the child the leader of a new process group,
// equivalent to Setpgid=true on Unix.
func getProcAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killProcessGroup terminates a process and its entire child tree on Windows.
// Uses taskkill /F /T /PID which forcefully kills the process tree.
// Exit code 128 from taskkill means the process is already gone — treated as success.
func killProcessGroup(pid int, sig syscall.Signal) error {
	err := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
	if err == nil {
		return nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
		return nil // process already gone
	}
	return fmt.Errorf("failed to kill process group (PID %d): %w", pid, err)
}
