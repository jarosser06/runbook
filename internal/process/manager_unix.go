//go:build unix

package process

import (
	"fmt"
	"syscall"
)

// getProcAttrs returns Unix-specific process attributes that create
// a new process group with the spawned process as the leader.
//
// This is CRITICAL for daemon cleanup: When Setpgid=true, the daemon
// becomes the leader of its own process group (PGID == PID). All child
// processes spawned by the daemon inherit this PGID. This allows us to
// terminate the entire process tree with a single signal to the group.
func getProcAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true, // Create new process group, daemon becomes leader
	}
}

// killProcessGroup sends a signal to an entire process group.
//
// The negative PID (-pgid) is the standard Unix way to signal a process
// group. All processes with that PGID receive the signal, including the
// daemon and all its descendants.
//
// This is safe because each daemon has its own isolated process group.
// We only affect processes spawned by that specific daemon.
func killProcessGroup(pid int, sig syscall.Signal) error {
	// Send signal to process group (negative PID)
	// The kernel correctly interprets negative values despite type conversion
	err := syscall.Kill(-pid, sig)
	if err != nil {
		// ESRCH means no such process/group - acceptable if already dead
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("failed to signal process group %d: %w", pid, err)
	}
	return nil
}

