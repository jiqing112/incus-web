package model

// ApiResponse 统一响应格式
type ApiResponse struct {
	Code  int         `json:"code"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// Operation 异步操作状态
type Operation struct {
	ID          string `json:"operation_id"`
	StatusURL   string `json:"status_url"`
	Status      string `json:"status"` // pending, running, success, failure
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// CreateInstanceRequest 创建实例请求
// ImageSource 为镜像来源：remote（远程源拉取，默认）或 local（引用本地已有镜像）。
// local 时以 ImageFingerprint 为准（为空则退回 ImageAlias 匹配本地别名）。
type CreateInstanceRequest struct {
	Name string `json:"name"`
	Type string `json:"type"` // container 或 virtual-machine
	// Architecture 为空时由 Incus 按宿主机架构决定（不再硬编码 x86_64，以支持 ARM 宿主机）
	Architecture     string   `json:"architecture"`
	ImageSource      string   `json:"image_source"`
	ImageAlias       string   `json:"image_alias"`
	ImageFingerprint string   `json:"image_fingerprint"`
	ImageServer      string   `json:"image_server"`
	ImageProtocol    string   `json:"image_protocol"`
	CPULimit         string   `json:"cpu_limit"`
	MemoryLimit      string   `json:"memory_limit"`
	DiskSize         string   `json:"disk_size"`
	StoragePool      string   `json:"storage_pool"`
	Network          string   `json:"network"`
	Profiles         []string `json:"profiles"`
}

// StateRequest 实例状态变更请求
type StateRequest struct {
	Action  string `json:"action"` // start, stop, restart
	Timeout int    `json:"timeout"`
	Force   bool   `json:"force"`
}

// UpdateInstanceConfigRequest 更新实例配置请求
// Config 仅包含需要改动的键（值为空串表示删除该键）；
// Devices 为可选，提供时表示期望的完整设备集合（含删除，故走全量 PUT）；
// Description 为实例顶层描述字段（非 config 键），指针区分「未提供」与「清空」。
type UpdateInstanceConfigRequest struct {
	Config      map[string]string            `json:"config"`
	Devices     map[string]map[string]string `json:"devices"`
	Description *string                      `json:"description"`
}

// InstanceInfo 实例信息
type InstanceInfo struct {
	Name         string                       `json:"name"`
	Status       string                       `json:"status"`
	Type         string                       `json:"type"`
	Description  string                       `json:"description"`
	Profiles     []string                     `json:"profiles"`
	Config       map[string]string            `json:"config"`
	Devices      map[string]map[string]string `json:"devices"`
	State        *InstanceState               `json:"state,omitempty"`
	CreatedAt    string                       `json:"created_at"`
	LastUsedAt   string                       `json:"last_used_at"`
	Architecture string                       `json:"architecture"`
	Ephemeral    bool                         `json:"ephemeral"`
}

// InstanceState 实例运行时状态
type InstanceState struct {
	CPU       InstanceCPUInfo                `json:"cpu"`
	Memory    InstanceMemoryInfo             `json:"memory"`
	Network   map[string]InstanceNetworkInfo `json:"network"`
	Disk      map[string]InstanceDiskInfo    `json:"disk"`
	Processes int                            `json:"processes"`
}

// InstanceCPUInfo CPU 信息
type InstanceCPUInfo struct {
	Usage    int64  `json:"usage"`
	UsageStr string `json:"usage_str"`
}

// InstanceMemoryInfo 内存信息
// Usage 为当前占用；UsagePeak 为历史峰值；Total 为 limits.memory 解析出的上限（未限制时为 0）。
type InstanceMemoryInfo struct {
	Usage     int64  `json:"usage"`
	UsagePeak int64  `json:"usage_peak"`
	UsageStr  string `json:"usage_str"`
	Total     int64  `json:"total"`
}

// InstanceNetworkInfo 网络信息
type InstanceNetworkInfo struct {
	Addresses []InstanceNetworkAddress `json:"addresses"`
	Counters  InstanceNetworkCounters  `json:"counters"`
}

// InstanceNetworkAddress 网络地址
type InstanceNetworkAddress struct {
	Family  string `json:"family"`
	Address string `json:"address"`
	Scope   string `json:"scope"`
}

// InstanceNetworkCounters 网络计数器
type InstanceNetworkCounters struct {
	BytesReceived   int64 `json:"bytes_received"`
	BytesSent       int64 `json:"bytes_sent"`
	PacketsReceived int64 `json:"packets_received"`
	PacketsSent     int64 `json:"packets_sent"`
}

// InstanceDiskInfo 磁盘信息
type InstanceDiskInfo struct {
	Usage    int64  `json:"usage"`
	UsageStr string `json:"usage_str"`
}

// ImageInfo 镜像信息
type ImageInfo struct {
	Fingerprint  string            `json:"fingerprint"`
	Filename     string            `json:"filename"`
	Size         int64             `json:"size"`
	Architecture string            `json:"architecture"`
	Type         string            `json:"type"`
	CreatedAt    string            `json:"created_at"`
	UploadedAt   string            `json:"uploaded_at"`
	ExpiresAt    string            `json:"expires_at"`
	Aliases      []ImageAlias      `json:"aliases"`
	Properties   map[string]string `json:"properties"`
	Public       bool              `json:"public"`
	Cached       bool              `json:"cached"`
	AutoUpdate   bool              `json:"auto_update"`
}

// ImageAlias 镜像别名
type ImageAlias struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RemoteImageInfo 远程镜像信息
type RemoteImageInfo struct {
	Alias        string `json:"alias"`
	Fingerprint  string `json:"fingerprint"`
	Architecture string `json:"architecture"`
	Description  string `json:"description"`
	OS           string `json:"os"`
	Release      string `json:"release"`
	Variant      string `json:"variant"`
	Size         int64  `json:"size"`
}

// ImportImageRequest 导入镜像请求
type ImportImageRequest struct {
	Server string `json:"server"`
	Alias  string `json:"alias"`
}

// ExportImageRequest 导出镜像请求
type ExportImageRequest struct {
	Fingerprint string `json:"fingerprint"`
	Format      string `json:"format"` // tar 或 tar.gz
}

// RefreshImageRequest 刷新远程镜像请求
type RefreshImageRequest struct {
	Server string `json:"server"`
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	Name      string `json:"name"`
	Stateful  bool   `json:"stateful"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

// CreateSnapshotRequest 创建快照请求
type CreateSnapshotRequest struct {
	Name     string `json:"name"`
	Stateful bool   `json:"stateful"`
	// ExpiresAt 为可选自动过期时间（RFC3339），到点后 Incus 自动删除该快照
	ExpiresAt string `json:"expires_at"`
}

// RenameSnapshotRequest 重命名快照请求
type RenameSnapshotRequest struct {
	Name string `json:"name"`
}

// RestoreSnapshotRequest 恢复快照请求
type RestoreSnapshotRequest struct {
	// Stateful 为 true 且目标快照本身是 stateful 时，连同运行时内存状态一起恢复
	Stateful bool `json:"stateful"`
}

// BackupInfo 备份信息
type BackupInfo struct {
	Name             string `json:"name"`
	CreatedAt        string `json:"created_at"`
	ExpiresAt        string `json:"expires_at"`
	InstanceOnly     bool   `json:"instance_only"`
	OptimizedStorage bool   `json:"optimized_storage"`
	ContainerOnly    bool   `json:"container_only"`
}

// CreateBackupRequest 创建备份请求
type CreateBackupRequest struct {
	Name             string `json:"name"`
	InstanceOnly     bool   `json:"instance_only"`
	OptimizedStorage bool   `json:"optimized_storage"`
	ExpiresAt        string `json:"expires_at"`
}

// ExportInstanceRequest 导出实例请求
type ExportInstanceRequest struct {
	Name string `json:"name"` // 镜像名称
}

// StoragePoolInfo 存储池信息
type StoragePoolInfo struct {
	Name      string            `json:"name"`
	Driver    string            `json:"driver"`
	Config    map[string]string `json:"config"`
	Status    string            `json:"status"`
	UsedBy    []string          `json:"used_by"`
	Locations []string          `json:"locations"`
}

// CreateStoragePoolRequest 创建存储池请求
type CreateStoragePoolRequest struct {
	Name   string            `json:"name"`
	Driver string            `json:"driver"`
	Config map[string]string `json:"config"`
}

// StorageVolumeInfo 存储卷信息
type StorageVolumeInfo struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Config   map[string]string `json:"config"`
	UsedBy   []string          `json:"used_by"`
	Location string            `json:"location"`
}

// CreateStorageVolumeRequest 创建存储卷请求
type CreateStorageVolumeRequest struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Config map[string]string `json:"config"`
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Managed     bool              `json:"managed"`
	Status      string            `json:"status"`
	Config      map[string]string `json:"config"`
	UsedBy      []string          `json:"used_by"`
	Locations   []string          `json:"locations"`
	Leases      []NetworkLease    `json:"leases,omitempty"`
}

// NetworkLease DHCP 租约
type NetworkLease struct {
	Hostname string `json:"hostname"`
	Hwaddr   string `json:"hwaddr"`
	Address  string `json:"address"`
	Type     string `json:"type"`
}

// CreateNetworkRequest 创建网络请求
type CreateNetworkRequest struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Config map[string]string `json:"config"`
}

// UpdateNetworkRequest 更新网络请求
type UpdateNetworkRequest struct {
	Config      map[string]string `json:"config"`
	Description string            `json:"description"`
}

// ProfileInfo 配置文件信息
type ProfileInfo struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Config      map[string]string            `json:"config"`
	Devices     map[string]map[string]string `json:"devices"`
	UsedBy      []string                     `json:"used_by"`
}

// CreateProfileRequest 创建配置文件请求
type CreateProfileRequest struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Config      map[string]string            `json:"config"`
	Devices     map[string]map[string]string `json:"devices"`
}

// UpdateProfileRequest 更新配置文件请求
type UpdateProfileRequest struct {
	Description string                       `json:"description"`
	Config      map[string]string            `json:"config"`
	Devices     map[string]map[string]string `json:"devices"`
}

// ServerInfo 服务器信息
type ServerInfo struct {
	APIStatus   string                 `json:"api_status"`
	APIVersion  string                 `json:"api_version"`
	Environment ServerEnvironment      `json:"environment"`
	Config      map[string]interface{} `json:"config"`
}

// ServerEnvironment 服务器环境
type ServerEnvironment struct {
	Kernel             string `json:"kernel"`
	KernelArchitecture string `json:"kernel_architecture"`
	KernelVersion      string `json:"kernel_version"`
	OSName             string `json:"os_name"`
	OSVersion          string `json:"os_version"`
	Project            string `json:"project"`
	Server             string `json:"server"`
	ServerVersion      string `json:"server_version"`
	Driver             string `json:"driver"`
	LTS                bool   `json:"lts"`
}

// ServerResources 服务器资源
type ServerResources struct {
	CPU     ServerCPUInfo    `json:"cpu"`
	Memory  ServerMemoryInfo `json:"memory"`
	Disk    ServerDiskInfo   `json:"disk"`
	GPU     *ServerGPUInfo   `json:"gpu,omitempty"`
	Network *ServerNetInfo   `json:"network,omitempty"`
}

// ServerCPUInfo 服务器 CPU 信息
type ServerCPUInfo struct {
	Architecture string `json:"architecture"`
	SocketCount  int    `json:"socket_count"`
	TotalCores   int    `json:"total_cores"`
	Usage        int64  `json:"usage"`
}

// ServerMemoryInfo 服务器内存信息
type ServerMemoryInfo struct {
	Total    int64  `json:"total"`
	Used     int64  `json:"used"`
	UsageStr string `json:"usage_str"`
}

// ServerDiskInfo 服务器磁盘信息
type ServerDiskInfo struct {
	Total    int64  `json:"total"`
	Used     int64  `json:"used"`
	UsageStr string `json:"usage_str"`
}

// ServerGPUInfo 服务器 GPU 信息
type ServerGPUInfo struct {
	Count int `json:"count"`
}

// ServerNetInfo 服务器网络信息
type ServerNetInfo struct {
	InterfaceCount int `json:"interface_count"`
}

// FileInfo 文件/目录条目
type FileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"` // file 或 directory
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
	Modified string `json:"modified"`
}

// OperationStatus 操作详情状态
type OperationStatus struct {
	ID          string                 `json:"id"`
	Status      string                 `json:"status"`
	StatusCode  int                    `json:"status_code"`
	Description string                 `json:"description"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
	MayCancel   bool                   `json:"may_cancel"`
	Err         string                 `json:"err"`
	Metadata    map[string]interface{} `json:"metadata"`
	Resources   map[string][]string    `json:"resources"`
}

// EventMessage SSE 事件消息
type EventMessage struct {
	Type      string      `json:"type"`   // operation, instance, event
	Status    string      `json:"status"` // started, success, failure, updated
	Payload   interface{} `json:"payload"`
	Timestamp string      `json:"timestamp"`
}
