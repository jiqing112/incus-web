<script lang="ts">
  /** 设置面板包装层：隔离构建器的导入分析问题。 */
  import SettingsTab from './SettingsTab.svelte';
  import type { InstanceInfo } from '$lib/types';

  let { name, instance, onreload }: { name: string; instance: InstanceInfo; onreload: () => void | Promise<void> } = $props();
</script>

<SettingsTab {name} {instance} {onreload} />
