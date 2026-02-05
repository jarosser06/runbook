//go:build windows

package process

import (
	"fmt"
	"syscall"
)

// getProcAttrs returns Windows-specific process attributes.
//
// NOTE: Windows process group handling is not yet implemented.
// This stub allows the code to compile on Windows, but daemon
// process cleanup may not work correctly on Windows systems.
//
// TODO: Implement proper Windows process group handling using:
// - CREATE_NEW_PROCESS_GROUP flag in CreationFlags
// - GenerateConsoleCtrlEvent or TerminateJobObject for termination
func getProcAttrs() *syscall.SysProcAttr {
	// Return minimal attributes - no process group isolation on Windows yet
	return &syscall.SysProcAttr{}
}

// killProcessGroup sends a signal to a process on Windows.
//
// NOTE: Windows doesn't have Unix-style process groups in the same way.
// This implementation only kills the immediate process, not its children.
//
// TODO: Implement proper process tree termination on Windows using
// Job Objects or enumerating child processes.
func killProcessGroup(pid int, sig syscall.Signal) error {
	// On Windows, we can only kill the immediate process
	// This will NOT kill child processes (known limitation)
	return fmt.Errorf("process group termination not implemented on Windows (PID %d)", pid)
}

