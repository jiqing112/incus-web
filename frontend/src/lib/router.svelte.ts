/**
 * 轻量 hash 路由（Svelte 5 runes）。
 * SPA 场景够用；页面以懒加载方式在 pages/index.ts 注册。
 */

export const router = $state<{ path: string }>({ path: '/' });

const listeners = new Set<() => void>();

export function navigate(path: string, replace = false) {
  if (replace) {
    history.replaceState(null, '', `#${path}`);
  } else {
    location.hash = path;
  }
  sync();
}

function sync() {
  const raw = location.hash.replace(/^#/, '') || '/dashboard';
  router.path = raw;
  listeners.forEach((fn) => fn());
}

export function onRouteChange(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function initRouter() {
  window.addEventListener('hashchange', sync);
  sync();
}

/** 解析 '/forwards?tab=black' 形式的路径 */
export function parsePath(path: string): { pathname: string; query: URLSearchParams } {
  const [pathname, qs] = path.split('?');
  return { pathname, query: new URLSearchParams(qs ?? '') };
}
