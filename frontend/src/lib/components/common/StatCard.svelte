<script lang="ts">
  import { TrendingUp, TrendingDown } from '@lucide/svelte';
  import { fmtNum } from '$lib/utils';

  let {
    label,
    value,
    unit,
    delta,
    icon,
    accent = 'primary',
  }: {
    label: string;
    value: string;
    /** 可选单位：以小一号字渲染在数值后，避免长值折行 */
    unit?: string;
    /** 相对变化百分数，正=上升 */
    delta?: number;
    icon?: import('svelte').Snippet;
    accent?: 'primary' | 'success' | 'warning' | 'destructive' | 'info';
  } = $props();

  const accents: Record<string, string> = {
    primary: 'bg-primary/10 text-primary',
    success: 'bg-success/10 text-success',
    warning: 'bg-warning/15 text-warning',
    destructive: 'bg-destructive/10 text-destructive',
    info: 'bg-info/10 text-info',
  };
</script>

<div
  class="lift-card group rounded-xl border border-border/70 bg-card p-5 transition-transform duration-200 hover:-translate-y-0.5"
>
  <div class="flex items-start justify-between gap-3">
    <div class="min-w-0">
      <div class="truncate text-[12.5px] font-medium text-muted-foreground">{label}</div>
      <div class="font-num mt-1.5 flex flex-wrap items-baseline gap-x-1.5 font-semibold tracking-tight">
        <!-- clamp：卡片宽度不足时数值整体缩放而不是折行 -->
        <span class="whitespace-nowrap text-[clamp(20px,2vw,26px)] leading-none">{value}</span>
        {#if unit}
          <span class="whitespace-nowrap text-[13px] leading-none text-muted-foreground">{unit}</span>
        {/if}
      </div>
    </div>
    {#if icon}
      <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg {accents[accent]}">
        {@render icon()}
      </div>
    {/if}
  </div>

  {#if delta !== undefined}
    <div class="mt-3 flex items-center gap-1 text-[11.5px]">
      {#if delta >= 0}
        <TrendingUp size={13} class="text-success" />
        <span class="font-num font-medium text-success">+{fmtNum(delta)}%</span>
      {:else}
        <TrendingDown size={13} class="text-destructive" />
        <span class="font-num font-medium text-destructive">{fmtNum(delta)}%</span>
      {/if}
      <span class="text-muted-foreground/80">较昨日</span>
    </div>
  {/if}

  <!-- 底部细节高亮线（transform 过渡，合成器直处理） -->
  <div class="absolute inset-x-3 bottom-0 h-[2px] origin-left scale-x-0 rounded-full bg-gradient-to-r from-primary/60 via-primary/20 to-transparent transition-transform duration-300 group-hover:scale-x-100"></div>
</div>
