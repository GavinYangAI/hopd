//go:build !darwin

package gui

import "syscall"

func detachAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }
