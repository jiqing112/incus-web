<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Boxes, Cpu, MemoryStick, HardDrive, Activity, Power, RotateCw,
    SquareTerminal, Server, Layers, Timer, Zap, ChevronRight,
  } from '@lucide/svelte';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import StatCard from '$lib/components/common/StatCard.svelte';
  import StatusDot from '$lib/components/common/StatusDot.svelte';
  import TypeBadge from '$lib/components/common/TypeBadge.svelte';
  import EmptyState from '$lib/components/common/EmptyState.svelte';
  import TrafficArea from '$lib/components/charts/TrafficArea.svelte';
  import BarRank from '$lib/components/charts/BarRank.svelte';
  import Sparkline from '$lib/components/charts/Sparkline.svelte';
  import { api, subscribeEvents, pollOperation } from '$lib/api';
  import type { InstanceInfo, ServerInfo, ServerResources, EventMessage } from '$lib/types';
  import { fmtBytes, fmtNum } from '$lib/utils';
  import { toast } from 'svelte-sonner';
  import { bus } from '$lib/components/layout/state.svelte';
  import { navigate } from '$lib/router.svelte';
  import { openDetail } from '$lib/stores/detailModal.svelte';
  import { metrics, sampleMetrics } from '$lib/stores/dashboardMetrics.svelte';

  let info = $state<ServerInfo | null>(null);
  let res = $state<ServerResources | null>(null);
  let instances = $state<InstanceInfo[]>([]);
  let error = $state('');
  let events = $state<EventMessage[]>([]);

  /* 实时趋势采样（缓存于 store，跨路由保留） */
  let polling = false;

  async function loadAll() {
    error = '';
    try {
      const [i, r, inst] = await Promise.all([api.serverInfo(), api.serverResources(), api.instances()]);
      info = i;
      res = r;
      instances = inst;
      sampleMetrics(r);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function tick() {
    if (polling || document.hidden) return;
    polling = true;
    try {
      const r = await api.serverResources();
      sampleMetrics(r);
      res = r;
      instances = (await api.instances()) ?? [];
    } catch {
      /* 静默重试 */
    } finally {
      polling = false;
    }
  }

  onMount(() => {
    loadAll();
    // 首次快速补采一次，约 0.8s 后即可凑够两点绘制趋势
    const warm = setTimeout(() => void tick(), 800);
    const t = setInterval(() => void tick(), 3000);
    const off = subscribeEvents((m) => {
      events = [m, ...events].slice(0, 12);
    });
    return () => {
      clearTimeout(warm);
      clearInterval(t);
      off();
    };
  });

  // 顶栏刷新按钮
  $effect(() => {
    void bus.refreshTick;
    if (bus.refreshTick > 0) loadAll();
  });

  const running = $derived(instances.filter((i) => i.status === 'Running'));
  const memRank = $derived(
    running
      .map((i) => ({ label: i.name, value: i.state?.memory.usage ?? 0 }))
      .sort((a, b) => b.value - a.value)
      .slice(0, 6),
  );
  const memPct = $derived(res ? (res.memory.used / res.memory.total) * 100 : 0);
  const diskPct = $derived(res ? (res.disk.used / res.disk.total) * 100 : 0);

  /* 实例快捷操作 */
  let acting = $state('');
  async function doAction(name: string, action: 'start' | 'stop' | 'restart') {
    acting = name + action;
    try {
      const op = await api.setInstanceState(name, action);
      toast.info(`${action} ${name}…`, { description: '操作已提交，等待 Incus 执行' });
      await pollOperation(op.operation_id);
      toast.success(`${name} ${action === 'start' ? '已启动' : action === 'stop' ? '已停止' : '已重启'}`);
      instances = (await api.instances()) ?? [];
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      acting = '';
    }
  }

</script>

<div class="animate-fade-up space-y-5">
  <!-- 统计卡片 -->
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
    <StatCard label="运行实例" value={info ? `${running.length}` : '—'} unit={info ? `/ ${instances.length}` : ''} accent="primary">
      {#snippet icon()}<Boxes size={17} strokeWidth={2} />{/snippet}
    </StatCard>
    <StatCard label="CPU 核心" value={res ? `${res.cpu.total_cores}` : '—'} unit={res ? 'cores' : ''} accent="info">
      {#snippet icon()}<Cpu size={17} strokeWidth={2} />{/snippet}
    </StatCard>
    <StatCard label="内存使用" value={res ? fmtBytes(res.memory.used, 1) : '—'} unit={res ? `/ ${fmtBytes(res.memory.total, 0)}` : ''} accent="success">
      {#snippet icon()}<MemoryStick size={17} strokeWidth={2} />{/snippet}
    </StatCard>
    <StatCard label="磁盘使用" value={res ? fmtBytes(res.disk.used, 1) : '—'} unit={res ? `/ ${fmtBytes(res.disk.total, 0)}` : ''} accent="warning">
      {#snippet icon()}<HardDrive size={17} strokeWidth={2} />{/snippet}
    </StatCard>
  </div>

  <!-- 趋势图 + 内存排行 -->
  <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
    <Card class="p-5 shadow-soft xl:col-span-2">
      <div class="mb-4 flex items-center justify-between gap-3">
        <div>
          <h2 class="text-sm font-semibold tracking-tight">资源实时趋势</h2>
          <p class="mt-0.5 text-xs text-muted-foreground">3 秒采样 · CPU / 内存占用率（%）</p>
        </div>
        <div class="flex items-center gap-4 text-[11.5px]">
          <span class="flex items-center gap-1.5">
            <span class="h-2 w-2 rounded-full bg-chart-1"></span>
            CPU
            <span class="font-num ml-0.5 text-muted-foreground">{metrics.cpuNow.toFixed(1)}%</span>
          </span>
          <span class="flex items-center gap-1.5">
            <span class="h-2 w-2 rounded-full bg-chart-2"></span>
            内存
            <span class="font-num ml-0.5 text-muted-foreground">{memPct.toFixed(1)}%</span>
          </span>
        </div>
      </div>
      {#if metrics.samples.length >= 2}
        <TrafficArea
          data={metrics.samples}
          height={264}
          uid="dash"
          name1="CPU"
          name2="内存"
          fmtY={(v) => `${Math.round(v)}%`}
        />
      {:else}
        <div class="animate-pulse rounded-lg bg-foreground/[0.04]" style="height:264px"></div>
      {/if}
    </Card>

    <Card class="p-5 shadow-soft">
      <div class="mb-4 flex items-center justify-between">
        <div>
          <h2 class="text-sm font-semibold tracking-tight">实例内存排行</h2>
          <p class="mt-0.5 text-xs text-muted-foreground">运行中实例 · 按常驻内存</p>
        </div>
      </div>
      {#if instances.length > 0}
        {#if memRank.length > 0}
          <BarRank items={memRank.map((r) => ({ label: r.label, value: r.value }))} fmt={(v) => fmtBytes(v, 1)} />
        {:else}
          <EmptyState compact title="暂无运行中的实例" desc="启动实例后即可看到内存占用排行。" />
        {/if}
      {:else}
        <div class="animate-pulse space-y-4 pt-2">
          {#each Array(5) as _, i (i)}
            <div class="h-8 rounded-lg bg-foreground/[0.04]"></div>
          {/each}
        </div>
      {/if}
      <div class="mt-5 border-t border-border/60 pt-4">
        <div class="mb-2 flex items-center justify-between text-[11.5px] text-muted-foreground">
          <span>CPU 占用走势</span>
          <span class="font-num">{metrics.cpuNow.toFixed(0)}%</span>
        </div>
        <Sparkline values={metrics.cpuTrend.length > 1 ? metrics.cpuTrend : [0, 0]} />
      </div>
    </Card>
  </div>

  <!-- 实例表 + 服务器信息/事件 -->
  <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
    <Card class="p-5 shadow-soft xl:col-span-2">
      <div class="mb-3 flex items-center justify-between">
        <div>
          <h2 class="text-sm font-semibold tracking-tight">实例概览</h2>
          <p class="mt-0.5 text-xs text-muted-foreground">全部实例与快捷操作</p>
        </div>
        <Button variant="ghost" size="sm" class="gap-1" onclick={() => navigate('/instances')}>
          全部实例 <ChevronRight size={14} />
        </Button>
      </div>
      {#if instances.length > 0}
        <div class="overflow-x-auto">
          <table class="w-full text-left text-[13px]">
            <thead>
              <tr class="border-b border-border/70 text-[11.5px] text-muted-foreground">
                <th class="py-2.5 pr-3 pl-1 font-medium">名称</th>
                <th class="py-2.5 pr-3 font-medium">状态</th>
                <th class="py-2.5 pr-3 font-medium">内存</th>
                <th class="py-2.5 pr-1 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {#each instances.slice(0, 6) as i (i.name)}
                <tr class="border-b border-border/40 transition-colors last:border-0 hover:bg-foreground/[0.02]">
                  <td class="py-2.5 pr-3 pl-1">
                    <button class="font-medium hover:text-primary" onclick={() => openDetail(i.name)}>
                      {i.name}
                    </button>
                    <span class="ml-2 align-middle"><TypeBadge label={i.type === 'virtual-machine' ? 'VM' : 'CT'} /></span>
                  </td>
                  <td class="py-2.5 pr-3">
                    <StatusDot
                      state={i.status === 'Running' ? 'active' : 'stopped'}
                      label={i.status === 'Running' ? '运行中' : '已停止'}
                      ping={false}
                    />
                  </td>
                  <td class="font-num py-2.5 pr-3 text-muted-foreground">
                    {i.state ? fmtBytes(i.state.memory.usage, 0) : '—'}
                  </td>
                  <td class="py-2.5 pr-1 text-right">
                    <div class="inline-flex items-center gap-1">
                      {#if i.status === 'Stopped'}
                        <Button variant="ghost" size="icon" class="size-7 text-muted-foreground" disabled={acting === i.name + 'start'} onclick={() => doAction(i.name, 'start')} aria-label="开机">
                          <Power size={13} class="text-success" />
                        </Button>
                      {:else}
                        <Button variant="ghost" size="icon" class="size-7 text-muted-foreground" disabled={acting === i.name + 'stop'} onclick={() => doAction(i.name, 'stop')} aria-label="关机">
                          <Power size={13} class="text-destructive" />
                        </Button>
                      {/if}
                      <Button variant="ghost" size="icon" class="size-7 text-muted-foreground" disabled={acting === i.name + 'restart'} onclick={() => doAction(i.name, 'restart')} aria-label="重启">
                        <RotateCw size={13} />
                      </Button>
                      <Button variant="ghost" size="icon" class="size-7 text-muted-foreground" onclick={() => openDetail(i.name, 'terminal')} aria-label="终端">
                        <SquareTerminal size={13} />
                      </Button>
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else if instances.length === 0 && !error}
        <EmptyState compact title="还没有实例" desc="在实例页创建第一个容器或虚拟机。" />
      {:else}
        <div class="animate-pulse space-y-3">
          {#each Array(4) as _, i (i)}
            <div class="h-9 rounded-lg bg-foreground/[0.04]"></div>
          {/each}
        </div>
      {/if}
    </Card>

    <div class="space-y-4">
      <Card class="p-5 shadow-soft">
        <h2 class="mb-4 text-sm font-semibold tracking-tight">服务器信息</h2>
        {#if info && res}
          <dl class="space-y-3.5 text-[13px]">
            <div class="flex items-center gap-3">
              <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary"><Server size={15} /></span>
              <div class="min-w-0 flex-1">
                <dt class="text-[11.5px] text-muted-foreground">发行版</dt>
                <dd class="truncate font-medium">{info.environment.os_name} {info.environment.os_version}</dd>
              </div>
              <Badge variant="outline" class="bg-success/10 text-success ring-success/20">
                {info.environment.server} {info.environment.server_version}
              </Badge>
            </div>
            <div class="flex items-center gap-3">
              <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-info/10 text-info"><Cpu size={15} /></span>
              <div class="min-w-0 flex-1">
                <dt class="text-[11.5px] text-muted-foreground">内核</dt>
                <dd class="truncate font-medium">{info.environment.kernel_version} · {info.environment.kernel_architecture}</dd>
              </div>
            </div>
            <div class="flex items-center gap-3">
              <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-success/10 text-success"><Layers size={15} /></span>
              <div class="min-w-0 flex-1">
                <dt class="text-[11.5px] text-muted-foreground">驱动</dt>
                <dd class="font-medium">{info.environment.driver}</dd>
              </div>
            </div>
            <div class="flex items-center gap-3">
              <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-warning/15 text-warning"><Timer size={15} /></span>
              <div class="min-w-0 flex-1">
                <dt class="text-[11.5px] text-muted-foreground">实时 CPU</dt>
                <dd class="font-num font-medium">{metrics.cpuNow.toFixed(1)}%</dd>
              </div>
            </div>
          </dl>
          <div class="mt-5 border-t border-border/60 pt-4">
            <div class="mb-2 flex items-center justify-between text-[11.5px] text-muted-foreground">
              <span>内存占用</span>
              <span class="font-num">{fmtBytes(res.memory.used, 0)} / {fmtBytes(res.memory.total, 0)}</span>
            </div>
            <div class="h-[6px] overflow-hidden rounded-full bg-foreground/[0.05]">
              <div class="h-full rounded-full bg-chart-2 transition-[width] duration-700 ease-out" style="width:{memPct}%"></div>
            </div>
            <div class="mb-2 mt-3 flex items-center justify-between text-[11.5px] text-muted-foreground">
              <span>磁盘占用</span>
              <span class="font-num">{fmtBytes(res.disk.used, 0)} / {fmtBytes(res.disk.total, 0)}</span>
            </div>
            <div class="h-[6px] overflow-hidden rounded-full bg-foreground/[0.05]">
              <div class="h-full rounded-full bg-chart-3 transition-[width] duration-700 ease-out" style="width:{diskPct}%"></div>
            </div>
          </div>
        {:else}
          <div class="animate-pulse space-y-4">
            {#each Array(4) as _, i (i)}
              <div class="h-10 rounded-lg bg-foreground/[0.04]"></div>
            {/each}
          </div>
        {/if}
      </Card>

      <Card class="p-5 shadow-soft">
        <div class="mb-3 flex items-center justify-between">
          <h2 class="text-sm font-semibold tracking-tight">实时活动</h2>
          <span class="flex items-center gap-1 text-[11px] text-muted-foreground">
            <Zap size={12} class="text-chart-2" /> SSE
          </span>
        </div>
        {#if events.length === 0}
          <p class="rounded-lg border border-dashed border-border/70 px-3 py-4 text-center text-[12px] text-muted-foreground">
            等待事件…
          </p>
        {:else}
          <ul class="space-y-2.5">
            {#each events.slice(0, 5) as m, idx (m.timestamp + m.type + idx)}
              <li class="flex items-start gap-2.5 text-[12px] leading-snug">
                <span class="mt-1.5 flex h-1.5 w-1.5 shrink-0 rounded-full {m.status === 'failure' ? 'bg-destructive' : m.type === 'instance' ? 'bg-chart-1' : 'bg-chart-2'}"></span>
                <div class="min-w-0 flex-1">
                  <span class="font-medium">{m.payload?.instance ?? m.payload?.description ?? m.type}</span>
                  <span class="text-muted-foreground">{m.payload?.action ?? m.status}</span>
                </div>
                <span class="font-num shrink-0 text-[10.5px] text-muted-foreground/70">
                  {new Date(m.timestamp).toLocaleTimeString('zh-CN', { hour12: false })}
                </span>
              </li>
            {/each}
          </ul>
        {/if}
      </Card>
    </div>
  </div>

  {#if error}
    <div class="flex items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
      <Activity size={15} />
      加载失败：{error}
      <Button variant="outline" size="sm" class="ml-auto" onclick={loadAll}>重试</Button>
    </div>
  {/if}
</div>
