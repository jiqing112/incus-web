/**
 * Incus Web API 客户端。
 * 统一 Bearer Token 认证；401 时清除会话并回到登录页。
 * 开发模式（VITE_MOCK=1）下全部请求由 mock.ts 提供，便于无 Incus 环境演示。
 */
import type {
  ApiResponse,
  CreateInstanceRequest,
  UpdateInstanceConfigRequest,
  InstanceInfo,
  ImageInfo,
  SnapshotInfo,
  BackupInfo,
  StoragePoolInfo,
  StorageVolumeInfo,
  NetworkInfo,
  ProfileInfo,
  ServerInfo,
  ServerResources,
  ServerFeatures,
  FileInfo,
  Operation,
  OperationStatus,
  EventMessage,
  NetworkDiagResult,
  ImageRemotesResult,
  RemoteImageResult,
} from './types';

export const TOKEN_KEY = 'incuspanel-token';

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? '';
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

/** 401 回调：由 session store 注册，用于跳转登录页 */
let onUnauthorized: (() => void) | null = null;
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

/** 开发/演示模式开关：由 vite.config.ts 在启动时注入（VITE_MOCK=1），生产构建恒为 false */
declare const __MOCK__: boolean;
const MOCK = __MOCK__;

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  if (MOCK) {
    const { mockRequest } = await import('./mock');
    return mockRequest<T>(method, path, body);
  }

  const opts: RequestInit = {
    method,
    headers: { Authorization: `Bearer ${getToken()}` },
  };
  if (body !== undefined) {
    opts.headers = { ...opts.headers, 'Content-Type': 'application/json' };
    opts.body = JSON.stringify(body);
  }

  const r = await fetch('/api' + path, opts);
  if (r.status === 401) {
    clearToken();
    onUnauthorized?.();
    throw new ApiError(401, '会话已失效，请重新登录');
  }
  let json: ApiResponse<T>;
  try {
    json = (await r.json()) as ApiResponse<T>;
  } catch {
    throw new ApiError(r.status, `请求失败（HTTP ${r.status}）`);
  }
  if (json.code !== 200) {
    throw new ApiError(json.code, json.error || `请求失败（code ${json.code}）`);
  }
  return json.data as T;
}

/**
 * 轮询异步操作直到完成。
 * - 前 20 次每秒一次，之后每 2 秒一次（大实例备份 / 大镜像导入常持续数十分钟）
 * - 默认总上限 30 分钟（原实现 60 秒，长任务必然被误报为「操作超时」）
 * - 后端在 Incus 已回收操作时返回 Unknown：视为已结束，由调用方刷新列表确认
 */
export async function pollOperation(
  id: string | undefined | null,
  onTick?: (s: string) => void,
  timeoutMs = 30 * 60 * 1000,
): Promise<string> {
  if (!id) return 'Success';
  const deadline = Date.now() + timeoutMs;
  let attempt = 0;
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, attempt < 20 ? 1000 : 2000));
    attempt++;
    const op = await request<OperationStatus>('GET', `/operations/${id}`);
    onTick?.(op.status);
    if (op.status === 'Success' || (op.status_code >= 200 && op.status_code < 300)) return op.status;
    if (op.status === 'Failure' || op.status === 'Cancelled') {
      throw new ApiError(500, op.err || `操作失败（${op.status}）`);
    }
    if (op.status === 'Unknown') {
      // 操作已被 Incus 回收且未赶上终态：不能断言成功，交由调用方刷新确认
      console.warn('[pollOperation]', op.description || '操作状态已不可查询');
      return 'Unknown';
    }
  }
  throw new ApiError(504, '操作超时（任务可能仍在后台执行，可刷新列表查看）');
}

export const api = {
  /* 认证自检（不依赖 Incus，登录校验用） */
  authVerify: () => request<{ incus_reachable: boolean }>('GET', '/auth/verify'),

  /* 服务器 */
  serverInfo: () => request<ServerInfo>('GET', '/server/info'),
  serverResources: () => request<ServerResources>('GET', '/server/resources'),
  serverFeatures: () => request<ServerFeatures>('GET', '/server/features'),

  /* 实例 */
  instances: () => request<InstanceInfo[]>('GET', '/instances'),
  instance: (name: string) => request<InstanceInfo>('GET', `/instances/${name}`),
  createInstance: (body: CreateInstanceRequest) =>
    request<Operation>('POST', '/instances', body),
  updateInstanceConfig: (name: string, body: UpdateInstanceConfigRequest) =>
    request<Operation>('PATCH', `/instances/${name}/config`, body),
  deleteInstance: (name: string, force = true) =>
    request<Operation>('DELETE', `/instances/${name}?force=${force}`),
  setInstanceState: (name: string, action: 'start' | 'stop' | 'restart', timeout = 30) =>
    request<Operation>('PUT', `/instances/${name}/state`, { action, timeout, force: action !== 'start' }),

  /* 快照 */
  snapshots: (name: string) => request<SnapshotInfo[]>('GET', `/instances/${name}/snapshots`),
  createSnapshot: (name: string, body: { name: string; stateful: boolean; expires_at?: string }) =>
    request<Operation>('POST', `/instances/${name}/snapshots`, body),
  restoreSnapshot: (name: string, snap: string, stateful = false) =>
    request<Operation>('POST', `/instances/${name}/snapshots/${snap}/restore`, { stateful }),
  renameSnapshot: (name: string, snap: string, newName: string) =>
    request<Operation>('POST', `/instances/${name}/snapshots/${snap}/rename`, { name: newName }),
  deleteSnapshot: (name: string, snap: string) =>
    request<Operation>('DELETE', `/instances/${name}/snapshots/${snap}`),

  /* 备份 */
  backups: (name: string) => request<BackupInfo[]>('GET', `/instances/${name}/backups`),
  createBackup: (name: string, body: { name: string; instance_only: boolean; optimized_storage: boolean }) =>
    request<Operation>('POST', `/instances/${name}/backup`, body),
  deleteBackup: (name: string, file: string) =>
    request<Operation>('DELETE', `/instances/${name}/backups/${file}`),
  backupDownloadURL: (name: string, file: string) =>
    `/api/instances/${name}/backups/${file}/download?token=${encodeURIComponent(getToken())}`,
  exportInstance: (name: string, image: string) =>
    request<Operation>('POST', `/instances/${name}/export`, { name: image }),

  /* 镜像 */
  images: () => request<ImageInfo[]>('GET', '/images'),
  refreshImages: (server: string) =>
    request<unknown>('POST', '/images/refresh', { server }),
  importImage: (server: string, alias: string) =>
    request<Operation>('POST', '/images/import', { server, alias }),
  deleteImage: (fp: string) => request<unknown>('DELETE', `/images/${fp}`),
  exportImage: (fp: string, format: string) =>
    request<unknown>('POST', '/images/export', { fingerprint: fp, format }),
  /**
   * 镜像导出下载地址。
   * 后端返回的是 tar 二进制流，不能走 request()（会按 JSON 解析而必然报错），
   * 必须用 <a download> 或 location 触发浏览器原生下载。
   */
  imageExportURL: (fp: string, format = 'tar.gz') =>
    `/api/images/${encodeURIComponent(fp)}/export?format=${encodeURIComponent(format)}&token=${encodeURIComponent(getToken())}`,

  /* 存储 */
  storagePools: () => request<StoragePoolInfo[]>('GET', '/storage-pools'),
  storagePool: (name: string) => request<StoragePoolInfo>('GET', `/storage-pools/${name}`),
  createStoragePool: (body: { name: string; driver: string; config: Record<string, string> }) =>
    request<Operation>('POST', '/storage-pools', body),
  deleteStoragePool: (name: string) => request<Operation>('DELETE', `/storage-pools/${name}`),
  storageVolumes: (pool: string) => request<StorageVolumeInfo[]>('GET', `/storage-pools/${pool}/volumes`),
  createStorageVolume: (pool: string, body: { name: string; type: string; config: Record<string, string> }) =>
    request<unknown>('POST', `/storage-pools/${pool}/volumes`, body),

  /* 网络 */
  networks: () => request<NetworkInfo[]>('GET', '/networks'),
  network: (name: string) => request<NetworkInfo>('GET', `/networks/${name}`),
  createNetwork: (body: { name: string; type: string; config: Record<string, string> }) =>
    request<Operation>('POST', '/networks', body),
  updateNetwork: (name: string, body: { config: Record<string, string>; description: string }) =>
    request<unknown>('PUT', `/networks/${name}`, body),
  deleteNetwork: (name: string) => request<Operation>('DELETE', `/networks/${name}`),

  /* 配置文件 */
  profiles: () => request<ProfileInfo[]>('GET', '/profiles'),
  profile: (name: string) => request<ProfileInfo>('GET', `/profiles/${name}`),
  createProfile: (body: { name: string; description: string; config: Record<string, string>; devices: Record<string, Record<string, string>> }) =>
    request<Operation>('POST', '/profiles', body),
  updateProfile: (name: string, body: { description: string; config: Record<string, string>; devices: Record<string, Record<string, string>> }) =>
    request<unknown>('PUT', `/profiles/${name}`, body),
  deleteProfile: (name: string) => request<Operation>('DELETE', `/profiles/${name}`),

  /* 文件 */
  files: (instance: string, path: string) =>
    request<FileInfo[]>('GET', `/instances/${instance}/files?path=${encodeURIComponent(path)}`),
  deleteFile: (instance: string, path: string) =>
    request<unknown>('DELETE', `/instances/${instance}/files?path=${encodeURIComponent(path)}`),
  fileDownloadURL: (instance: string, path: string) =>
    `/api/instances/${instance}/files/download?path=${encodeURIComponent(path)}&token=${encodeURIComponent(getToken())}`,
  uploadFile: async (instance: string, dir: string, file: File) => {
    if (MOCK) {
      const { mockRequest } = await import('./mock');
      await mockRequest('POST', `/instances/${instance}/files/upload`, { dir, name: file.name });
      return;
    }
    const form = new FormData();
    form.append('path', dir.endsWith('/') ? dir : dir + '/');
    form.append('file', file);
    const r = await fetch(`/api/instances/${instance}/files/upload`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${getToken()}` },
      body: form,
    });
    const json = (await r.json()) as ApiResponse;
    if (json.code !== 200) throw new ApiError(json.code, json.error || '上传失败');
  },

  /* 终端 WebSocket（token 走查询参数） */
  terminalURL: (instance: string) => {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${proto}//${location.host}/api/instances/${instance}/terminal?token=${encodeURIComponent(getToken())}`;
  },

  /* 操作查询 */
  operation: (id: string) => request<OperationStatus>('GET', `/operations/${id}`),

  /* 镜像远程源（缓存优先：命中磁盘缓存时秒回，并带 updated_at 元信息） */
  imageRemotes: () => request<ImageRemotesResult>('GET', '/image-remotes'),
  addImageRemote: (name: string, url: string) =>
    request<{ message: string }>('POST', '/image-remotes', { name, url }),
  removeImageRemote: (name: string) =>
    request<{ message: string }>('DELETE', `/image-remotes/${encodeURIComponent(name)}`),
  remoteImages: (name: string, keyword = '', refresh = false) =>
    request<RemoteImageResult>(
      'GET',
      `/image-remotes/${encodeURIComponent(name)}/images?keyword=${encodeURIComponent(keyword)}${refresh ? '&refresh=1' : ''}`,
    ),
  /** 强制刷新某远程源的镜像索引（绕过缓存，同步拉取） */
  refreshRemoteImages: (name: string, keyword = '') =>
    request<RemoteImageResult>(
      'POST',
      `/image-remotes/${encodeURIComponent(name)}/refresh?keyword=${encodeURIComponent(keyword)}`,
    ),

  /* 网络诊断 */
  networkDiagnose: () => request<NetworkDiagResult>('GET', '/network/diagnose'),
  networkRepair: (fixId: string) =>
    request<{ message: string }>('POST', '/network/repair', { fix_id: fixId }),

  /* 日志 */
  instanceLogs: (name: string) => request<{ logs: string }>('GET', `/instances/${name}/logs`),

  /* 安装向导 */
  installStatus: () => request<{
    installed: boolean;
    version: string;
    initialized: boolean;
    service_running: boolean;
    os_name: string;
    os_version: string;
    /** 面板实际使用的 Incus unix socket 路径（Debian 与 RHEL 系不同） */
    socket_path?: string;
    socket_exists?: boolean;
    /** 能否连上该 socket；为 false 时除安装向导外面板不可用 */
    api_reachable?: boolean;
    /** 面板进程是否为 root；为 false 时无法执行安装脚本 */
    is_root?: boolean;
  }>('GET', '/server/install-status'),
  installIncus: (body: {
    storage_backend?: string;
    storage_size_gb?: number;
    storage_pool?: string;
    network_bridge?: string;
    install_method?: 'auto' | 'package' | 'source';
  }) => request<{ message: string; log_file: string; started_at: string }>('POST', '/server/install', body),
  installLog: (file: string) =>
    request<{ lines: string[]; exists: boolean; done: boolean; success: boolean }>(
      'GET',
      `/server/install-log?file=${encodeURIComponent(file)}`,
    ),
};

/**
 * 订阅全局 SSE 事件流。
 * 返回取消函数；内部自动重连（3s 退避）。
 */
export function subscribeEvents(
  handler: (msg: EventMessage) => void,
  onState?: (connected: boolean) => void,
): () => void {
  if (MOCK) {
    let stopped = false;
    onState?.(true);
    import('./mock').then(({ startMockEvents }) => startMockEvents(handler));
    return () => {
      stopped = true;
    };
  }
  let es: EventSource | null = null;
  let retry: ReturnType<typeof setTimeout> | null = null;
  let closed = false;

  const connect = () => {
    if (closed) return;
    es = new EventSource(`/api/events?token=${encodeURIComponent(getToken())}`);
    es.onopen = () => onState?.(true);
    es.onmessage = (e) => {
      try {
        handler(JSON.parse(e.data) as EventMessage);
      } catch {
        /* 忽略无法解析的心跳/错误帧 */
      }
    };
    es.onerror = () => {
      onState?.(false);
      es?.close();
      es = null;
      if (!closed) retry = setTimeout(connect, 3000);
    };
  };
  connect();

  return () => {
    closed = true;
    if (retry) clearTimeout(retry);
    es?.close();
  };
}
