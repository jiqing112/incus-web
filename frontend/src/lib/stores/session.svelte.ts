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
    const info = await api.serverInfo();
    session.authed = true;
    session.serverName = info.environment.server || 'incus';
    session.serverVersion = info.environment.server_version || '';
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
