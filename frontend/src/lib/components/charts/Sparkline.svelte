<script lang="ts">
  /** 迷你走势线：用于统计卡片底部。 */
  let { values, color = 'var(--chart-1)' }: { values: number[]; color?: string } = $props();

  const W = 96;
  const H = 30;

  const min = $derived(Math.min(...values));
  const max = $derived(Math.max(...values));
  const path = $derived.by(() => {
    if (values.length < 2) return '';
    const pts = values.map(
      (v, i) =>
        [
          (i / (values.length - 1)) * W,
          H - 3 - ((v - min) / Math.max(max - min, 1)) * (H - 6),
        ] as const,
    );
    return pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p[0].toFixed(1)},${p[1].toFixed(1)}`).join(' ');
  });
</script>

<svg width={W} height={H} viewBox="0 0 {W} {H}" aria-hidden="true">
  <path d={path} fill="none" stroke={color} stroke-width="1.6" stroke-linecap="round" opacity="0.85" />
</svg>
