//go:build linux

package incus

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

const userHZ = 100

// procStatBusyNs 读取 /proc/stat 计算自开机以来所有核心的 busy 时间（ns）。
// 前端对两次采样做差分即可得到 CPU 占用率。
func procStatBusyNs() int64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		var vals []int64
		for _, f := range fields {
			v, err := strconv.ParseInt(f, 10, 64)
			if err != nil {
				continue
			}
			vals = append(vals, v)
		}
		if len(vals) < 5 {
			return 0
		}
		// user nice system idle iowait irq softirq steal ...
		var total, idle int64
		for i, v := range vals {
			total += v
			if i == 3 || i == 4 { // idle + iowait
				idle += v
			}
		}
		return (total - idle) * int64(1e9/userHZ)
	}
	return 0
}

// statfsRoot 根文件系统容量（面板所在主机的实际磁盘占用）
func statfsRoot() (total, used int64, ok bool) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs("/", &fs); err != nil {
		return 0, 0, false
	}
	bs := int64(fs.Bsize)
	return int64(fs.Blocks) * bs, int64(fs.Blocks-fs.Bfree) * bs, true
}
