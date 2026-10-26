<script lang="ts">
  import {
    Cpu, MemoryStick, HardDrive, Info, Globe, Server, Layers, CalendarClock,
    Power, ChevronDown, Boxes, Activity,
  } from '@lucide/svelte';
  import { fade } from 'svelte/transition';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import Gauge from '$lib/components/common/Gauge.svelte';
  import KeyValues from '$lib/components/common/KeyValues.svelte';
  import type { InstanceInfo } from '$lib/types';
  import { fmtBytes, fmtNum, relativeTime } from '$lib/utils';

  let { instance, onstart }: { instance: InstanceInfo; onstart?: () => void | Promise<void> } = $props();

  const st = $derived(instance.state);
  const running = $derived(instance.status === 'Running');
  const memPct = $derived(st && st.memory.total > 0 ? (st.memory.usage / st.memory.total) * 100 : 0);
  const diskEntry = $derived(st ? Object.entries(st.disk)[0] : null);
  const addrs = $derived.by(() => {
    if (!st?.network) return [];
    const out: { iface: string; family: string; address: string; scope: string }[] = [];
    for (const [iface, net] of Object.entries(st.network)) {
      for (const a of net.addresses ?? []) out.push({ iface, family: a.family, address: a.address, scope: a.scope });
    }
    return out;
  });
  const counters = $derived.by(() => {
    if (!st?.network) return [];
    return Object.entries(st.network)
      .filter(([iface]) => iface !== 'lo')
      .map(([iface, net]) => ({ iface, ...net.counters }));
  });

  /* 系统信息摘要 */
  const osLabel = $derived(
    [instance.config?.['image.os'], instance.config?.['image.release']].filter(Boolean).join(' ') || instance.architecture,
  );
  const imageDesc = $derived(instance.config?.['image.description'] ?? '');

  /* 配置分组：image.* 镜像信息 / volatile.* 运行时状态（默认折叠）/ 其余为配置项 */
  let showVolatile = $state(false);
  const configGroups = $derived.by(() => {
    const image: [string, string][] = [];
    const volatileKV: [string, string][] = [];
    const main: [string, string][] = [];
    for (const [k, v] of Object.entries(instance.config ?? {})) {
      if (k.startsWith('image.')) image.push([k, v]);
      else if (k.startsWith('volatile.')) volatileKV.push([k, v]);
      else main.push([k, v]);
    }
    return { image, volatile: volatileKV, main };
  });
  const deviceEntries = $derived(Object.entries(instance.devices ?? {}));

  /* 纳秒 → 人类可读时长（Incus 未提供 usage_str 时的回退） */
  function fmtCpuTime(ns: number): string {
    const s = ns / 1e9;
    if (s < 60) return `${s.toFixed(1)} 秒`;
    const m = Math.floor(s / 60);
    if (m < 60) return `${m} 分 ${Math.floor(s % 60)} 秒`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h} 小时 ${m % 60} 分`;
    return `${Math.floor(h / 24)} 天 ${h % 24} 小时`;
  }
  const cpuTimeLabel = $derived(st ? (st.cpu.usage_str || fmtCpuTime(st.cpu.usage)) : '—');
</script>

<div class="animate-fade-up space-y-4">
  <!-- 系统信息摘要带 -->
  <Card class="p-4 shadow-soft">
    <dl class="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-3 xl:grid-cols-5">
      <div class="min-w-0">
        <dt class="flex items-center gap-1.5 text-[11px] text-muted-foreground"><Server size={12} />发行版</dt>
        <dd class="mt-1 truncate text-[13px] font-medium" title={imageDesc || osLabel}>{osLabel}</dd>
      </div>
      <div class="min-w-0">
        <dt class="flex items-center gap-1.5 text-[11px] text-muted-foreground"><Boxes size={12} />镜像</dt>
        <dd class="mt-1 truncate font-mono text-[12.5px] font-medium" title={imageDesc}>{imageDesc || instance.config?.['image.serial'] || '—'}</dd>
      </div>
      <div class="min-w-0">
        <dt class="flex items-center gap-1.5 text-[11px] text-muted-foreground"><Cpu size={12} />架构</dt>
        <dd class="mt-1 font-mono text-[12.5px] font-medium">{instance.architecture}</dd>
      </div>
      <div class="min-w-0">
        <dt class="flex items-center gap-1.5 text-[11px] text-muted-foreground"><CalendarClock size={12} />创建 / 最近使用</dt>
        <dd class="font-num mt-1 truncate text-[12.5px] font-medium">
          {relativeTime(instance.created_at)} <span class="text-muted-foreground/60">·</span> {relativeTime(instance.last_used_at)}
        </dd>
      </div>
      <div class="min-w-0">
        <dt class="flex items-center gap-1.5 text-[11px] text-muted-foreground"><Layers size={12} />配置文件</dt>
        <dd class="mt-1 flex flex-wrap gap-1">
          {#each instance.profiles as p (p)}
            <Badge variant="secondary" class="px-1.5 py-0 text-[10.5px] font-normal">{p}</Badge>
          {/each}
        </dd>
      </div>
    </dl>
  </Card>

  <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
    <!-- 左列（2/3）：资源 + 网络 -->
    <div class="space-y-4 xl:col-span-2">
      {#if running && st}
        <Card class="p-5 shadow-soft">
          <div class="mb-4 flex items-center gap-2">
            <h2 class="text-sm font-semibold tracking-tight">资源占用</h2>
            <Badge variant="outline" class="ml-auto bg-foreground/[0.04] text-[10.5px] text-muted-foreground ring-border">
              <Activity size={10} class="mr-1" />实时
            </Badge>
          </div>
          <div class="grid grid-cols-3 gap-2 py-2">
            <Gauge
              value={st.memory.total > 0 ? memPct : 0}
              label={st.memory.total > 0 ? '内存' : '内存（未限制）'}
              sub={fmtBytes(st.memory.usage, 1)}
            />
            <div class="flex flex-col items-center justify-center gap-2">
              <span class="flex h-[52px] items-center text-chart-3"><HardDrive size={30} strokeWidth={1.6} /></span>
              <div class="text-center">
                <div class="text-[11.5px] font-medium">根磁盘已用</div>
                <div class="font-num mt-0.5 text-[15px] font-semibold">{diskEntry ? fmtBytes(diskEntry[1].usage, 1) : '—'}</div>
              </div>
            </div>
            <div class="flex flex-col items-center justify-center gap-2">
              <span class="flex h-[52px] items-center text-chart-1"><Cpu size={30} strokeWidth={1.6} /></span>
              <div class="text-center">
                <div class="text-[11.5px] font-medium">进程数</div>
                <div class="font-num mt-0.5 text-[15px] font-semibold">{fmtNum(st.processes)}</div>
              </div>
            </div>
          </div>
          <dl class="mt-4 grid grid-cols-1 gap-x-8 gap-y-2.5 border-t border-border/60 pt-4 text-[12.5px] sm:grid-cols-2">
            <div class="flex items-center justify-between gap-4">
              <dt class="flex items-center gap-1.5 text-muted-foreground"><Cpu size={12} />累计 CPU 时间</dt>
              <dd class="font-num font-medium">{cpuTimeLabel}</dd>
            </div>
            <div class="flex items-center justify-between gap-4">
              <dt class="flex items-center gap-1.5 text-muted-foreground"><MemoryStick size={12} />内存上限</dt>
              <dd class="font-num font-medium">{st.memory.total > 0 ? fmtBytes(st.memory.total, 1) : '未限制'}</dd>
            </div>
          </dl>
        </Card>

        <Card class="p-5 shadow-soft">
          <div class="mb-4 flex items-center gap-2">
            <h2 class="text-sm font-semibold tracking-tight">网络</h2>
            {#if addrs.length > 0}
              <Badge variant="outline" class="ml-auto bg-success/8 text-[10.5px] text-success ring-success/20">{addrs.length} 个地址</Badge>
            {/if}
          </div>
          <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div class="space-y-2">
              {#each addrs as a (a.iface + a.address)}
                <div class="flex items-center gap-2.5 rounded-lg border border-border/60 px-3 py-2">
                  <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-info/10 text-info"><Globe size={13} /></span>
                  <div class="min-w-0 flex-1">
                    <div class="truncate font-mono text-[12.5px] font-medium">{a.address}</div>
                    <div class="text-[10.5px] text-muted-foreground">{a.iface} · {a.family} · {a.scope}</div>
                  </div>
                </div>
              {/each}
            </div>
            {#if counters.length > 0}
              <div class="flex flex-col justify-center">
                <div class="mb-2.5 text-[11.5px] text-muted-foreground">流量计数</div>
                {#each counters as c (c.iface)}
                  <div class="mb-3 last:mb-0">
                    <div class="mb-1 flex justify-between text-[11.5px]">
                      <span class="font-mono">{c.iface}</span>
                      <span class="font-num text-muted-foreground">↓ {fmtBytes(c.bytes_received, 0)} · ↑ {fmtBytes(c.bytes_sent, 0)}</span>
                    </div>
                    <div class="flex h-[5px] gap-0.5 overflow-hidden rounded-full bg-foreground/[0.05]">
                      <div class="h-full bg-chart-2" style="width:{(c.bytes_received / Math.max(c.bytes_received + c.bytes_sent, 1)) * 100}%"></div>
                      <div class="h-full bg-chart-1" style="width:{(c.bytes_sent / Math.max(c.bytes_received + c.bytes_sent, 1)) * 100}%"></div>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </Card>
      {:else}
        <!-- 停止态：单一引导卡，不再留两块空白 -->
        <Card class="shadow-soft">
          <div class="flex flex-col items-center justify-center gap-3 px-6 py-12 text-center">
            <span class="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary"><Power size={20} /></span>
            <div class="text-[15px] font-medium">实例未运行</div>
            <p class="max-w-sm text-[12.5px] leading-relaxed text-muted-foreground">
              启动后可查看实时 CPU / 内存占用、网络地址与流量统计。配置与设备信息仍可在右侧随时查看。
            </p>
            <Button size="sm" class="mt-1 gap-1.5" onclick={() => onstart?.()}>
              <Power size={14} />
              启动实例
            </Button>
          </div>
        </Card>
      {/if}
    </div>

    <!-- 右列（1/3）：设备 + 配置 -->
    <div class="space-y-4">
      <Card class="p-5 shadow-soft">
        <div class="mb-4 flex items-center gap-2">
          <h2 class="text-sm font-semibold tracking-tight">设备</h2>
          <Badge variant="outline" class="ml-auto bg-foreground/[0.04] text-[10.5px] text-muted-foreground ring-border">{deviceEntries.length} 项</Badge>
        </div>
        {#if deviceEntries.length > 0}
          <div class="space-y-2">
            {#each deviceEntries as [devName, dev] (devName)}
              <div class="rounded-lg border border-border/60 px-3 py-2.5">
                <div class="flex items-center gap-2">
                  <span class="font-mono text-[12.5px] font-semibold">{devName}</span>
                  <Badge variant="outline" class="px-1.5 text-[10px] text-muted-foreground ring-border">{dev.type}</Badge>
                </div>
                <div class="mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[11px] text-muted-foreground">
                  {#each Object.entries(dev).filter(([k]) => k !== 'type') as [k, v] (k)}
                    <span><span class="text-muted-foreground/70">{k}</span> <span class="font-mono text-foreground/80">{v}</span></span>
                  {/each}
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <p class="rounded-lg border border-dashed border-border/70 px-3 py-4 text-center text-[12px] text-muted-foreground">
            无设备（继承自配置文件）
          </p>
        {/if}
      </Card>

      <Card class="p-5 shadow-soft">
        <div class="mb-4 flex items-center gap-2">
          <h2 class="text-sm font-semibold tracking-tight">配置</h2>
          <Badge variant="outline" class="ml-auto bg-foreground/[0.04] text-[10.5px] text-muted-foreground ring-border">
            {configGroups.main.length + configGroups.image.length + configGroups.volatile.length} 项
          </Badge>
        </div>

        <div class="space-y-4">
          {#if configGroups.main.length > 0}
            <section>
              <div class="mb-2 flex items-center gap-1.5 text-[11.5px] font-medium text-muted-foreground">
                <Info size={12} />配置项
              </div>
              <KeyValues entries={configGroups.main} emptyText="无配置项" />
            </section>
          {/if}

          {#if configGroups.image.length > 0}
            <section>
              <div class="mb-2 flex items-center gap-1.5 text-[11.5px] font-medium text-muted-foreground">
                <Boxes size={12} />镜像信息
              </div>
              <KeyValues entries={configGroups.image} />
            </section>
          {/if}

          {#if configGroups.volatile.length > 0}
            <section>
              <button
                type="button"
                class="mb-2 flex w-full items-center gap-1.5 rounded-md px-1 py-1 text-left text-[11.5px] font-medium text-muted-foreground transition-colors hover:bg-foreground/[0.03] hover:text-foreground"
                onclick={() => (showVolatile = !showVolatile)}
              >
                <ChevronDown size={12} class="transition-transform {showVolatile ? '' : '-rotate-90'}" />
                运行时状态（volatile）
                <Badge variant="outline" class="ml-auto px-1.5 text-[10px] text-muted-foreground ring-border">{configGroups.volatile.length}</Badge>
              </button>
              {#if showVolatile}
                <div in:fade={{ duration: 120 }}>
                  <KeyValues entries={configGroups.volatile} />
                </div>
              {/if}
            </section>
          {/if}

          {#if configGroups.main.length + configGroups.image.length + configGroups.volatile.length === 0}
            <p class="rounded-lg border border-dashed border-border/70 px-3 py-4 text-center text-[12px] text-muted-foreground">无配置项</p>
          {/if}
        </div>
      </Card>
    </div>
  </div>
</div>
