/** 仪表盘实时指标缓存：跨路由保留采样点，回到首页时立即可见而非重新等待。 */
import type { ServerResources } from '$lib/types';

export interface MetricPoint {
  t: number;
  rx: number;
  tx: number;
}

const MAX = 40;

export const metrics = $state<{
  samples: MetricPoint[];
  cpuTrend: number[];
  cpuNow: number;
  memNow: number;
  prevCpu: { usage: number; ts: number } | null;
}>({
  samples: [],
  cpuTrend: [],
  cpuNow: 0,
  memNow: 0,
  prevCpu: null,
});

/** 采集一次资源指标：基于相邻两次采样差值计算 CPU 使用率。 */
export function sampleMetrics(r: ServerResources) {
  const now = Date.now();
  let cpuPct = 0;
  const prev = metrics.prevCpu;
  if (prev) {
    const dNs = r.cpu.usage - prev.usage;
    const dS = Math.max((now - prev.ts) / 1000, 0.5);
    cpuPct = Math.min(100, (dNs / (dS * 1e9 * Math.max(r.cpu.total_cores, 1))) * 100);
  }
  metrics.prevCpu = { usage: r.cpu.usage, ts: now };
  metrics.cpuNow = cpuPct;
  metrics.memNow = (r.memory.used / r.memory.total) * 100;
  metrics.cpuTrend = [...metrics.cpuTrend.slice(-(MAX - 1)), Math.round(cpuPct)];
  metrics.samples = [...metrics.samples.slice(-(MAX - 1)), { t: now, rx: cpuPct, tx: metrics.memNow }];
}
