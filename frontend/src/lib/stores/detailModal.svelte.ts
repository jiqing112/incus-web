/** 实例详情模态框状态：列表/仪表盘点击实例名或终端按钮时打开。 */
export const detailModal = $state<{ name: string; tab: string; open: boolean }>({
  name: '',
  tab: 'overview',
  open: false,
});

export function openDetail(name: string, tab = 'overview') {
  detailModal.name = name;
  detailModal.tab = tab;
  detailModal.open = true;
}

export function closeDetail() {
  detailModal.open = false;
}
