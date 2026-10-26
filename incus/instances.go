package incus

import (
	"fmt"
	"strconv"
	"strings"

	"incus-web/model"
)

// IncusInstanceResponse Incus 返回的实例列表响应
type IncusInstanceResponse struct {
	Type       string   `json:"type"`
	Status     string   `json:"status"`
	StatusCode int      `json:"status_code"`
	Metadata   []string `json:"metadata"`
}

// IncusFullInstanceResponse 完整实例列表响应（带详情）
type IncusFullInstanceResponse struct {
	Type       string                `json:"type"`
	Status     string                `json:"status"`
	StatusCode int                   `json:"status_code"`
	Metadata   []IncusInstanceRecord `json:"metadata"`
}

// IncusInstanceRecord 实例记录
type IncusInstanceRecord struct {
	Name         string                       `json:"name"`
	Status       string                       `json:"status"`
	Type         string                       `json:"type"`
	Profiles     []string                     `json:"profiles"`
	Config       map[string]string            `json:"config"`
	Devices      map[string]map[string]string `json:"devices"`
	Architecture string                       `json:"architecture"`
	Ephemeral    bool                         `json:"ephemeral"`
	CreatedAt    string                       `json:"created_at"`
	LastUsedAt   string                       `json:"last_used_at"`
	Stateful     bool                         `json:"stateful"`
	Description  string                       `json:"description"`
	// State 仅在 recursion=2 的列表响应中出现（运行中实例含网络地址等运行时信息）
	State *IncusInstanceState `json:"state,omitempty"`
}

// IncusInstanceState 实例运行时状态。
// `/instances/{name}/state` 的 metadata 与 `/instances?recursion=2` 各条目的 state 字段同构。
type IncusInstanceState struct {
	CPU struct {
		Usage int64 `json:"usage"`
	} `json:"cpu"`
	Memory struct {
		Usage     int64 `json:"usage"`
		UsagePeak int64 `json:"usage_peak"`
	} `json:"memory"`
	Network map[string]struct {
		Addresses []model.InstanceNetworkAddress `json:"addresses"`
		Counters  model.InstanceNetworkCounters  `json:"counters"`
	} `json:"network"`
	Disk map[string]struct {
		Usage int64 `json:"usage"`
	} `json:"disk"`
	Processes int `json:"processes"`
}

// toModel 转换为对外模型。
// memLimit 为实例 limits.memory 解析出的字节上限（0 表示未限制），
// 不能把 usage_peak（历史峰值）当成「总内存」，否则占用率会恒接近 100%。
func (s *IncusInstanceState) toModel(memLimit int64) *model.InstanceState {
	if s == nil {
		return nil
	}
	state := &model.InstanceState{
		Processes: s.Processes,
		CPU:       model.InstanceCPUInfo{Usage: s.CPU.Usage},
		Memory: model.InstanceMemoryInfo{
			Usage:     s.Memory.Usage,
			UsagePeak: s.Memory.UsagePeak,
			Total:     memLimit,
		},
		Network: make(map[string]model.InstanceNetworkInfo, len(s.Network)),
		Disk:    make(map[string]model.InstanceDiskInfo, len(s.Disk)),
	}
	for iface, net := range s.Network {
		state.Network[iface] = model.InstanceNetworkInfo{
			Addresses: net.Addresses,
			Counters:  net.Counters,
		}
	}
	for dev, d := range s.Disk {
		state.Disk[dev] = model.InstanceDiskInfo{Usage: d.Usage}
	}
	return state
}

// IncusStateResponse 实例状态响应
type IncusStateResponse struct {
	Type       string             `json:"type"`
	Status     string             `json:"status"`
	StatusCode int                `json:"status_code"`
	Metadata   IncusInstanceState `json:"metadata"`
}

// IncusOperationResponse Incus 操作响应
type IncusOperationResponse struct {
	Type       string `json:"type"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	Operation  string `json:"operation"`
	Metadata   struct {
		ID          string `json:"id"`
		Status      string `json:"status"`
		StatusCode  int    `json:"status_code"`
		Description string `json:"description"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	} `json:"metadata"`
}

// ListInstances 列出所有实例
func (c *Client) ListInstances(instanceType, status string) ([]model.InstanceInfo, error) {
	// 先获取实例名称列表
	path := "/instances?recursion=2"
	if instanceType != "" {
		path = "/instances?recursion=2&type=" + instanceType
	}

	var fullResp IncusFullInstanceResponse
	if err := c.Get(path, &fullResp); err != nil {
		return nil, err
	}

	instances := []model.InstanceInfo{}
	for _, rec := range fullResp.Metadata {
		if status != "" && rec.Status != status {
			continue
		}
		inst := model.InstanceInfo{
			Name:         rec.Name,
			Status:       rec.Status,
			Type:         rec.Type,
			Description:  rec.Description,
			Profiles:     rec.Profiles,
			Config:       rec.Config,
			Devices:      rec.Devices,
			Architecture: rec.Architecture,
			Ephemeral:    rec.Ephemeral,
			CreatedAt:    rec.CreatedAt,
			LastUsedAt:   rec.LastUsedAt,
			State:        rec.State.toModel(parseMemoryLimit(rec.Config["limits.memory"])),
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

// GetInstance 获取单个实例详情
func (c *Client) GetInstance(name string) (*model.InstanceInfo, error) {
	type wrapper struct {
		Type       string              `json:"type"`
		Status     string              `json:"status"`
		StatusCode int                 `json:"status_code"`
		Metadata   IncusInstanceRecord `json:"metadata"`
	}
	var w wrapper
	if err := c.Get("/instances/"+name+"?recursion=1", &w); err != nil {
		return nil, err
	}
	rec := w.Metadata

	inst := &model.InstanceInfo{
		Name:         rec.Name,
		Status:       rec.Status,
		Type:         rec.Type,
		Description:  rec.Description,
		Profiles:     rec.Profiles,
		Config:       rec.Config,
		Devices:      rec.Devices,
		Architecture: rec.Architecture,
		Ephemeral:    rec.Ephemeral,
		CreatedAt:    rec.CreatedAt,
		LastUsedAt:   rec.LastUsedAt,
		State:        rec.State.toModel(parseMemoryLimit(rec.Config["limits.memory"])),
	}

	// 如果实例正在运行，获取运行时状态（recursion=1 的详情不携带 state）
	if inst.Status == "Running" {
		state, err := c.GetInstanceState(name)
		if err == nil {
			state.Memory.Total = parseMemoryLimit(rec.Config["limits.memory"])
			inst.State = state
		}
	}

	return inst, nil
}

// GetInstanceState 获取实例运行时状态
func (c *Client) GetInstanceState(name string) (*model.InstanceState, error) {
	var resp IncusStateResponse
	if err := c.Get("/instances/"+name+"/state", &resp); err != nil {
		return nil, err
	}

	state := resp.Metadata.toModel(0)
	if state == nil {
		state = &model.InstanceState{}
	}
	return state, nil
}

// parseMemoryLimit 把 Incus 的 limits.memory 取值解析为字节数。
// 支持 "2GB"/"512MiB"/"1.5GiB"/纯字节数；百分比（如 "50%"）与无法识别的值返回 0（视为未限制）。
func parseMemoryLimit(v string) int64 {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasSuffix(v, "%") {
		return 0
	}
	i := 0
	for i < len(v) && ((v[i] >= '0' && v[i] <= '9') || v[i] == '.') {
		i++
	}
	if i == 0 {
		return 0
	}
	num, err := strconv.ParseFloat(v[:i], 64)
	if err != nil || num <= 0 {
		return 0
	}
	var mul float64
	switch strings.ToLower(strings.TrimSpace(v[i:])) {
	case "", "b":
		mul = 1
	case "kb":
		mul = 1e3
	case "mb":
		mul = 1e6
	case "gb":
		mul = 1e9
	case "tb":
		mul = 1e12
	case "kib":
		mul = 1024
	case "mib":
		mul = 1024 * 1024
	case "gib":
		mul = 1024 * 1024 * 1024
	case "tib":
		mul = 1024 * 1024 * 1024 * 1024
	default:
		return 0
	}
	return int64(num * mul)
}

// CreateInstance 创建实例（异步），返回操作 URL
//
// 镜像来源支持两种：
//   - remote：从远程 simplestreams 源拉取（需要 server/protocol）
//   - local：直接引用本地镜像库中已有的镜像（按 fingerprint，或退回本地别名），
//     此时必须省略 server/protocol，否则 Incus 会当作远程源解析。
func (c *Client) CreateInstance(req model.CreateInstanceRequest) (string, error) {
	var source map[string]interface{}
	if req.ImageSource == "local" {
		source = map[string]interface{}{"type": "image"}
		if req.ImageFingerprint != "" {
			source["fingerprint"] = req.ImageFingerprint
		} else {
			source["alias"] = req.ImageAlias
		}
	} else {
		source = map[string]interface{}{
			"type":     "image",
			"alias":    req.ImageAlias,
			"server":   req.ImageServer,
			"protocol": req.ImageProtocol,
		}
	}

	incusReq := map[string]interface{}{
		"name":     req.Name,
		"profiles": req.Profiles,
		"type":     req.Type,
		"source":   source,
		"config":   map[string]string{},
		"devices":  map[string]map[string]string{},
	}
	// 架构留空时交给 Incus 按宿主机/镜像决定，避免在 ARM 宿主机上写死 x86_64
	if req.Architecture != "" {
		incusReq["architecture"] = req.Architecture
	}

	if len(req.Profiles) == 0 {
		incusReq["profiles"] = []string{"default"}
	}

	if req.CPULimit != "" {
		incusReq["config"].(map[string]string)["limits.cpu"] = req.CPULimit
	}
	if req.MemoryLimit != "" {
		incusReq["config"].(map[string]string)["limits.memory"] = req.MemoryLimit
	}

	pool := req.StoragePool
	if pool == "" {
		pool = "default"
	}
	rootDev := map[string]string{
		"type": "disk",
		"path": "/",
		"pool": pool,
	}
	if req.DiskSize != "" {
		rootDev["size"] = req.DiskSize
	}
	incusReq["devices"].(map[string]map[string]string)["root"] = rootDev

	if req.Network != "" {
		incusReq["devices"].(map[string]map[string]string)["eth0"] = map[string]string{
			"type":    "nic",
			"nictype": "bridged",
			"parent":  req.Network,
		}
	}

	var opResp IncusOperationResponse
	if err := c.Post("/instances", incusReq, &opResp); err != nil {
		return "", err
	}

	return opResp.Operation, nil
}

// UpdateInstanceConfig 更新实例配置。
// 仅改 config 时用 PATCH（同步，空串删除键）；包含设备变更时改用 PUT 全量替换
// （Incus 的 PATCH 无法删除设备），PUT 为异步操作，返回操作 URL。
func (c *Client) UpdateInstanceConfig(name string, req model.UpdateInstanceConfigRequest) (string, error) {
	// 实测：Incus 的 PATCH 会静默忽略顶层 description，因此 description 变更
	// 必须走 PUT 全量分支；仅 config 变更时 PATCH 同步生效。
	if req.Devices == nil && req.Description == nil {
		body := map[string]interface{}{"config": req.Config}
		if err := c.Patch("/instances/"+name, body, nil); err != nil {
			return "", err
		}
		return "", nil
	}

	// 读取当前定义，合并 config 后再全量提交，避免覆盖未改动的键与 volatile 数据
	type wrapper struct {
		Metadata IncusInstanceRecord `json:"metadata"`
	}
	var w wrapper
	if err := c.Get("/instances/"+name+"?recursion=1", &w); err != nil {
		return "", err
	}
	cur := w.Metadata

	mergedConfig := make(map[string]string, len(cur.Config))
	for k, v := range cur.Config {
		mergedConfig[k] = v
	}
	for k, v := range req.Config {
		if v == "" {
			delete(mergedConfig, k)
		} else {
			mergedConfig[k] = v
		}
	}

	// 仅改 description 时 devices 未提供，回退为当前设备集合，避免 PUT null 清空设备
	devices := req.Devices
	if devices == nil {
		devices = cur.Devices
	}

	body := map[string]interface{}{
		"config":       mergedConfig,
		"devices":      devices,
		"profiles":     cur.Profiles,
		"type":         cur.Type,
		"architecture": cur.Architecture,
		"ephemeral":    cur.Ephemeral,
		"stateful":     cur.Stateful,
	}
	if req.Description != nil {
		body["description"] = *req.Description
	} else {
		body["description"] = cur.Description
	}

	var opResp IncusOperationResponse
	if err := c.Put("/instances/"+name, body, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}

// DeleteInstance 删除实例
func (c *Client) DeleteInstance(name string, force bool) (string, error) {
	path := "/instances/" + name
	if force {
		path += "?force=true"
	}

	var opResp IncusOperationResponse
	if err := c.Delete(path, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}

// ChangeInstanceState 变更实例状态（启动/停止/重启）
func (c *Client) ChangeInstanceState(name string, req model.StateRequest) (string, error) {
	incusReq := map[string]interface{}{
		"action":  req.Action,
		"timeout": 30,
		"force":   req.Force,
	}
	if req.Timeout > 0 {
		incusReq["timeout"] = req.Timeout
	}

	var opResp IncusOperationResponse
	if err := c.Put("/instances/"+name+"/state", incusReq, &opResp); err != nil {
		return "", err
	}
	return opResp.Operation, nil
}

// FormatSize 将字节数格式化为可读字符串
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
