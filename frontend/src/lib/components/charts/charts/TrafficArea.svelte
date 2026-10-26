<script lang="ts">
  /**
   * 双序列（入站/出站）渐变面积图。
   * 平滑曲线 + 悬停十字线 + 浮动提示，数据由父组件传入。
   */
  import { fmtBytes, fmtTime } from '$lib/utils';

  export interface Point {
    t: number;
    rx: number;
    tx: number;
  }

  let { data, height = 260 }: { data: Point[]; height?: number } = $props();

  let wrapW = $state(0);
  const PAD = { l: 46, r: 10, t: 12, b: 24 };
  const w = $derived(Math.max(wrapW, 320));
  const iw = $derived(w - PAD.l - PAD.r);
  const ih = $derived(height - PAD.t - PAD.b);

  let hover: number | null = $state(null);

  const yMax = $derived(niceMax(Math.max(...data.map((d) => Math.max(d.rx, d.tx)), 1)));

  function niceMax(v: number): number {
    const exp = Math.floor(Math.log10(v));
    const base = Math.pow(10, exp);
    const n = v / base;
    const nice = n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10;
    return nice * base;
  }

  const x = $derived((i: number) => PAD.l + (i / Math.max(data.length - 1, 1)) * iw);
  const y = $derived((v: number) => PAD.t + ih - (v / yMax) * ih);

  // catmull-rom → cubic bezier 平滑
  function smoothPath(values: number[], fy: (v: number) => number): string {
    const pts = values.map((v, i) => [x(i), fy(v)] as const);
    if (pts.length < 2) return '';
    let d = `M ${pts[0][0]},${pts[0][1]}`;
    for (let i = 0; i < pts.length - 1; i++) {
      const p0 = pts[Math.max(i - 1, 0)];
      const p1 = pts[i];
      const p2 = pts[i + 1];
      const p3 = pts[Math.min(i + 2, pts.length - 1)];
      const c1x = p1[0] + (p2[0] - p0[0]) / 6;
      const c1y = p1[1] + (p2[1] - p0[1]) / 6;
      const c2x = p2[0] - (p3[0] - p1[0]) / 6;
      const c2y = p2[1] - (p3[1] - p1[1]) / 6;
      d += ` C ${c1x.toFixed(1)},${c1y.toFixed(1)} ${c2x.toFixed(1)},${c2y.toFixed(1)} ${p2[0].toFixed(1)},${p2[1].toFixed(1)}`;
    }
    return d;
  }

  const rxPath = $derived(smoothPath(data.map((d) => d.rx), y));
  const txPath = $derived(smoothPath(data.map((d) => d.tx), y));

  const rxArea = $derived(
    rxPath ? `${rxPath} L ${x(data.length - 1)},${PAD.t + ih} L ${x(0)},${PAD.t + ih} Z` : '',
  );
  const txArea = $derived(
    txPath ? `${txPath} L ${x(data.length - 1)},${PAD.t + ih} L ${x(0)},${PAD.t + ih} Z` : '',
  );

  const gridVals = $derived([0.25, 0.5, 0.75, 1].map((f) => yMax * f));

  const xTicks = $derived.by(() => {
    const n = Math.min(6, data.length);
    const step = Math.floor(data.length / n);
    const ticks: { i: number; label: string }[] = [];
    for (let i = data.length - 1; i >= 0; i -= step) {
      ticks.push({ i, label: fmtTime(data[i].t).slice(0, 5) });
      if (ticks.length >= n) break;
    }
    return ticks;
  });

  function onMove(e: PointerEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const px = e.clientX - rect.left;
    const i = Math.round(((px - PAD.l) / iw) * (data.length - 1));
    hover = Math.min(Math.max(i, 0), data.length - 1);
  }

  const tipLeft = $derived(
    hover === null ? 0 : Math.min(Math.max(x(hover), 90), w - 90),
  );
</script>

<div
  class="relative w-full select-none"
  bind:clientWidth={wrapW}
  style="height:{height}px"
  onpointermove={onMove}
  onpointerleave={() => (hover = null)}
  role="img"
  aria-label="流量趋势图"
>
  <svg width={w} height={height} class="overflow-visible">
    <defs>
      <linearGradient id="fp-grad-rx" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" stop-color="var(--chart-1)" stop-opacity="0.28" />
        <stop offset="100%" stop-color="var(--chart-1)" stop-opacity="0.02" />
      </linearGradient>
      <linearGradient id="fp-grad-tx" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" stop-color="var(--chart-2)" stop-opacity="0.22" />
        <stop offset="100%" stop-color="var(--chart-2)" stop-opacity="0.02" />
      </linearGradient>
    </defs>

    <!-- 网格与 Y 轴 -->
    {#each gridVals as gv (gv)}
      <line x1={PAD.l} x2={w - PAD.r} y1={y(gv)} y2={y(gv)} class="stroke-border/60" stroke-dasharray="3 5" stroke-width="1" />
      <text x={PAD.l - 8} y={y(gv) + 3.5} text-anchor="end" class="fill-muted-foreground/70 font-num text-[10px]">
        {fmtBytes(gv, 1)}
      </text>
    {/each}
    <text x={PAD.l - 8} y={PAD.t + ih + 3.5} text-anchor="end" class="fill-muted-foreground/70 font-num text-[10px]">0</text>

    <!-- X 轴时间 -->
    {#each xTicks as tk (tk.i)}
      <text x={x(tk.i)} y={height - 6} text-anchor="middle" class="fill-muted-foreground/70 font-num text-[10px]">
        {tk.label}
      </text>
    {/each}

    <!-- 面积与线 -->
    <path d={rxArea} fill="url(#fp-grad-rx)" style="animation: fade-up .5s cubic-bezier(.16,1,.3,1) both" />
    <path d={txArea} fill="url(#fp-grad-tx)" style="animation: fade-up .5s .08s cubic-bezier(.16,1,.3,1) both" />
    <path
      d={rxPath}
      fill="none"
      stroke="var(--chart-1)"
      stroke-width="2"
      stroke-linecap="round"
      pathLength="1"
      class="fp-draw"
    />
    <path
      d={txPath}
      fill="none"
      stroke="var(--chart-2)"
      stroke-width="2"
      stroke-linecap="round"
      pathLength="1"
      class="fp-draw fp-draw-delay"
    />

    <!-- 十字线 -->
    {#if hover !== null}
      <line x1={x(hover)} x2={x(hover)} y1={PAD.t} y2={PAD.t + ih} class="stroke-foreground/25" stroke-width="1" stroke-dasharray="2 3" />
      <circle cx={x(hover)} cy={y(data[hover].rx)} r="3.5" fill="var(--chart-1)" stroke="var(--card)" stroke-width="1.5" />
      <circle cx={x(hover)} cy={y(data[hover].tx)} r="3.5" fill="var(--chart-2)" stroke="var(--card)" stroke-width="1.5" />
    {/if}
  </svg>

  <!-- 浮动提示 -->
  {#if hover !== null}
    <div
      class="pointer-events-none absolute top-2 z-10 min-w-[148px] -translate-x-1/2 rounded-lg border border-border/80 bg-popover p-2.5 shadow-lift"
      style="left:{tipLeft}px"
    >
      <div class="font-num mb-1.5 text-[10.5px] text-muted-foreground">
        {fmtTime(data[hover].t)}
      </div>
      <div class="flex items-center gap-1.5 text-[11.5px]">
        <span class="h-2 w-2 rounded-full bg-chart-1"></span>
        <span class="text-muted-foreground">入站</span>
        <span class="font-num ml-auto font-medium">{fmtBytes(data[hover].rx)}/s</span>
      </div>
      <div class="mt-1 flex items-center gap-1.5 text-[11.5px]">
        <span class="h-2 w-2 rounded-full bg-chart-2"></span>
        <span class="text-muted-foreground">出站</span>
        <span class="font-num ml-auto font-medium">{fmtBytes(data[hover].tx)}/s</span>
      </div>
    </div>
  {/if}
</div>

<style>
  @keyframes fp-draw {
    from {
      stroke-dashoffset: 1;
    }
    to {
      stroke-dashoffset: 0;
    }
  }
  .fp-draw {
    stroke-dasharray: 1;
    animation: fp-draw 0.9s cubic-bezier(0.4, 0, 0.2, 1) both;
  }
  .fp-draw-delay {
    animation-delay: 0.12s;
  }
</style>
