/** 布局共享状态：当前页面标题 / 移动端菜单开合。 */

export const layout = $state({
  mobileMenuOpen: false,
});

/** 全局刷新信号：顶栏刷新按钮自增，页面 $effect 监听后重载 */
export const bus = $state({ refreshTick: 0, refreshedAt: 0 });

export function triggerRefresh() {
  bus.refreshTick++;
  bus.refreshedAt = Date.now();
}
