//go:build !windows

package main

import (
	"os"
	"syscall"
)

// ownProcessGroup keeps Ctrl+C away from bwrap, which would otherwise die and SIGKILL the sandbox.
func ownProcessGroup() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }

func signalPid(pid int, sig os.Signal) error { return syscall.Kill(pid, sig.(syscall.Signal)) }
