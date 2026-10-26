package handler

import (
	"bufio"
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"incus-web/config"
	"incus-web/model"

	"github.com/gin-gonic/gin"
)

//go:embed scripts/install_incus.sh
var installScriptFS embed.FS

// runWithTimeout 执行子进程并强制超时，防止 CLI 在个别环境卡死拖住整个请求
func runWithTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// InstallHandler Incus 安装向导处理器
type InstallHandler struct{}

// NewInstallHandler 创建安装处理器
func NewInstallHandler() *InstallHandler {
	return &InstallHandler{}
}

// installRunning 防止并发触发多个安装脚本（apt/dpkg 锁互斥）
var installRunning atomic.Bool

// installMaxDuration 包管理器安装的硬超时。
// 脚本内部对 apt/dnf/git/launch 都加了 timeout，但仍存在「等包管理锁」这类无限等待，
// 一旦挂住 installRunning 永远为 true，后续所有安装请求都返回 409，只能重启面板。
const installMaxDuration = 45 * time.Minute

// sourceInstallMaxDuration 源码编译的硬超时。
// 该路径要下载 Go、克隆仓库、make deps（最多重试 6 次）再整体编译，
// 45 分钟经常不够，会在编译中途被强杀，留下装了依赖却没装 incus 的半成品。
const sourceInstallMaxDuration = 150 * time.Minute

// installTimeoutFor auto 可能回退到源码编译，因此与 source 用同一预算
func installTimeoutFor(method string) time.Duration {
	if method == "package" {
		return installMaxDuration
	}
	return sourceInstallMaxDuration
}

// keepInstallRuns 保留最近几次安装日志，其余清理，避免临时目录无限堆积
const keepInstallRuns = 5

// pruneOldInstallRuns 删除历史安装目录（脚本执行完已被删，只剩日志）
func pruneOldInstallRuns(baseDir string, keep int) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}
	type runDir struct {
		path string
		mod  time.Time
	}
	runs := make([]runDir, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(baseDir, e.Name())
		if st, err := os.Stat(p); err == nil {
			runs = append(runs, runDir{path: p, mod: st.ModTime()})
		}
	}
	if len(runs) <= keep {
		return
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].mod.After(runs[j].mod) })
	for _, r := range runs[keep:] {
		_ = os.RemoveAll(r.path)
	}
}

// nameRe 存储池/网桥名称合法性（字母数字下划线连字符，≤63 位）
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,63}$`)

// installBaseDir 安装脚本与日志的根目录。
// 固定放在临时目录下的专用子目录并设为 0700：
//   - 避免其他本地用户篡改待执行的 root 脚本（TOCTOU 提权）
//   - 让日志读取校验可以精确限定在本目录内，而不是整个 /tmp
func installBaseDir() string {
	return filepath.Join(os.TempDir(), "incus-web-install")
}

// installLogName 安装日志固定文件名（便于读取侧做白名单校验）
const installLogName = "install.log"

// appendLogLine 追加一行到安装日志（用于脚本被强杀等无法自己写日志的场景）
func appendLogLine(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("15:04:05"), line)
}

// ensureDoneMarker 保证日志中一定存在 [DONE] 行。
// 前端靠该行结束轮询，缺失时会永远停在「正在安装」，因此脚本没能自己写标记时必须补上。
func ensureDoneMarker(path string, runErr error, timedOut bool, maxDur time.Duration) {
	if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), "[DONE]") {
		return
	}
	switch {
	case timedOut:
		appendLogLine(path, fmt.Sprintf("[DONE] 安装脚本超过 %d 分钟未结束，已被强制终止 (rc=124)", int(maxDur.Minutes())))
	case runErr == nil:
		// 退出码 0 却没有标记：多半是日志管道（tee）异常，以复检结果为准
		appendLogLine(path, "[DONE] 安装脚本已退出，但日志缺少结束标记，请点击「重新检测」确认结果 (rc=0)")
	default:
		code := 1
		msg := "安装脚本异常退出（未输出结束标记）"
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			code = ee.ExitCode()
		} else {
			msg = "安装脚本无法启动: " + runErr.Error()
		}
		appendLogLine(path, fmt.Sprintf("[DONE] %s (rc=%d)", msg, code))
	}
}

// incusBinary 定位 incus CLI。
// 源码安装会装到 /usr/local/bin，而 systemd 拉起的面板 PATH 可能不含该目录，
// 只用 LookPath 会在装好之后仍报「未安装」，把用户重新推回安装向导。
func incusBinary() string {
	if p, err := exec.LookPath("incus"); err == nil {
		return p
	}
	for _, p := range []string{"/usr/local/bin/incus", "/usr/bin/incus", "/usr/sbin/incus", "/snap/bin/incus"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// InstallStatus GET /api/server/install-status
// 检测宿主机上 incus 的安装/初始化/服务状态
func (h *InstallHandler) InstallStatus(c *gin.Context) {
	installed := false
	version := ""
	initialized := false
	serviceRunning := false
	osName := ""
	osVersion := ""

	// 尝试读取 OS 信息
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "NAME=") {
				osName = strings.Trim(strings.TrimPrefix(line, "NAME="), `"`)
			}
			if strings.HasPrefix(line, "VERSION_ID=") {
				osVersion = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
			}
		}
	}

	if incusBin := incusBinary(); incusBin != "" {
		installed = true
		if out, err := runWithTimeout(5*time.Second, incusBin, "--version"); err == nil {
			version = out
		}
		// 服务运行检测：优先 systemd；非 systemd（如 Alpine OpenRC）则回退到 socket/进程
		if out, err := runWithTimeout(5*time.Second, "systemctl", "is-active", "incus"); err == nil && out != "" {
			serviceRunning = out == "active"
		} else if _, err := os.Stat(config.IncusSocket()); err == nil {
			serviceRunning = true
		} else if out, err := runWithTimeout(5*time.Second, "pgrep", "-x", "incusd"); err == nil && out != "" {
			serviceRunning = true
		}
		// 检查是否已初始化：incus info 在守护进程一启动就成功，不代表已 init；
		// 以「存在至少一个存储池」为初始化依据（CSV 格式无表头，无池时输出为空）
		if _, err := runWithTimeout(5*time.Second, incusBin, "info"); err == nil {
			serviceRunning = true
			if out, err := runWithTimeout(5*time.Second, incusBin, "storage", "list", "--format", "csv"); err == nil {
				initialized = strings.TrimSpace(out) != ""
			}
		}
	}

	// socket 探测：RHEL/Fedora 系 socket 在 /run/incus，Debian 在 /var/lib/incus。
	// 面板连不上时前端必须能看到「装是装了，但面板连不上」，否则会误判为一切正常。
	socketPath := config.IncusSocket()
	socketExists := false
	if st, err := os.Stat(socketPath); err == nil && st.Mode()&os.ModeSocket != 0 {
		socketExists = true
	}
	apiReachable := false
	if socketExists {
		if conn, err := net.DialTimeout("unix", socketPath, 2*time.Second); err == nil {
			_ = conn.Close()
			apiReachable = true
		}
	}

	c.JSON(http.StatusOK, model.ApiResponse{
		Code: 200,
		Data: gin.H{
			"installed":       installed,
			"version":         version,
			"initialized":     initialized,
			"service_running": serviceRunning,
			"os_name":         osName,
			"os_version":      osVersion,
			"socket_path":     socketPath,
			"socket_exists":   socketExists,
			"api_reachable":   apiReachable,
			// 非 root 时安装脚本必然失败，前端据此提前提示，而不是等点了「开始安装」才报 403
			"is_root": os.Geteuid() == 0,
		},
	})
}

// InstallIncus POST /api/server/install
// 触发 Incus 安装（异步执行脚本）
func (h *InstallHandler) InstallIncus(c *gin.Context) {
	var req struct {
		StorageBackend string `json:"storage_backend"`
		StorageSizeGB  int    `json:"storage_size_gb"`
		StoragePool    string `json:"storage_pool"`
		NetworkBridge  string `json:"network_bridge"`
		InstallMethod  string `json:"install_method"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: err.Error()})
		return
	}

	// 默认值
	if req.StorageBackend == "" {
		req.StorageBackend = "btrfs"
	}
	if req.StorageSizeGB <= 0 {
		req.StorageSizeGB = 50
	}
	if req.StoragePool == "" {
		req.StoragePool = "default"
	}
	if req.NetworkBridge == "" {
		req.NetworkBridge = "incusbr0"
	}
	if req.InstallMethod == "" {
		req.InstallMethod = "auto"
	}

	// 参数校验：后端白名单 + 名称合法性（防止注入到安装脚本）
	switch req.StorageBackend {
	case "btrfs", "zfs", "dir":
	default:
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "不支持的存储后端: " + req.StorageBackend})
		return
	}
	switch req.InstallMethod {
	case "auto", "package", "source":
	default:
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "不支持的安装方式: " + req.InstallMethod})
		return
	}
	if !nameRe.MatchString(req.StoragePool) || !nameRe.MatchString(req.NetworkBridge) {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "存储池/网桥名称仅允许字母、数字、下划线、连字符（1-63 位）"})
		return
	}
	if req.StorageSizeGB < 1 || req.StorageSizeGB > 10000 {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "存储池大小需在 1-10000 GB 之间"})
		return
	}

	// Incus 只有 Linux 版本，非 Linux 宿主机上跑安装脚本没有意义（bash/apt 都不存在），
	// 直接拒绝比让前端盯着日志等一条「脚本无法启动」清楚得多
	if runtime.GOOS != "linux" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "安装向导仅支持 Linux 宿主机（Incus 不提供 Windows/macOS 版本）"})
		return
	}
	// 脚本要跑 apt/dnf、systemctl 并写 /etc，必须 root。
	// 非 root 时直接拒绝，比启动一个立刻失败的脚本再去日志里找原因直观得多。
	if os.Geteuid() != 0 {
		c.JSON(http.StatusForbidden, model.ApiResponse{Code: 403, Error: "面板需以 root 身份运行才能安装 Incus（当前进程不是 root）"})
		return
	}

	// 并发互斥：同一时间只允许一个安装任务
	if !installRunning.CompareAndSwap(false, true) {
		c.JSON(http.StatusConflict, model.ApiResponse{Code: 409, Error: "已有安装任务正在执行，请等待其完成或刷新日志查看进度"})
		return
	}
	// 任一步骤失败都必须复位标记，否则后续安装请求会永久返回 409（只能重启面板恢复）
	releaseInstall := func() { installRunning.Store(false) }

	// 脚本与日志放在 0700 专用目录，避免 /tmp 下的可预测路径被其他用户替换
	baseDir := installBaseDir()
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		releaseInstall()
		c.JSON(http.StatusInternalServerError, model.ApiResponse{Code: 500, Error: "创建安装工作目录失败: " + err.Error()})
		return
	}
	pruneOldInstallRuns(baseDir, keepInstallRuns)
	runDir := filepath.Join(baseDir, fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		releaseInstall()
		c.JSON(http.StatusInternalServerError, model.ApiResponse{Code: 500, Error: "创建安装工作目录失败: " + err.Error()})
		return
	}
	scriptPath := filepath.Join(runDir, "install.sh")
	logPath := filepath.Join(runDir, installLogName)

	scriptContent, err := installScriptFS.ReadFile("scripts/install_incus.sh")
	if err != nil {
		releaseInstall()
		c.JSON(http.StatusInternalServerError, model.ApiResponse{Code: 500, Error: "读取安装脚本失败: " + err.Error()})
		return
	}
	if err := os.WriteFile(scriptPath, scriptContent, 0o700); err != nil {
		releaseInstall()
		c.JSON(http.StatusInternalServerError, model.ApiResponse{Code: 500, Error: "写入安装脚本失败: " + err.Error()})
		return
	}
	// 预建空日志：脚本的 tee 首行可能晚于前端第一次轮询，先建文件可避免「日志不存在」误判
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		releaseInstall()
		c.JSON(http.StatusInternalServerError, model.ApiResponse{Code: 500, Error: "创建安装日志失败: " + err.Error()})
		return
	}

	// 异步执行安装脚本
	go func() {
		defer installRunning.Store(false)
		maxDur := installTimeoutFor(req.InstallMethod)
		ctx, cancel := context.WithTimeout(context.Background(), maxDur)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", scriptPath,
			req.StorageBackend,
			fmt.Sprintf("%d", req.StorageSizeGB),
			req.StoragePool,
			req.NetworkBridge,
			logPath,
			req.InstallMethod,
		)
		// 独立进程组：超时时要连同 apt/dnf 等孙进程一起杀掉，否则包管理锁一直被占用
		prepareProcessGroup(cmd)
		// 丢弃标准输出（脚本自己写日志文件）
		cmd.Stdout = nil
		cmd.Stderr = nil
		err := cmd.Run()
		if ctx.Err() == context.DeadlineExceeded {
			killProcessGroup(cmd)
			log.Printf("安装脚本超时（%s），已终止", maxDur)
		} else if err != nil {
			log.Printf("安装脚本退出：%v", err)
		}
		// bash 不存在、脚本被信号杀死等情况下 EXIT trap 不会执行，日志里永远没有 [DONE]，
		// 前端会无限轮询「正在安装」。这里兜底补写结束标记（已有 [DONE] 时不覆盖脚本的真实结果）。
		ensureDoneMarker(logPath, err, ctx.Err() == context.DeadlineExceeded, maxDur)
		// 执行完后清理脚本文件（日志保留，便于失败后排查）
		_ = os.Remove(scriptPath)
	}()

	c.JSON(http.StatusOK, model.ApiResponse{
		Code: 200,
		Data: gin.H{
			"message":    "安装已启动",
			"log_file":   logPath,
			"started_at": time.Now().Format(time.RFC3339),
		},
	})
}

// InstallLog GET /api/server/install-log?file=<installBaseDir>/<run>/install.log
// 读取安装日志（支持 tail 模式，前端轮询用）
func (h *InstallHandler) InstallLog(c *gin.Context) {
	logFile := c.Query("file")
	if logFile == "" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "缺少 file 参数"})
		return
	}

	// 安全校验：只允许读取本程序创建的安装日志（专用目录 + 固定文件名），
	// 而不是整个 /tmp —— 面板以 root 运行，放宽到 /tmp 等于任意文件读取。
	clean := filepath.Clean(logFile)
	baseDir := filepath.Clean(installBaseDir())
	if !strings.HasPrefix(clean, baseDir+string(os.PathSeparator)) || filepath.Base(clean) != installLogName {
		c.JSON(http.StatusForbidden, model.ApiResponse{Code: 403, Error: "非法日志路径"})
		return
	}

	f, err := os.Open(clean)
	if err != nil {
		// 明确返回 done=false：前端据此继续轮询，而不是因为缺字段卡死
		c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{
			"lines":   []string{},
			"exists":  false,
			"done":    false,
			"success": false,
		}})
		return
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	// 默认单行上限 64KB：apt/dnf 的 \r 进度输出可能堆成长行，
	// 一旦超限扫描会中断，导致永远读不到 [DONE] 而让前端无限轮询。
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	// 只返回最近 200 行避免过大
	if len(lines) > 200 {
		lines = lines[len(lines)-200:]
	}

	// 判断安装是否完成：脚本用 trap 保证无论成败都会输出 [DONE] 行。
	// 从末尾往前找（而不是只看最后一行），避免收尾日志追加在标记之后。
	done := false
	success := false
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "[DONE]") {
			done = true
			success = strings.Contains(lines[i], "rc=0")
			break
		}
	}

	c.JSON(http.StatusOK, model.ApiResponse{
		Code: 200,
		Data: gin.H{
			"lines":   lines,
			"exists":  true,
			"done":    done,
			"success": success,
		},
	})
}
