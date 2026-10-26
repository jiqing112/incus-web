<script lang="ts">
  import { getContext } from 'svelte';

  let {
    value,
    disabled = false,
    class: className = '',
    children,
  }: {
    value: string;
    disabled?: boolean;
    class?: string;
    children?: import('svelte').Snippet;
  } = $props();

  const root = getContext<{
    value: string;
    select: (v: string) => void;
    orientation: 'horizontal' | 'vertical';
  }>('incus-tabs');

  const isActive = $derived(root.value === value);
</script>

<button
  type="button"
  role="tab"
  data-slot="tabs-trigger"
  data-state={isActive ? 'active' : 'inactive'}
  aria-selected={isActive}
  {disabled}
  class="gap-1.5 rounded-md border border-transparent px-1.5 py-0.5 text-sm font-medium relative inline-flex h-[calc(100%_-_1px)] flex-1 items-center justify-center whitespace-nowrap text-foreground/60 transition-[color,background-color,border-color,box-shadow,transform,opacity] hover:text-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50 dark:text-muted-foreground dark:hover:text-foreground data-active:bg-background data-active:text-foreground dark:data-active:bg-input/30 dark:data-active:text-foreground {className}"
  onclick={() => root.select(value)}
>
  {@render children?.()}
</button>
