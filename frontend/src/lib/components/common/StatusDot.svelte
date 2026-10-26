<script lang="ts">
  /** 状态圆点：实例/资源运行状态。 */
  let {
    state,
    label,
    ping = true,
  }: {
    state: 'active' | 'stopped' | 'pending' | 'error';
    label?: string;
    ping?: boolean;
  } = $props();

  const dot: Record<string, string> = {
    active: 'bg-success',
    stopped: 'bg-muted-foreground/40',
    pending: 'bg-warning',
    error: 'bg-destructive',
  };
</script>

<span class="inline-flex items-center gap-1.5" title={label}>
  <span class="relative flex h-[7px] w-[7px]">
    {#if state === 'active' && ping}
      <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-success opacity-50"></span>
    {/if}
    <span class="relative inline-flex h-[7px] w-[7px] rounded-full {dot[state]}"></span>
  </span>
  {#if label}
    <span class="text-xs {state === 'stopped' ? 'text-muted-foreground' : 'text-foreground/80'}">{label}</span>
  {/if}
</span>
