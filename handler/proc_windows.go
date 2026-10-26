//go:build windows

package handler

import "os/exec"

// prepareProcessGroup Windows 无进程组语义，空实现（安装脚本本身只在 Linux 上运行）。
func prepareProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup Windows 下只结束直接子进程。
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
