//go:build !linux

package incus

// 非 Linux 平台（如开发机）无法读取 /proc 或 statfs，返回零值即可。

func procStatBusyNs() int64 { return 0 }

func statfsRoot() (total, used int64, ok bool) { return 0, 0, false }
