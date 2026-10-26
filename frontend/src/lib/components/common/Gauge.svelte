<script lang="ts">
  /**
   * 环形仪表：CPU / 内存 / 磁盘等占用率。
   * 0–100%，超过阈值渐进着色（80% 预警、92% 危险）。
   */
  let {
    value,
    label,
    sub,
    size = 92,
    stroke = 8,
  }: {
    /** 0–100 */
    value: number;
    label: string;
    sub?: string;
    size?: number;
    stroke?: number;
  } = $props();

  const r = $derived((size - stroke) / 2);
  const c = $derived(2 * Math.PI * r);
  const pct = $derived(Math.min(Math.max(value, 0), 100));
  const color = $derived(pct >= 92 ? 'var(--destructive)' : pct >= 80 ? 'var(--warning)' : 'var(--chart-1)');
</script>

<div class="flex flex-col items-center gap-2">
  <div class="relative" style="width:{size}px;height:{size}px">
    <svg width={size} height={size} class="-rotate-90">
      <circle cx={size / 2} cy={size / 2} {r} fill="none" stroke="var(--foreground)" stroke-opacity="0.06" stroke-width={stroke} />
      <circle
        cx={size / 2}
        cy={size / 2}
        {r}
        fill="none"
        stroke={color}
        stroke-width={stroke}
        stroke-linecap="round"
        stroke-dasharray={c}
        stroke-dashoffset={c * (1 - pct / 100)}
        style="transition: stroke-dashoffset .6s cubic-bezier(.16,1,.3,1), stroke .3s"
      />
    </svg>
    <div class="absolute inset-0 flex items-center justify-center">
      <span class="font-num text-[15px] font-semibold tracking-tight">{Math.round(pct)}<span class="text-[10px] font-normal text-muted-foreground">%</span></span>
    </div>
  </div>
  <div class="text-center">
    <div class="text-[11.5px] font-medium">{label}</div>
    {#if sub}
      <div class="font-num mt-0.5 text-[10.5px] text-muted-foreground">{sub}</div>
    {/if}
  </div>
</div>
