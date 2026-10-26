/**
 * 开发/演示用 mock 后端：内存中维护一个逼真的 Incus 服务器世界。
 * 通过 VITE_MOCK=1 启用；生产构建不打包生效路径的数据变更逻辑。
 */
import type {
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
  FileInfo,
  Operation,
  OperationStatus,
  EventMessage,
} from './types';

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

let opSeq = 0;
const operations = new Map<string, OperationStatus>();

function newOp(description: string): Operation {
  const id = `mock-op-${String(++opSeq).padStart(4, '0')}`;
  const op: OperationStatus = {
    id,
    status: 'Running',
    status_code: 103,
    description,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    may_cancel: true,
    err: '',
  };
  operations.set(id, op);
  // 80% 概率 1.5~3s 后成功
  setTimeout(() => {
    op.status = 'Success';
    op.status_code = 200;
    op.updated_at = new Date().toISOString();
  }, 1500 + Math.random() * 1500);
  return { operation_id: id, status_url: `/1.0/operations/${id}`, status: op.status };
}

function now(offsetMin = 0): string {
  return new Date(Date.now() - offsetMin * 60000).toISOString();
}

/* ------------------------------ 实例世界 ------------------------------ */

function mkNet(ipv4: string) {
  return {
    eth0: {
      addresses: [
        { family: 'inet', address: ipv4, scope: 'global' },
        { family: 'inet6', address: `fd42:b64a:2f6e::${Math.floor(Math.random() * 200 + 1)}`, scope: 'global' },
        { family: 'inet6', address: 'fe80::201:ff:fe01:1001', scope: 'link' },
      ],
      counters: {
        bytes_received: Math.floor(Math.random() * 8e9 + 2e9),
        bytes_sent: Math.floor(Math.random() * 5e9 + 1e9),
        packets_received: Math.floor(Math.random() * 6e6 + 1e6),
        packets_sent: Math.floor(Math.random() * 4e6 + 5e5),
      },
    },
    lo: { addresses: [], counters: { bytes_received: 12e6, bytes_sent: 12e6, packets_received: 96000, packets_sent: 96000 } },
  };
}

interface WorldInstance {
  name: string;
  status: string;
  type: string;
  profiles: string[];
  ipv4: string;
  cpuUsageNs: number;
  memUsed: number;
  memTotal: number;
  diskUsed: number;
  processes: number;
  createdAt: string;
  os: string;
  release: string;
}

const world: WorldInstance[] = [
  { name: 'web-nginx', status: 'Running', type: 'container', profiles: ['default'], ipv4: '10.10.10.101', cpuUsageNs: 8.62e12, memUsed: 686 * 1024 * 1024, memTotal: 2 * 1024 * 1024 * 1024, diskUsed: 3.1 * 1024 * 1024 * 1024, processes: 24, createdAt: now(60 * 24 * 42), os: 'Debian', release: '12' },
  { name: 'db-postgres', status: 'Running', type: 'container', profiles: ['default', 'db'], ipv4: '10.10.10.102', cpuUsageNs: 3.14e13, memUsed: 4.2 * 1024 * 1024 * 1024, memTotal: 8 * 1024 * 1024 * 1024, diskUsed: 22.6 * 1024 * 1024 * 1024, processes: 41, createdAt: now(60 * 24 * 120), os: 'Ubuntu', release: '24.04' },
  { name: 'cache-redis', status: 'Running', type: 'container', profiles: ['default'], ipv4: '10.10.10.103', cpuUsageNs: 1.9e12, memUsed: 924 * 1024 * 1024, memTotal: 2 * 1024 * 1024 * 1024, diskUsed: 820 * 1024 * 1024, processes: 6, createdAt: now(60 * 24 * 30), os: 'Alpine', release: '3.20' },
  { name: 'build-runner', status: 'Stopped', type: 'virtual-machine', profiles: ['default', 'dev'], ipv4: '', cpuUsageNs: 4.7e13, memUsed: 0, memTotal: 8 * 1024 * 1024 * 1024, diskUsed: 31 * 1024 * 1024 * 1024, processes: 0, createdAt: now(60 * 24 * 15), os: 'Fedora', release: '40' },
  { name: 'monitor-grafana', status: 'Running', type: 'container', profiles: ['default', 'docker'], ipv4: '10.10.10.105', cpuUsageNs: 9.8e12, memUsed: 1.3 * 1024 * 1024 * 1024, memTotal: 4 * 1024 * 1024 * 1024, diskUsed: 6.7 * 1024 * 1024 * 1024, processes: 18, createdAt: now(60 * 24 * 66), os: 'Ubuntu', release: '22.04' },
  { name: 'test-alpine', status: 'Stopped', type: 'container', profiles: ['default'], ipv4: '', cpuUsageNs: 3.3e11, memUsed: 0, memTotal: 512 * 1024 * 1024, diskUsed: 112 * 1024 * 1024, processes: 0, createdAt: now(60 * 24 * 3), os: 'Alpine', release: '3.21' },
];

function toInstanceInfo(w: WorldInstance): InstanceInfo {
  const running = w.status === 'Running';
  return {
    name: w.name,
    status: w.status,
    type: w.type,
    profiles: w.profiles,
    config: {
      'image.os': w.os,
      'image.release': w.release,
      ...(w.type === 'container'
        ? { 'limits.cpu': String(Math.ceil(w.memTotal / (1024 * 1024 * 1024)) ) }
        : { 'limits.memory': `${w.memTotal / (1024 * 1024 * 1024)}GiB` }),
    },
    devices: {
      eth0: { name: 'eth0', network: 'incusbr0', type: 'nic' },
      root: { path: '/', pool: 'local', type: 'disk' },
    },
    state: running
      ? {
          cpu: { usage: w.cpuUsageNs, usage_str: `${(w.cpuUsageNs / 3.6e12).toFixed(1)}s` },
          memory: { usage: w.memUsed, usage_str: `${(w.memUsed / 1024 ** 3).toFixed(2)}GiB`, total: w.memTotal },
          network: mkNet(w.ipv4),
          disk: { root: { usage: w.diskUsed, usage_str: `${(w.diskUsed / 1024 ** 3).toFixed(2)}GiB` } },
          processes: w.processes,
        }
      : undefined,
    created_at: w.createdAt,
    last_used_at: running ? now(2) : now(60 * 26),
    architecture: 'x86_64',
    ephemeral: false,
  };
}

/* ------------------------------ 镜像 ------------------------------ */

const images: ImageInfo[] = [
  { fingerprint: '3f1cba2105 d6a8f9e2b7c4d1e0f8a9b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7', filename: '', size: 118 * 1024 * 1024, architecture: 'x86_64', type: 'container', created_at: now(60 * 24 * 20), uploaded_at: now(60 * 24 * 18), expires_at: now(-60 * 24 * 60), aliases: [{ name: 'debian/12', description: 'Debian bookworm (amd64)' }], properties: { os: 'Debian', release: '12', variant: 'default' }, public: false, cached: true, auto_update: true },
  { fingerprint: 'a91b44e2093c8d7f6e5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e', filename: '', size: 186 * 1024 * 1024, architecture: 'x86_64', type: 'container', created_at: now(60 * 24 * 33), uploaded_at: now(60 * 24 * 31), expires_at: now(-60 * 24 * 52), aliases: [{ name: 'ubuntu/24.04', description: 'Ubuntu noble (amd64)' }], properties: { os: 'Ubuntu', release: 'noble', variant: 'default' }, public: false, cached: true, auto_update: true },
  { fingerprint: '77c2d010facb9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d', filename: '', size: 74 * 1024 * 1024, architecture: 'x86_64', type: 'container', created_at: now(60 * 24 * 11), uploaded_at: now(60 * 24 * 10), expires_at: now(-60 * 24 * 80), aliases: [{ name: 'alpine/3.20', description: 'Alpine 3.20 (amd64)' }], properties: { os: 'Alpine', release: '3.20' }, public: false, cached: false, auto_update: false },
  { fingerprint: 'e02b98d4716a5b4c3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8a7b6c5d4e3f2a1b0c', filename: 'fedora40.img', size: 1.6 * 1024 * 1024 * 1024, architecture: 'x86_64', type: 'virtual-machine', created_at: now(60 * 24 * 15), uploaded_at: now(60 * 24 * 15), expires_at: '', aliases: [{ name: 'fedora/40', description: 'Fedora 40 (amd64) VM' }], properties: { os: 'Fedora', release: '40' }, public: false, cached: false, auto_update: false },
  { fingerprint: '5b8e17d9302a4c5b6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c', filename: '', size: 142 * 1024 * 1024, architecture: 'x86_64', type: 'container', created_at: now(60 * 24 * 7), uploaded_at: now(60 * 24 * 6), expires_at: '', aliases: [{ name: 'ubuntu/22.04', description: 'Ubuntu jammy (amd64)' }], properties: { os: 'Ubuntu', release: 'jammy' }, public: false, cached: false, auto_update: false },
];

/* --------------------------- 快照与备份 --------------------------- */

const snapshots = new Map<string, SnapshotInfo[]>([
  ['web-nginx', [
    { name: 'snap0', stateful: false, created_at: now(60 * 26), expires_at: '' },
    { name: 'pre-upgrade', stateful: false, created_at: now(60 * 8), expires_at: now(-60 * 24 * 7) },
    { name: 'daily-2026-06-01', stateful: false, created_at: now(60 * 3), expires_at: now(-60 * 24 * 6) },
  ]],
  ['db-postgres', [
    { name: 'pre-migration', stateful: false, created_at: now(60 * 50), expires_at: '' },
    { name: 'nightly', stateful: false, created_at: now(60 * 12), expires_at: now(-60 * 24 * 3) },
  ]],
  ['cache-redis', [{ name: 'snap0', stateful: false, created_at: now(60 * 29), expires_at: '' }]],
]);

const backups = new Map<string, BackupInfo[]>([
  ['web-nginx', [
    { name: 'web-nginx-2026-05-30.tar.gz', created_at: now(60 * 24 * 2), expires_at: '', instance_only: true, optimized_storage: false, container_only: true },
    { name: 'web-nginx-2026-05-23.tar.gz', created_at: now(60 * 24 * 9), expires_at: '', instance_only: true, optimized_storage: false, container_only: true },
  ]],
  ['db-postgres', [
    { name: 'db-postgres-full-2026-05-28.tar', created_at: now(60 * 24 * 4), expires_at: '', instance_only: false, optimized_storage: true, container_only: false },
  ]],
]);

/* ------------------------------ 网络 ------------------------------ */

const networks: NetworkInfo[] = [
  { name: 'incusbr0', type: 'bridge', description: 'Default bridge network', managed: true, status: 'Created', config: { 'ipv4.address': '10.10.10.1/24', 'ipv4.nat': 'true', 'ipv6.address': 'fd42:b64a:2f6e::1/64', 'ipv6.nat': 'true' }, used_by: ['/1.0/instances/web-nginx', '/1.0/instances/db-postgres', '/1.0/instances/cache-redis', '/1.0/profiles/default'], locations: [], leases: [ { hostname: 'web-nginx', hwaddr: '00:16:3e:01:10:01', address: '10.10.10.101', type: 'dynamic' }, { hostname: 'db-postgres', hwaddr: '00:16:3e:01:10:02', address: '10.10.10.102', type: 'dynamic' }, { hostname: 'cache-redis', hwaddr: '00:16:3e:01:10:03', address: '10.10.10.103', type: 'dynamic' } ] },
  { name: 'incusbr1', type: 'bridge', description: 'DMZ network', managed: true, status: 'Created', config: { 'ipv4.address': '10.10.20.1/24', 'ipv4.nat': 'true' }, used_by: ['/1.0/instances/monitor-grafana'], locations: [], leases: [] },
  { name: 'lo', type: 'loopback', description: '', managed: false, status: 'Created', config: {}, used_by: [], locations: [], leases: [] },
];

/* ------------------------------ 存储 ------------------------------ */

const pools: StoragePoolInfo[] = [
  { name: 'local', driver: 'zfs', status: 'Created', config: { source: '/var/lib/incus/storage-pools/local', 'zfs.pool_name': 'incus-local', size: '512GiB' }, used_by: ['/1.0/instances/web-nginx', '/1.0/instances/db-postgres', '/1.0/instances/cache-redis', '/1.0/profiles/default'], locations: [] },
  { name: 'remote', driver: 'dir', status: 'Created', config: { source: '/srv/incus/remote' }, used_by: ['/1.0/instances/build-runner'], locations: [] },
];

const volumes: Record<string, StorageVolumeInfo[]> = {
  local: [
    { name: 'container/web-nginx', type: 'container', config: {}, used_by: ['/1.0/instances/web-nginx'], location: '' },
    { name: 'container/db-postgres', type: 'container', config: {}, used_by: ['/1.0/instances/db-postgres'], location: '' },
    { name: 'container/cache-redis', type: 'container', config: {}, used_by: ['/1.0/instances/cache-redis'], location: '' },
    { name: 'custom/pgdata', type: 'custom', config: { size: '20GiB' }, used_by: ['/1.0/instances/db-postgres'], location: '' },
    { name: 'custom/grafana', type: 'custom', config: { size: '10GiB' }, used_by: ['/1.0/instances/monitor-grafana'], location: '' },
    { name: 'image/3f1cba2105d6a8f9e2b7c4d1e0f8a9b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f', type: 'image', config: {}, used_by: [], location: '' },
  ],
  remote: [
    { name: 'virtual-machine/build-runner', type: 'virtual-machine', config: {}, used_by: ['/1.0/instances/build-runner'], location: '' },
  ],
};

/* ---------------------------- 配置文件 ---------------------------- */

const profiles: ProfileInfo[] = [
  { name: 'default', description: 'Default Incus profile', config: { 'limits.memory': '4GiB', 'boot.autostart': '1' }, devices: { eth0: { name: 'eth0', network: 'incusbr0', type: 'nic' }, root: { path: '/', pool: 'local', type: 'disk' } }, used_by: ['/1.0/instances/web-nginx', '/1.0/instances/db-postgres', '/1.0/instances/cache-redis', '/1.0/instances/test-alpine'] },
  { name: 'docker', description: 'Docker-in-Incus (nesting + fan overlay)', config: { 'security.nesting': 'true', 'security.syscalls.intercept.mknod': 'true' }, devices: { eth0: { name: 'eth0', network: 'incusbr1', type: 'nic' } }, used_by: ['/1.0/instances/monitor-grafana'] },
  { name: 'dev', description: '开发调试：更多 CPU 与 /dev/kvm', config: { 'limits.cpu': '8' }, devices: { kvm: { path: '/dev/kvm', type: 'unix-char' } }, used_by: ['/1.0/instances/build-runner'] },
  { name: 'db', description: '数据库实例：更大内存与自定义卷', config: { 'limits.memory': '8GiB', 'limits.cpu': '4' }, devices: { pgdata: { path: '/var/lib/postgresql', pool: 'local', source: 'pgdata', type: 'disk' } }, used_by: ['/1.0/instances/db-postgres'] },
];

/* ------------------------------ 文件 ------------------------------ */

interface FsNode {
  name: string;
  type: 'file' | 'directory';
  size: number;
  mode: string;
  modified: string;
  children?: Record<string, FsNode>;
}

function d(name: string, children: Record<string, FsNode> = {}, mode = 'drwxr-xr-x'): FsNode {
  return { name, type: 'directory', size: 4096, mode, modified: now(Math.floor(Math.random() * 4000)), children };
}
function f(name: string, size = 1024, mode = '-rw-r--r--'): FsNode {
  return { name, type: 'file', size, mode, modified: now(Math.floor(Math.random() * 4000)) };
}

function buildFs(): FsNode {
  return d('/', {
    bin: d('bin', { sh: f('sh', 121432, '-rwxr-xr-x'), ls: f('ls', 142096, '-rwxr-xr-x') }),
    etc: d('etc', {
      hostname: f('hostname', 13),
      hosts: f('hosts', 221),
      'resolv.conf': f('resolv.conf', 148),
      passwd: f('passwd', 1204),
      ssh: d('ssh', { sshd_config: f('sshd_config', 3188) }, 'drwxr-xr-x'),
    }, 'drwxr-xr-x'),
    home: d('home', {
      ubuntu: d('ubuntu', {
        '.bashrc': f('.bashrc', 3771),
        'README.md': f('README.md', 512),
        app: d('app', { 'main.py': f('main.py', 4096), 'requirements.txt': f('requirements.txt', 220), '.env': f('.env', 128, '-rw-------') }, 'drwxr-xr-x'),
        'notes.txt': f('notes.txt', 96),
      }, 'drwxr-x---'),
    }),
    root: d('root', { '.profile': f('.profile', 607), '.bash_history': f('.bash_history', 2048, '-rw-------') }, 'drwx------'),
    tmp: d('tmp', {}, 'drwxrwxrwt'),
    var: d('var', {
      log: d('log', { syslog: f('syslog', 81234), 'nginx': d('nginx', { 'access.log': f('access.log', 2048576), 'error.log': f('error.log', 18432) }, 'drwxr-xr-x') }, 'drwxr-xr-x'),
      www: d('www', { html: d('html', { 'index.html': f('index.html', 1256), 'logo.svg': f('logo.svg', 4210) }, 'drwxr-xr-x') }, 'drwxr-xr-x'),
    }),
  });
}

const filesystems = new Map<string, FsNode>(world.map((w) => [w.name, buildFs()]));

function findNode(root: FsNode, path: string): FsNode | null {
  const parts = path.split('/').filter(Boolean);
  let node: FsNode = root;
  for (const p of parts) {
    if (node.type !== 'directory' || !node.children?.[p]) return null;
    node = node.children[p];
  }
  return node;
}

function toFileInfos(node: FsNode, dirPath: string): FileInfo[] {
  const list: FileInfo[] = [];
  for (const child of Object.values(node.children ?? {})) {
    list.push({
      name: child.name,
      path: (dirPath === '/' ? '' : dirPath) + '/' + child.name,
      type: child.type,
      size: child.size,
      mode: child.mode.replace(/^d/, 'd').replace(/^-/, '-'),
      uid: child.mode.includes('rwx') ? 0 : 1000,
      gid: 0,
      modified: child.modified,
    });
  }
  return list.sort((a, b) => {
    if (a.type !== b.type) return a.type === 'directory' ? -1 : 1;
    return a.name.localeCompare(b.name);
  });
}

/* ------------------------------ 服务器 ------------------------------ */

const cpuTotal = 16;
const memTotal = 32 * 1024 * 1024 * 1024;
const diskTotal = 512 * 1024 * 1024 * 1024;

/** CPU 累计占用时间（ns）：随机游走的忙碌核心数，供前端差分出实时占用率 */
let cpuCumulativeNs = 8.4e12;
let busyCores = 3.2;
let lastCpuTs = Date.now();

function cpuUsageNs(): number {
  const nowMs = Date.now();
  const elapsedS = Math.min((nowMs - lastCpuTs) / 1000, 30);
  lastCpuTs = nowMs;
  busyCores = Math.min(Math.max(busyCores + (Math.random() - 0.5) * 2.4, 0.6), 13);
  cpuCumulativeNs += elapsedS * busyCores * 1e9;
  return cpuCumulativeNs;
}

function memBase(): number {
  const running = world.filter((w) => w.status === 'Running');
  const jitter = Math.sin(Date.now() / 9000) * 140 * 1024 * 1024;
  return running.reduce((s, w) => s + w.memUsed, 0) + 3.2 * 1024 * 1024 * 1024 + jitter;
}

function serverInfo(): ServerInfo {
  return {
    api_status: 'Stable',
    api_version: '1.0',
    environment: {
      kernel: 'Linux',
      kernel_architecture: 'x86_64',
      kernel_version: '6.8.0-45-generic',
      os_name: 'Ubuntu',
      os_version: '24.04 LTS',
      project: 'default',
      server: 'incus',
      server_version: '6.0.2',
      driver: 'lxc | qemu',
      lts: true,
    },
    config: { 'core.https_address': '[::]:8443', 'core.trust_password': false },
  };
}

function serverResources(): ServerResources {
  return {
    cpu: { architecture: 'x86_64', socket_count: 1, total_cores: cpuTotal, usage: cpuUsageNs() },
    memory: { total: memTotal, used: memBase(), usage_str: `${(memBase() / 1024 ** 3).toFixed(1)}GiB` },
    disk: { total: diskTotal, used: 187 * 1024 * 1024 * 1024, usage_str: '187GiB' },
    network: { interface_count: 4 },
  };
}

/* --------------------------- 事件发生器 --------------------------- */

let eventHandler: ((m: EventMessage) => void) | null = null;
let eventTimer: ReturnType<typeof setInterval> | null = null;

const sampleEvents: Array<() => EventMessage> = [
  () => ({ type: 'instance', status: 'updated', payload: { instance: world[Math.floor(Math.random() * world.length)].name, action: 'cpu-usage' }, timestamp: new Date().toISOString() }),
  () => ({ type: 'operation', status: 'success', payload: { operation_id: `auto-${Math.random().toString(36).slice(2, 8)}`, description: 'Showing log files' }, timestamp: new Date().toISOString() }),
  () => ({ type: 'event', status: 'success', payload: { description: 'Instance state refreshed' }, timestamp: new Date().toISOString() }),
];

export function startMockEvents(handler: (m: EventMessage) => void) {
  eventHandler = handler;
  if (eventTimer) clearInterval(eventTimer);
  eventTimer = setInterval(() => {
    if (!eventHandler || Math.random() < 0.35) return;
    eventHandler(sampleEvents[Math.floor(Math.random() * sampleEvents.length)]());
  }, 4000);
}

/* ------------------------------ 调度入口 ------------------------------ */

export async function mockRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
  await sleep(90 + Math.random() * 180); // 模拟网络与 unix socket 延迟
  const [p, qs] = path.split('?');
  const params = new URLSearchParams(qs ?? '');
  const seg = p.split('/').filter(Boolean);
  const ok = (data?: unknown) => data as T;
  const fail = (code: number, error: string): T => {
    throw new Error(error);
  };

  // PATCH /instances/:name/config —— 同步生效（空串删除键）；devices 走全量 PUT
  if (seg[0] === 'instances' && seg[2] === 'config' && method === 'PATCH') {
    const w = world.find((x) => x.name === seg[1]);
    if (!w) return fail(404, '实例不存在');
    const changes = (body as { config?: Record<string, string | null>; devices?: Record<string, Record<string, string>> }) ?? {};
    void changes; // mock 世界不持久化 config 细节，返回成功即可驱动 UI 流程
    return ok('');
  }

  // 网络诊断
  if (p === '/network/diagnose' && method === 'GET') {
    return ok({
      checks: [
        { id: 'ip_forward', title: 'IPv4 转发已启用', status: 'ok', detail: 'net.ipv4.ip_forward = 1', fixable: false },
        { id: 'forward_policy', title: 'FORWARD 链策略', status: 'ok', detail: '-P FORWARD ACCEPT', fixable: false },
        { id: 'docker_conflict', title: 'Docker 转发链冲突', status: 'ok', detail: '未安装 Docker（无 docker0 网桥），无转发链冲突风险', fixable: false },
        { id: 'fwd_incusbr0', title: '网桥转发放行（incusbr0）', status: 'ok', detail: 'nftables fwd.incusbr0 双向 accept 已存在', fixable: false },
        { id: 'masq_incusbr0', title: '出站 MASQUERADE（incusbr0）', status: 'ok', detail: 'nftables pstrt.incusbr0 伪装规则已存在', fixable: false },
        { id: 'host_egress', title: '宿主机出外网', status: 'ok', detail: 'ping 223.5.5.5 通', fixable: false },
        { id: 'container_egress', title: '容器出外网实测', status: 'ok', detail: '实例 web-nginx 内 ping 223.5.5.5 通', fixable: false },
      ],
      fail: 0,
      warn: 0,
      checked_at: new Date().toLocaleString('zh-CN', { hour12: false }),
    });
  }
  if (p === '/network/repair' && method === 'POST') return ok({ message: '修复完成（演示）' });

  // GET /server/*
  if (p === '/server/info') return ok(serverInfo());
  if (p === '/server/resources') return ok(serverResources());

  // 安装向导（演示：视为已安装且已初始化）
  if (p === '/server/install-status' && method === 'GET') {
    return ok({
      installed: true, version: '6.0.0', initialized: true, service_running: true,
      os_name: 'Debian GNU/Linux', os_version: '13',
    });
  }
  if (p === '/server/install' && method === 'POST') {
    return ok({ message: '安装已启动', log_file: '/tmp/mock_install.log', started_at: new Date().toISOString() });
  }
  if (p === '/server/install-log' && method === 'GET') {
    return ok({ lines: ['[00:00:00] [DONE] 演示模式：无需安装 (rc=0)'], exists: true, done: true, success: true });
  }

  // /instances
  if (p === '/instances' && method === 'GET') return ok(world.map(toInstanceInfo));
  if (p === '/instances' && method === 'POST') {
    const b = body as { name: string; type: string; image_alias: string };
    if (world.some((w) => w.name === b.name)) return fail(500, `实例「${b.name}」已存在`);
    const w: WorldInstance = {
      name: b.name, status: 'Running', type: b.type, profiles: ['default'],
      ipv4: `10.10.10.${110 + world.length}`, cpuUsageNs: 0,
      memUsed: 120 * 1024 * 1024, memTotal: 2 * 1024 * 1024 * 1024,
      diskUsed: 320 * 1024 * 1024, processes: 8,
      createdAt: new Date().toISOString(), os: b.image_alias.split('/')[0], release: b.image_alias.split('/')[1] ?? '',
    };
    world.push(w);
    filesystems.set(w.name, buildFs());
    eventHandler?.({ type: 'instance', status: 'created', payload: { instance: w.name, action: 'created' }, timestamp: new Date().toISOString() });
    return ok(newOp(`Creating ${b.name}`));
  }
  if (seg[0] === 'instances' && seg.length === 2 && method === 'GET') {
    const w = world.find((x) => x.name === seg[1]);
    return w ? ok(toInstanceInfo(w)) : fail(404, '实例不存在');
  }
  if (seg[0] === 'instances' && seg.length === 2 && method === 'DELETE') {
    const i = world.findIndex((x) => x.name === seg[1]);
    if (i < 0) return fail(404, '实例不存在');
    const [gone] = world.splice(i, 1);
    filesystems.delete(gone.name);
    eventHandler?.({ type: 'instance', status: 'deleted', payload: { instance: gone.name, action: 'deleted' }, timestamp: new Date().toISOString() });
    return ok(newOp(`Deleting ${gone.name}`));
  }
  if (seg[0] === 'instances' && seg[2] === 'state' && method === 'PUT') {
    const w = world.find((x) => x.name === seg[1]);
    if (!w) return fail(404, '实例不存在');
    const action = (body as { action: string }).action;
    setTimeout(() => {
      w.status = action === 'stop' ? 'Stopped' : 'Running';
      if (action !== 'stop') {
        w.memUsed = Math.max(w.memUsed, 96 * 1024 * 1024);
        w.processes = w.processes || 12;
      } else {
        w.memUsed = 0;
        w.processes = 0;
      }
      eventHandler?.({ type: 'instance', status: action === 'stop' ? 'stopped' : 'started', payload: { instance: w.name, action }, timestamp: new Date().toISOString() });
    }, 1800);
    return ok(newOp(`${action} ${w.name}`));
  }

  // 快照
  if (seg[2] === 'snapshots' && seg.length === 3 && method === 'GET') {
    return ok(snapshots.get(seg[1]) ?? []);
  }
  if (seg[2] === 'snapshots' && seg.length === 3 && method === 'POST') {
    const b = body as { name: string };
    const list = snapshots.get(seg[1]) ?? [];
    list.unshift({ name: b.name, stateful: false, created_at: new Date().toISOString(), expires_at: '' });
    snapshots.set(seg[1], list);
    return ok(newOp(`Snapshot ${b.name}`));
  }
  if (seg[2] === 'snapshots' && seg.length === 5 && seg[4] === 'restore') {
    return ok(newOp(`Restore ${seg[3]}`));
  }
  if (seg[2] === 'snapshots' && seg.length === 4 && method === 'DELETE') {
    const list = (snapshots.get(seg[1]) ?? []).filter((s) => s.name !== seg[3]);
    snapshots.set(seg[1], list);
    return ok(newOp(`Delete snapshot ${seg[3]}`));
  }

  // 备份
  if (seg[2] === 'backups' && seg.length === 3 && method === 'GET') return ok(backups.get(seg[1]) ?? []);
  if (seg[2] === 'backup' && seg.length === 3 && method === 'POST') {
    const b = body as { name: string };
    const list = backups.get(seg[1]) ?? [];
    const file = `${b.name || seg[1] + '-' + new Date().toISOString().slice(0, 10)}.tar.gz`;
    list.unshift({ name: file, created_at: new Date().toISOString(), expires_at: '', instance_only: true, optimized_storage: false, container_only: true });
    backups.set(seg[1], list);
    return ok(newOp(`Backup ${seg[1]}`));
  }
  if (seg[2] === 'backups' && seg.length === 4 && method === 'DELETE') {
    backups.set(seg[1], (backups.get(seg[1]) ?? []).filter((b) => b.name !== seg[3]));
    return ok(newOp(`Delete backup ${seg[3]}`));
  }
  if (seg[2] === 'export' && seg.length === 3 && method === 'POST') return ok(newOp(`Export ${seg[1]}`));

  // 镜像
  if (p === '/images' && method === 'GET') return ok(images);
  if (p === '/images/refresh' && method === 'POST') return ok({ refreshed: images.length });
  if (p === '/images/import' && method === 'POST') {
    const b = body as { alias: string };
    images.unshift({ fingerprint: Math.random().toString(16).slice(2).padEnd(64, '0'), filename: '', size: 128 * 1024 * 1024, architecture: 'x86_64', type: 'container', created_at: new Date().toISOString(), uploaded_at: new Date().toISOString(), expires_at: '', aliases: [{ name: b.alias, description: b.alias }], properties: {}, public: false, cached: true, auto_update: true });
    return ok(newOp(`Import ${b.alias}`));
  }
  if (seg[0] === 'images' && seg.length === 2 && method === 'DELETE') {
    const i = images.findIndex((x) => x.fingerprint === seg[1]);
    if (i >= 0) images.splice(i, 1);
    return ok('ok');
  }
  if (p === '/images/export' && method === 'POST') return ok(newOp('Export image'));

  // 存储
  if (p === '/storage-pools' && method === 'GET') return ok(pools);
  if (p === '/storage-pools' && method === 'POST') {
    const b = body as { name: string; driver: string; config: Record<string, string> };
    pools.push({ name: b.name, driver: b.driver, status: 'Created', config: b.config, used_by: [], locations: [] });
    volumes[b.name] = [];
    return ok(newOp(`Create pool ${b.name}`));
  }
  if (seg[0] === 'storage-pools' && seg.length === 2 && method === 'GET') {
    return ok(pools.find((x) => x.name === seg[1]) ?? pools[0]);
  }
  if (seg[0] === 'storage-pools' && seg.length === 2 && method === 'DELETE') {
    const i = pools.findIndex((x) => x.name === seg[1]);
    if (i >= 0) pools.splice(i, 1);
    delete volumes[seg[1]];
    return ok(newOp(`Delete pool ${seg[1]}`));
  }
  if (seg[0] === 'storage-pools' && seg[2] === 'volumes' && seg.length === 3 && method === 'GET') {
    return ok(volumes[seg[1]] ?? []);
  }
  if (seg[0] === 'storage-pools' && seg[2] === 'volumes' && seg.length === 3 && method === 'POST') {
    const b = body as { name: string; type: string; config: Record<string, string> };
    (volumes[seg[1]] ??= []).push({ name: b.name, type: b.type, config: b.config, used_by: [], location: '' });
    return ok(newOp(`Create volume ${b.name}`));
  }

  // 网络
  if (p === '/networks' && method === 'GET') return ok(networks);
  if (p === '/networks' && method === 'POST') {
    const b = body as { name: string; type: string; config: Record<string, string> };
    networks.push({ name: b.name, type: b.type, description: '', managed: true, status: 'Created', config: b.config, used_by: [], locations: [], leases: [] });
    return ok(newOp(`Create network ${b.name}`));
  }
  if (seg[0] === 'networks' && seg.length === 2 && method === 'GET') {
    return ok(networks.find((x) => x.name === seg[1]) ?? networks[0]);
  }
  if (seg[0] === 'networks' && seg.length === 2 && method === 'PUT') return ok('ok');
  if (seg[0] === 'networks' && seg.length === 2 && method === 'DELETE') {
    const i = networks.findIndex((x) => x.name === seg[1]);
    if (i >= 0) networks.splice(i, 1);
    return ok(newOp(`Delete network ${seg[1]}`));
  }

  // 配置文件
  if (p === '/profiles' && method === 'GET') return ok(profiles);
  if (p === '/profiles' && method === 'POST') {
    const b = body as { name: string; description: string; config: Record<string, string>; devices: Record<string, Record<string, string>> };
    profiles.push({ name: b.name, description: b.description, config: b.config, devices: b.devices, used_by: [] });
    return ok(newOp(`Create profile ${b.name}`));
  }
  if (seg[0] === 'profiles' && seg.length === 2 && method === 'GET') {
    return ok(profiles.find((x) => x.name === seg[1]) ?? profiles[0]);
  }
  if (seg[0] === 'profiles' && seg.length === 2 && method === 'PUT') return ok('ok');
  if (seg[0] === 'profiles' && seg.length === 2 && method === 'DELETE') {
    const i = profiles.findIndex((x) => x.name === seg[1]);
    if (i >= 0 && profiles[i].name !== 'default') profiles.splice(i, 1);
    return ok(newOp(`Delete profile ${seg[1]}`));
  }

  // 文件
  if (seg[2] === 'files' && seg.length === 3 && method === 'GET') {
    const fs = filesystems.get(seg[1]);
    const node = fs ? findNode(fs, params.get('path') ?? '/') : null;
    if (!node || node.type !== 'directory') return fail(500, '目录不存在');
    return ok(toFileInfos(node, params.get('path') ?? '/'));
  }
  if (seg[2] === 'files' && seg.length === 3 && method === 'DELETE') {
    const fs = filesystems.get(seg[1]);
    const target = params.get('path') ?? '/';
    const parent = fs ? findNode(fs, target.slice(0, target.lastIndexOf('/')) || '/') : null;
    if (parent?.children) delete parent.children[target.slice(target.lastIndexOf('/') + 1)];
    return ok('ok');
  }
  if (seg[2] === 'files' && seg[3] === 'upload' && method === 'POST') {
    const b = body as { dir: string; name: string };
    const fs = filesystems.get(seg[1]);
    const node = fs ? findNode(fs, b.dir) : null;
    node?.children && (node.children[b.name] = f(b.name, Math.floor(Math.random() * 90000) + 1024));
    return ok('ok');
  }
  if (seg[2] === 'files' && seg[3] === 'download') return ok('ok');

  // 日志
  if (seg[2] === 'logs' && seg.length === 3) {
    return ok({ logs: `Jun 01 10:23:01 ${seg[1]} CRON[1820]: (root) CMD (command -v debian-sa1 > /dev/null)\nJun 01 10:23:44 ${seg[1]} systemd[1]: Starting Daily apt download activities...\nJun 01 10:24:02 ${seg[1]} incus-agent[412]: heartbeat ok (0.8ms)` });
  }

  // 操作
  if (seg[0] === 'operations' && seg.length === 2) {
    const op = operations.get(seg[1]);
    return ok(op ?? { id: seg[1], status: 'Success', status_code: 200, description: '', created_at: '', updated_at: '', may_cancel: false, err: '' });
  }

  return fail(404, `mock 未实现: ${method} ${p}`);
}
