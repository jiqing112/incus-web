//go:build !windows

package handler

import (
	"os/exec"
	"syscall"
)

// prepareProcessGroup 让子进程成为独立进程组组长，
// 便于超时时整组清理（安装脚本会派生 apt/dnf/git 等孙进程）。
func prepareProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup 杀掉整个进程组（负 pid 表示进程组）。
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}
