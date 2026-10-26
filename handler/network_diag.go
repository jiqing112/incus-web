package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"incus-web/incus"
	"incus-web/model"

	"github.com/gin-gonic/gin"
)

// NetCheck 单项诊断结果
type NetCheck struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"` // ok | warn | fail | skip
	Detail  string `json:"detail"`
	Fixable bool   `json:"fixable"`
	FixDesc string `json:"fix_desc,omitempty"`
	FixID   string `json:"fix_id,omitempty"`
}

// NetworkDiagHandler 网络诊断与修复
type NetworkDiagHandler struct {
	client   *incus.Client
	imgCache *RemoteImageCache
}

func NewNetworkDiagHandler(client *incus.Client, imgCache *RemoteImageCache) *NetworkDiagHandler {
	return &NetworkDiagHandler{client: client, imgCache: imgCache}
}

// persistSysctl 写入持久化配置，返回错误供调用方如实汇报（此前忽略错误会误报修复成功）
func persistSysctl(content string) error {
	if err := os.MkdirAll("/etc/sysctl.d", 0755); err != nil {
		return err
	}
	return os.WriteFile("/etc/sysctl.d/99-incus-forward.conf", []byte(content), 0644)
}

func runCmd(timeout time.Duration, name string, args ...string) (string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			return string(out), -1
		}
	}
	return string(out), code
}

// RegisterRoutes 注册诊断与修复路由
func (h *NetworkDiagHandler) Diagnose(c *gin.Context) {
	if !isLinux() {
		c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: "网络诊断仅支持 Linux 宿主机"})
		return
	}
	checks := h.diagnose()

	fail, warn := 0, 0
	for _, ck := range checks {
		switch ck.Status {
		case "fail":
			fail++
		case "warn":
			warn++
		}
	}
	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{
		"checks":     checks,
		"fail":       fail,
		"warn":       warn,
		"checked_at": time.Now().Format("2006-01-02 15:04:05"),
	}})
}

type repairReq struct {
	FixID string `json:"fix_id"`
}

func (h *NetworkDiagHandler) Repair(c *gin.Context) {
	if !isLinux() {
		c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: "网络诊断仅支持 Linux 宿主机"})
		return
	}
	var req repairReq
	if err := c.ShouldBindJSON(&req); err != nil || req.FixID == "" {
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "fix_id is required"})
		return
	}

	var msg string
	switch req.FixID {
	case "fix_ip_forward":
		if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
			c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: "启用 IPv4 转发失败: " + err.Error()})
			return
		}
		msg = "已启用 IPv4 转发"
		if err := persistSysctl("net.ipv4.ip_forward=1\n"); err != nil {
			msg += "（持久化失败：" + err.Error() + "，重启后将失效）"
		} else {
			msg += "并写入 /etc/sysctl.d/99-incus-forward.conf 持久化"
		}
	case "fix_forward_policy":
		if out, code := runCmd(5*time.Second, "iptables", "-P", "FORWARD", "ACCEPT"); code != 0 {
			c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: "设置 FORWARD 策略失败: " + strings.TrimSpace(out)})
			return
		}
		msg = "已将 FORWARD 默认策略改为 ACCEPT（请确认防火墙由其它层管理）"
	case "fix_docker_user":
		bridges := h.managedBridges()
		if len(bridges) == 0 {
			c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: "未找到 Incus 托管网桥"})
			return
		}
		// 链可能不存在（Docker 未创建过），先建链并忽略「已存在」错误
		runCmd(5*time.Second, "iptables", "-N", "DOCKER-USER")
		wan := defaultIface()
		n := 0
		for _, b := range bridges {
			if dockerUserHasRule(b) {
				continue // 幂等：已放行则跳过，避免重复插入
			}
			runCmd(5*time.Second, "iptables", "-I", "DOCKER-USER", "-i", wan, "-o", b, "-j", "ACCEPT")
			runCmd(5*time.Second, "iptables", "-I", "DOCKER-USER", "-o", wan, "-i", b, "-j", "ACCEPT")
			n += 2
		}
		// 尽力持久化（存在 netfilter-persistent 时）
		runCmd(5*time.Second, "netfilter-persistent", "save")
		if n == 0 {
			msg = "DOCKER-USER 放行规则已存在，无需变更"
		} else {
			msg = fmt.Sprintf("已向 DOCKER-USER 链添加 %d 条放行规则（网桥: %s，外网口: %s）；重启 Docker 后规则仍在，如需跨机重启持久化请安装 iptables-persistent", n, strings.Join(bridges, ", "), wan)
		}
	case "fix_ipv6_forward":
		if err := os.WriteFile("/proc/sys/net/ipv6/conf/all/forwarding", []byte("1"), 0644); err != nil {
			c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: "启用 IPv6 转发失败: " + err.Error()})
			return
		}
		msg = "已启用 IPv6 转发"
		content := "net.ipv4.ip_forward=1\nnet.ipv6.conf.all.forwarding=1\n"
		if err := persistSysctl(content); err != nil {
			msg += "（持久化失败：" + err.Error() + "，重启后将失效）"
		} else {
			msg += "并更新 /etc/sysctl.d/99-incus-forward.conf 持久化"
		}
	case "fix_reload_incus":
		_, code := runCmd(60*time.Second, "systemctl", "restart", "incus")
		if code != 0 {
			_, code = runCmd(90*time.Second, "systemctl", "restart", "snap.incus.daemon") // snap 安装形态
		}
		if code != 0 {
			c.JSON(http.StatusOK, model.ApiResponse{Code: 500, Error: "重启 Incus 失败（incus / snap.incus.daemon 均失败）"})
			return
		}
		msg = "已重启 Incus 守护进程，网络规则（转发放行 / MASQUERADE）已重新生成；运行中的容器不受影响"
	default:
		c.JSON(http.StatusBadRequest, model.ApiResponse{Code: 400, Error: "未知修复动作: " + req.FixID})
		return
	}

	c.JSON(http.StatusOK, model.ApiResponse{Code: 200, Data: gin.H{"message": msg}})
}

/* ------------------------------ 诊断实现 ------------------------------ */

func (h *NetworkDiagHandler) diagnose() []NetCheck {
	checks := []NetCheck{}
	bridges := h.managedBridges()

	running := 0
	if instances, err := h.client.ListInstances("", "Running"); err == nil {
		running = len(instances)
	}

	checks = append(checks, checkIPForward())
	checks = append(checks, checkIPv6Forward())
	checks = append(checks, checkForwardPolicy())
	checks = append(checks, checkDockerConflict(bridges)...)
	for _, b := range bridges {
		checks = append(checks, checkBridgeUp(b, running))
		checks = append(checks, checkBridgeFwd(b))
		checks = append(checks, checkBridgeMasq(b))
	}
	checks = append(checks, checkHostEgress())
	checks = append(checks, h.checkContainerEgress())
	return checks
}

// checkIPv6Forward IPv6 转发：Incus 网桥默认启用 IPv6 NAT，转发关闭会断 IPv6 出网
func checkIPv6Forward() NetCheck {
	data, err := os.ReadFile("/proc/sys/net/ipv6/conf/all/forwarding")
	if err != nil {
		return NetCheck{ID: "ip6_forward", Title: "IPv6 转发", Status: "skip", Detail: "无法读取（可能未启用 IPv6）"}
	}
	v := strings.TrimSpace(string(data))
	if v == "1" {
		return NetCheck{ID: "ip6_forward", Title: "IPv6 转发已启用", Status: "ok", Detail: "net.ipv6.conf.all.forwarding = 1"}
	}
	return NetCheck{
		ID: "ip6_forward", Title: "IPv6 转发已关闭", Status: "warn",
		Detail:  "net.ipv6.conf.all.forwarding = 0：容器的 IPv6 出网（含 fd42:: 段 NAT）将不可用，IPv4 不受影响",
		Fixable: true, FixDesc: "sysctl -w net.ipv6.conf.all.forwarding=1 并写入 /etc/sysctl.d 持久化", FixID: "fix_ipv6_forward",
	}
}

// checkBridgeUp 网桥链路状态：有运行实例但网桥 DOWN 属于异常；全部停止时 DOWN 属正常
func checkBridgeUp(b string, running int) NetCheck {
	out, code := runCmd(3*time.Second, "ip", "link", "show", "dev", b)
	id := "bridge_up_" + b
	if code != 0 {
		return NetCheck{ID: id, Title: "网桥存在性（" + b + "）", Status: "fail", Detail: "网桥接口不存在（可能被删除）"}
	}
	if strings.Contains(out, "state UP") || strings.Contains(out, " UP ") {
		return NetCheck{ID: id, Title: "网桥状态（" + b + "）", Status: "ok", Detail: "UP"}
	}
	if running == 0 {
		return NetCheck{ID: id, Title: "网桥状态（" + b + "）", Status: "ok", Detail: "DOWN（无运行中的实例挂载，属正常）"}
	}
	return NetCheck{
		ID: id, Title: "网桥状态（" + b + "）", Status: "warn",
		Detail: fmt.Sprintf("网桥处于 DOWN，但当前有 %d 个运行中的实例：其网卡可能未正确挂载到网桥", running),
	}
}

func (h *NetworkDiagHandler) managedBridges() []string {
	nets, err := h.client.ListNetworks()
	if err != nil {
		return nil
	}
	var out []string
	for _, n := range nets {
		if n.Managed && n.Type == "bridge" {
			out = append(out, n.Name)
		}
	}
	return out
}

func checkIPForward() NetCheck {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return NetCheck{ID: "ip_forward", Title: "IPv4 转发已启用", Status: "warn", Detail: "无法读取 /proc/sys/net/ipv4/ip_forward: " + err.Error()}
	}
	v := strings.TrimSpace(string(data))
	if v == "1" {
		return NetCheck{ID: "ip_forward", Title: "IPv4 转发已启用", Status: "ok", Detail: "net.ipv4.ip_forward = 1"}
	}
	return NetCheck{
		ID: "ip_forward", Title: "IPv4 转发已关闭", Status: "fail",
		Detail:  "net.ipv4.ip_forward = 0：容器将无法 NAT 出网，外部转发也会失败",
		Fixable: true, FixDesc: "sysctl -w net.ipv4.ip_forward=1 并写入 /etc/sysctl.d 持久化", FixID: "fix_ip_forward",
	}
}

func checkForwardPolicy() NetCheck {
	out, code := runCmd(5*time.Second, "iptables", "-S", "FORWARD")
	if code != 0 {
		return NetCheck{ID: "forward_policy", Title: "FORWARD 链策略", Status: "warn", Detail: "读取 iptables 失败：" + strings.TrimSpace(out)}
	}
	if strings.HasPrefix(out, "-P FORWARD DROP") {
		return NetCheck{
			ID: "forward_policy", Title: "FORWARD 默认策略为 DROP", Status: "fail",
			Detail:  "所有未被显式放行的转发流量都会被丢弃——这是「Docker 等软件导致容器无法被外部访问」的最常见原因",
			Fixable: true, FixDesc: "iptables -P FORWARD ACCEPT（全局生效，请确认防火墙由其它层管理）", FixID: "fix_forward_policy",
		}
	}
	return NetCheck{ID: "forward_policy", Title: "FORWARD 链策略", Status: "ok", Detail: strings.Split(out, "\n")[0]}
}

// dockerUserHasRule 检查 DOCKER-USER 链中是否存在以指定网桥为输出接口的放行规则。
// 按整字段匹配，避免 incusbr0 误匹配 incusbr01 的规则。
func dockerUserHasRule(bridge string) bool {
	out, code := runCmd(5*time.Second, "iptables", "-S", "DOCKER-USER")
	if code != 0 {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "-o" && i+1 < len(fields) && fields[i+1] == bridge {
				for _, ff := range fields {
					if ff == "ACCEPT" {
						return true
					}
				}
			}
		}
	}
	return false
}

func checkDockerConflict(bridges []string) []NetCheck {
	_, code := runCmd(3*time.Second, "ip", "link", "show", "docker0")
	if code != 0 {
		return []NetCheck{{ID: "docker_conflict", Title: "Docker 转发链冲突", Status: "ok", Detail: "未安装 Docker（无 docker0 网桥），无转发链冲突风险"}}
	}

	checks := []NetCheck{{ID: "docker_conflict", Title: "检测到 Docker", Status: "warn", Detail: "Docker 会接管 iptables FORWARD 链，可能丢弃 Incus 网桥的转发流量"}}

	// DOCKER-USER 放行检查
	if _, code := runCmd(5*time.Second, "iptables", "-S", "DOCKER-USER"); code != 0 {
		checks = append(checks, NetCheck{
			ID: "docker_user", Title: "缺少 DOCKER-USER 链", Status: "fail",
			Detail:  "Docker 已接管 iptables 但未找到 DOCKER-USER 链，Incus 网桥流量将被 FORWARD 策略丢弃",
			Fixable: true, FixDesc: "创建 DOCKER-USER 链并放行 Incus 网桥与外网口之间的转发", FixID: "fix_docker_user",
		})
		return checks
	}
	var missing []string
	for _, b := range bridges {
		if !dockerUserHasRule(b) {
			missing = append(missing, b)
		}
	}
	if len(missing) > 0 {
		checks = append(checks, NetCheck{
			ID: "docker_user", Title: "DOCKER-USER 未放行 Incus 网桥", Status: "fail",
			Detail:  "以下网桥的转发流量会被 Docker 规则丢弃：" + strings.Join(missing, ", "),
			Fixable: true, FixDesc: "向 DOCKER-USER 链添加外网口与网桥间的放行规则（并尽力持久化）", FixID: "fix_docker_user",
		})
	} else {
		checks = append(checks, NetCheck{ID: "docker_user", Title: "DOCKER-USER 已放行 Incus 网桥", Status: "ok", Detail: "各托管网桥的转发均已被显式放行"})
	}
	return checks
}

func checkBridgeFwd(b string) NetCheck {
	out, code := runCmd(5*time.Second, "nft", "list", "table", "inet", "incus")
	id := "fwd_" + b
	if code != 0 {
		return NetCheck{ID: id, Title: "网桥转发放行（" + b + "）", Status: "warn", Detail: "读取 nftables 失败：" + strings.TrimSpace(out)}
	}
	if strings.Contains(out, "fwd."+b) && strings.Contains(out, `oifname "`+b+`" accept`) {
		return NetCheck{ID: id, Title: "网桥转发放行（" + b + "）", Status: "ok", Detail: "nftables fwd." + b + " 双向 accept 已存在"}
	}
	return NetCheck{
		ID: id, Title: "网桥转发放行缺失（" + b + "）", Status: "warn",
		Detail:  "nftables 中未找到 fwd." + b + " 放行规则（可能被防火墙重载清除），外部访问该网桥容器可能失败",
		Fixable: true, FixDesc: "重启 Incus 守护进程以重新生成网桥转发规则（容器不受影响，proxy 转发进程会短暂重启）", FixID: "fix_reload_incus",
	}
}

func checkBridgeMasq(b string) NetCheck {
	out, code := runCmd(5*time.Second, "nft", "list", "table", "inet", "incus")
	id := "masq_" + b
	if code != 0 {
		return NetCheck{ID: id, Title: "出站 MASQUERADE（" + b + "）", Status: "warn", Detail: "读取 nftables 失败"}
	}
	if strings.Contains(out, "pstrt."+b) && strings.Contains(out, "masquerade") {
		return NetCheck{ID: id, Title: "出站 MASQUERADE（" + b + "）", Status: "ok", Detail: "nftables pstrt." + b + " 伪装规则已存在"}
	}
	// iptables 侧兜底检查
	iout, _ := runCmd(5*time.Second, "iptables", "-t", "nat", "-S", "POSTROUTING")
	if strings.Contains(iout, "MASQUERADE") {
		return NetCheck{ID: id, Title: "出站 MASQUERADE（" + b + "）", Status: "ok", Detail: "iptables nat 表存在 MASQUERADE 规则"}
	}
	return NetCheck{
		ID: id, Title: "出站 MASQUERADE 缺失（" + b + "）", Status: "fail",
		Detail:  "未找到 " + b + " 的出站伪装规则，容器将无法访问外网",
		Fixable: true, FixDesc: "重启 Incus 守护进程以重新生成 MASQUERADE 规则", FixID: "fix_reload_incus",
	}
}

func checkHostEgress() NetCheck {
	for _, target := range []string{"223.5.5.5", "114.114.114.114", "1.1.1.1"} {
		if _, code := runCmd(4*time.Second, "ping", "-c", "1", "-W", "2", target); code == 0 {
			return NetCheck{ID: "host_egress", Title: "宿主机出外网", Status: "ok", Detail: "ping " + target + " 通"}
		}
	}
	return NetCheck{
		ID: "host_egress", Title: "宿主机无法访问外网", Status: "warn",
		Detail: "ping 223.5.5.5 / 114.114.114.114 / 1.1.1.1 均失败：宿主机自身无外网，容器自然也无法出网（此为上游网络问题，非 Incus 配置问题）",
	}
}

func (h *NetworkDiagHandler) checkContainerEgress() NetCheck {
	instances, err := h.client.ListInstances("", "Running")
	if err != nil || len(instances) == 0 {
		return NetCheck{ID: "container_egress", Title: "容器出外网实测", Status: "skip", Detail: "没有运行中的实例，跳过实测（启动实例后可再次诊断）"}
	}
	name := instances[0].Name
	_, code := runCmd(8*time.Second, "incus", "exec", name, "--", "ping", "-c", "1", "-W", "2", "223.5.5.5")
	if code == 0 {
		return NetCheck{ID: "container_egress", Title: "容器出外网实测", Status: "ok", Detail: "实例 " + name + " 内 ping 223.5.5.5 通"}
	}
	return NetCheck{
		ID: "container_egress", Title: "容器无法访问外网", Status: "fail",
		Detail: "实例 " + name + " 内 ping 失败：结合上方 IPv4 转发 / MASQUERADE / 转发放行各项定位原因",
	}
}

/* ------------------------------ 工具 ------------------------------ */

func defaultIface() string {
	out, _ := runCmd(3*time.Second, "ip", "route", "show", "default")
	fields := strings.Fields(out)
	for i, ff := range fields {
		if ff == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return "eth0"
}

func isLinux() bool {
	return fileExists("/proc/self/status")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
