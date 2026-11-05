package main

import (
	"os"
	"syscall"
)

func ownProcessGroup() *syscall.SysProcAttr { return nil }

func signalPid(pid int, sig os.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(sig)
}
