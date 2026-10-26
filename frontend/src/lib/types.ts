/** 与 Go model/types.go 对齐的前端类型定义。 */

export interface ApiResponse<T = unknown> {
  code: number;
  data?: T;
  error?: string;
}

export interface Operation {
  operation_id: string;
  status_url?: string;
  status?: string;
  description?: string;
  created_at?: string;
}

export interface CreateInstanceRequest {
  name: string;
  type: string;
  /** 镜像来源：'local' 引用本地镜像；其它/缺省为远程源拉取 */
  image_source?: 'local' | 'remote';
  image_alias: string;
  /** 本地镜像指纹（image_source === 'local' 时优先使用） */
  image_fingerprint?: string;
  image_server: string;
  image_protocol: string;
  cpu_limit: string;
  memory_limit: string;
  disk_size: string;
  /** 存储池名称，缺省 default */
  storage_pool?: string;
  network: string;
  profiles: string[];
}

export interface UpdateInstanceConfigRequest {
  /** 仅需改动的配置键；空串表示删除该键 */
  config: Record<string, string>;
  /** 可选：期望的完整设备集合（提供时后端走全量 PUT，以支持删除） */
  devices?: Record<string, Record<string, string>>;
  /** 可选：实例顶层描述（非 config 键；undefined=不改动） */
  description?: string;
}

export interface InstanceNetworkAddress {
  family: string;
  address: string;
  scope: string;
}

export interface InstanceNetworkCounters {
  bytes_received: number;
  bytes_sent: number;
  packets_received: number;
  packets_sent: number;
}

export interface InstanceNetworkInfo {
  addresses: InstanceNetworkAddress[];
  counters: InstanceNetworkCounters;
}

export interface InstanceDiskInfo {
  usage: number;
  usage_str: string;
}

export interface InstanceState {
  cpu: { usage: number; usage_str: string };
  /** total 为 limits.memory 解析出的上限（未限制/百分比时为 0）；usage_peak 为历史峰值 */
  memory: { usage: number; usage_peak?: number; usage_str: string; total: number };
  network: Record<string, InstanceNetworkInfo>;
  disk: Record<string, InstanceDiskInfo>;
  processes: number;
}

export interface InstanceInfo {
  name: string;
  status: string;
  type: string;
  /** 实例描述 / 备注（Incus 顶层字段，非 config 键） */
  description?: string;
  profiles: string[];
  config: Record<string, string>;
  devices: Record<string, Record<string, string>>;
  state?: InstanceState;
  created_at: string;
  last_used_at: string;
  architecture: string;
  ephemeral: boolean;
}

export interface ImageAlias {
  name: string;
  description: string;
}

export interface ImageInfo {
  fingerprint: string;
  filename: string;
  size: number;
  architecture: string;
  type: string;
  created_at: string;
  uploaded_at: string;
  expires_at: string;
  aliases: ImageAlias[];
  properties: Record<string, string>;
  public: boolean;
  cached: boolean;
  auto_update: boolean;
}

export interface SnapshotInfo {
  name: string;
  stateful: boolean;
  created_at: string;
  expires_at: string;
}
export interface BackupInfo {
  name: string;
  created_at: string;
  expires_at: string;
  instance_only: boolean;
  optimized_storage: boolean;
  container_only: boolean;
}

export interface StoragePoolInfo {
  name: string;
  driver: string;
  config: Record<string, string>;
  status: string;
  used_by: string[];
  locations: string[];
}

export interface StorageVolumeInfo {
  name: string;
  type: string;
  config: Record<string, string>;
  used_by: string[];
  location: string;
}

export interface NetworkLease {
  hostname: string;
  hwaddr: string;
  address: string;
  type: string;
}

export interface NetworkInfo {
  name: string;
  type: string;
  description: string;
  managed: boolean;
  status: string;
  config: Record<string, string>;
  used_by: string[];
  locations: string[];
  leases?: NetworkLease[];
}

export interface ProfileInfo {
  name: string;
  description: string;
  config: Record<string, string>;
  devices: Record<string, Record<string, string>>;
  used_by: string[];
}

export interface ServerEnvironment {
  kernel: string;
  kernel_architecture: string;
  kernel_version: string;
  os_name: string;
  os_version: string;
  project: string;
  server: string;
  server_version: string;
  driver: string;
  lts: boolean;
}

export interface ServerInfo {
  api_status: string;
  api_version: string;
  environment: ServerEnvironment;
  config: Record<string, unknown>;
}

export interface ServerResources {
  cpu: {
    architecture: string;
    socket_count: number;
    total_cores: number;
    usage: number;
  };
  memory: { total: number; used: number; usage_str: string };
  disk: { total: number; used: number; usage_str: string };
  gpu?: { count: number };
  network?: { interface_count: number };
}

/** 宿主机可选能力探测（GET /api/server/features） */
export interface ServerFeatures {
  /** 宿主机是否安装了 CRIU */
  criu: boolean;
  criu_version?: string;
  /** 容器能否创建 stateful（运行时）快照，等价于 CRIU 可用 */
  container_stateful: boolean;
  /** 虚拟机 stateful 快照由 QEMU 原生支持 */
  vm_stateful: boolean;
}

export interface FileInfo {  name: string;
  path: string;
  type: 'file' | 'directory' | string;
  size: number;
  mode: string;
  uid: number;
  gid: number;
  modified: string;
}

export interface OperationStatus {
  id: string;
  status: string;
  status_code: number;
  description: string;
  created_at: string;
  updated_at: string;
  may_cancel: boolean;
  err: string;
}

/** SSE 事件消息（event/broker.go EventMessage） */
export interface EventMessage {
  type: 'operation' | 'instance' | 'event' | string;
  status: string;
  payload: Record<string, string> | null;
  timestamp: string;
}

/* 网络诊断 */
export interface NetCheck {
  id: string;
  title: string;
  status: 'ok' | 'warn' | 'fail' | 'skip';
  detail: string;
  fixable: boolean;
  fix_desc?: string;
  fix_id?: string;
}

export interface NetworkDiagResult {
  checks: NetCheck[];
  fail: number;
  warn: number;
  checked_at: string;
}

/* 镜像远程源 */
export interface ImageRemote {
  name: string;
  addr: string;
  protocol: string;
  static: boolean;
}

export interface RemoteImage {
  alias: string;
  aliases: string[];
  description: string;
  os: string;
  release: string;
  fingerprint: string;
  size: number;
  type: string;
}

/** GET /api/image-remotes 响应：远程源列表 + 缓存元信息 */
export interface ImageRemotesResult {
  remotes: ImageRemote[];
  cached: boolean;
  updated_at: string;
}

/** GET /api/image-remotes/:name/images 响应：镜像索引 + 缓存元信息 */
export interface RemoteImageResult {
  images: RemoteImage[];
  total: number;
  cached: boolean;
  refreshing: boolean;
  updated_at: string;
  error?: string;
}
