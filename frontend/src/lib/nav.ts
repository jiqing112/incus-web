/** 侧栏导航结构。 */
import {
  LayoutDashboard,
  Radio,
  Boxes,
  IdCard,
  Disc3,
  HardDrive,
  Network,
  Stethoscope,
  type LucideIcon,
} from '@lucide/svelte';

export interface NavItem {
  path: string;
  label: string;
  icon: LucideIcon;
}

export interface NavGroup {
  label: string;
  items: NavItem[];
}

export const navGroups: NavGroup[] = [
  {
    label: '可观测',
    items: [
      { path: '/dashboard', label: '仪表盘', icon: LayoutDashboard },
      { path: '/events', label: '活动日志', icon: Radio },
    ],
  },
  {
    label: '工作负载',
    items: [
      { path: '/instances', label: '实例', icon: Boxes },
      { path: '/profiles', label: '配置文件', icon: IdCard },
    ],
  },
  {
    label: '镜像与存储',
    items: [
      { path: '/images', label: '镜像', icon: Disc3 },
      { path: '/storage', label: '存储池', icon: HardDrive },
    ],
  },
  {
    label: '网络',
    items: [
      { path: '/networks', label: '网络', icon: Network },
      { path: '/network-diagnostics', label: '网络诊断', icon: Stethoscope },
    ],
  },
];

export const navTitleMap: Record<string, { title: string; sub: string }> = {
  '/dashboard': { title: '仪表盘', sub: '服务器运行概览与实时资源' },
  '/instances': { title: '实例', sub: '容器与虚拟机生命周期管理' },
  '/images': { title: '镜像', sub: '本地镜像 · 远程导入 · 导出' },
  '/networks': { title: '网络', sub: '网桥管理 · DHCP 租约 · 引用关系' },
  '/storage': { title: '存储池', sub: '池与卷管理 · 空间占用' },
  '/profiles': { title: '配置文件', sub: '配置模板 · 设备映射' },
  '/events': { title: '活动日志', sub: '实时事件流 · 操作审计' },
  '/network-diagnostics': { title: '网络诊断', sub: '转发链 · Docker 冲突 · 连通性检测与一键修复' },
};
