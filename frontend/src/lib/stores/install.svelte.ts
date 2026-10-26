/**
 * Incus 安装状态（App 与安装向导共享）。
 * 注意：request<T>() 已解包 ApiResponse.data，返回的就是状态对象本身。
 */
import { api } from '$lib/api';

export const installState = $state({
  /** 已完成过至少一次检测 */
  checked: false,
  /** 已安装且已初始化（检测失败时 fail-open 置 true，不劫持路由） */
  ready: true,
  /** 检测请求进行中 */
  loading: false,
});

/** 检测宿主机 incus 状态并同步到共享 store，返回 ready */
export async function refreshInstallStatus(): Promise<boolean> {
  if (installState.loading) return installState.ready;
  installState.loading = true;
  try {
    const res = await api.installStatus();
    installState.ready = Boolean(res?.installed && res?.initialized);
  } catch {
    // 检测失败时保守处理：不劫持用户路由（fail-open），面板内其他功能会自行报错
    installState.ready = true;
  } finally {
    installState.checked = true;
    installState.loading = false;
  }
  return installState.ready;
}
