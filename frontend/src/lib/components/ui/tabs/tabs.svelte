<script lang="ts">
  /**
   * 轻量受控 Tabs（签名兼容 shadcn-svelte 用法）：
   * <Tabs.Root value onValueChange> <Tabs.List> <Tabs.Trigger value> <Tabs.Content value>
   */
  import { setContext } from 'svelte';

  let {
    value = $bindable(''),
    onValueChange,
    orientation = 'horizontal',
    class: className = '',
    children,
  }: {
    value?: string;
    onValueChange?: (v: string) => void;
    orientation?: 'horizontal' | 'vertical';
    class?: string;
    children?: import('svelte').Snippet;
  } = $props();

  setContext('incus-tabs', {
    get value() {
      return value;
    },
    select: (v: string) => {
      if (v === value) return;
      value = v;
      onValueChange?.(v);
    },
    get orientation() {
      return orientation;
    },
  });
</script>

<div data-slot="tabs" data-orientation={orientation} class="gap-2 group/tabs flex data-horizontal:flex-col {className}">
  {@render children?.()}
</div>
