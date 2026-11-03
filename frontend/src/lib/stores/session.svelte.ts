/** 会话状态：Token 校验与登录跳转。 */
import { api, getToken, setToken, setUnauthorizedHandler, clearToken } from '$lib/api';
import { navigate } from '$lib/router.svelte';

export const session = $state({
  authed: false,
  checking: false,
  /** 服务器名称（如 incus 6.0.2），登录后填充 */
  serverName: '',
  serverVersion: '',
});

export async function loginWithToken(token: string): Promise<void> {
  setToken(token.trim());
  await verify(true);
}

export async function verify(throwOnError = false): Promise<boolean> {
  if (!getToken()) {
    session.authed = false;
    return false;
  }
  session.checking = true;
  try {
    // 登录校验只验证 token 本身（/auth/verify 不依赖 Incus）。
    // 若复用 /server/info，未安装或未启动 Incus 的全新主机将永远无法登录，
    // 安装向导也就无从进入；Incus 不可达时放行，由安装向导接管。
    const res = await api.authVerify();
    session.authed = true;
    if (res.incus_reachable) {
      // 服务器名称/版本仅用于展示，取不到不影响登录状态
      try {
        const info = await api.serverInfo();
        session.serverName = info.environment.server || 'incus';
        session.serverVersion = info.environment.server_version || '';
      } catch {
        /* 忽略 */
      }
    }
    return true;
  } catch (e) {
    session.authed = false;
    if (throwOnError) {
      clearToken();
      throw e;
    }
    return false;
  } finally {
    session.checking = false;
  }
}

export function logout() {
  clearToken();
  session.authed = false;
  navigate('/login', true);
}

/** 401 时由 api 层回调：回登录页 */
setUnauthorizedHandler(() => {
  session.authed = false;
  navigate('/login', true);
});
