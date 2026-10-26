<script lang="ts">
  import { Radio, CircleCheck, Package, Boxes, CircleAlert } from '@lucide/svelte';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import EmptyState from '$lib/components/common/EmptyState.svelte';
  import TypeBadge from '$lib/components/common/TypeBadge.svelte';
  import { subscribeEvents } from '$lib/api';
  import type { EventMessage } from '$lib/types';
  import { onMount, onDestroy } from 'svelte';
  import { fade } from 'svelte/transition';

  const MAX = 200;
  let events = $state<(EventMessage & { _id: number })[]>([]);
  let connected = $state(false);
  let paused = $state(false);
  let filter = $state<'all' | 'instance' | 'operation' | 'event'>('all');
  let idSeq = 0;
  let off: (() => void) | null = null;
  let buffer: (EventMessage & { _id: number })[] = [];

  onMount(() => {
    off = subscribeEvents(
      (m) => {
        if (paused) return;
        buffer.push({ ...m, _id: ++idSeq });
      },
      (ok) => {
        connected = ok;
      },
    );
    // 批量合并渲染，避免高频事件卡顿
    const t = setInterval(() => {
      if (buffer.length === 0) return;
      const incoming = buffer.reverse();
      buffer = [];
      events = [...incoming, ...events].slice(0, MAX);
    }, 400);
    return () => {
      clearInterval(t);
      off?.();
    };
  });

  onDestroy(() => off?.());

  const filtered = $derived(filter === 'all' ? events : events.filter((e) => e.type === filter));

  function timeOf(ts: string): string {
    try {
      return new Date(ts).toLocaleTimeString('zh-CN', { hour12: false });
    } catch {
      return ts;
    }
  }

  function typeIcon(t: string) {
    if (t === 'instance') return Boxes;
    if (t === 'operation') return Package;
    return Radio;
  }

  const counts = $derived({
    instance: events.filter((e) => e.type === 'instance').length,
    operation: events.filter((e) => e.type === 'operation').length,
    event: events.filter((e) => e.type === 'event').length,
  });
</script>

<div class="animate-fade-up space-y-4">
  <div class="flex flex-wrap items-center gap-2">
    <div class="flex items-center gap-1.5 rounded-lg border border-border/60 bg-card px-2 py-1 shadow-soft">
      <span class="relative flex h-[7px] w-[7px]">
        {#if connected}
          <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-success opacity-50"></span>
        {/if}
        <span class="relative inline-flex h-[7px] w-[7px] rounded-full {connected ? 'bg-success' : 'bg-warning'}"></span>
      </span>
      <span class="text-[11.5px] text-muted-foreground">{connected ? '事件流已连接' : '等待事件…'}</span>
    </div>

    <div class="flex items-center gap-1">
      {#each [['all', '全部'], ['instance', `实例 (${counts.instance})`], ['operation', `操作 (${counts.operation})`], ['event', `事件 (${counts.event})`]] as [val, label] (val)}
        <button
          class="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors {filter === val
            ? 'bg-accent text-accent-foreground'
            : 'text-muted-foreground hover:bg-foreground/[0.04] hover:text-foreground'}"
          onclick={() => (filter = val as typeof filter)}
        >
          {label}
        </button>
      {/each}
    </div>

    <div class="ml-auto flex items-center gap-2">
      <Button size="sm" variant="outline" class="h-8 text-[12px]" onclick={() => (paused = !paused)}>
        {paused ? '继续滚动' : '暂停滚动'}
      </Button>
      <Button size="sm" variant="ghost" class="h-8 text-[12px] text-muted-foreground" onclick={() => (events = [])}>
        清空
      </Button>
    </div>
  </div>

  <Card class="shadow-soft">
    {#if filtered.length === 0}
      <div class="p-5">
        <EmptyState
          title={paused ? '已暂停' : '等待事件'}
          desc={paused ? '点击「继续滚动」恢复实时接收。' : '仅实时显示本页打开后发生的事件（启停、快照、文件、操作等），空闲中的容器不会产生事件。'}
        >
          {#snippet icon()}<Radio size={20} />{/snippet}
        </EmptyState>
      </div>
    {:else}
      <ul class="max-h-[calc(100svh-240px)] divide-y divide-border/40 overflow-y-auto">
        {#each filtered as e (e._id)}
          <li class="flex items-start gap-3 px-5 py-3 transition-colors hover:bg-foreground/[0.02]" in:fade={{ duration: 120 }}>
            <span class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg {e.status === 'failure' ? 'bg-destructive/10 text-destructive' : e.type === 'instance' ? 'bg-primary/10 text-primary' : 'bg-info/10 text-info'}">
              {#if e.status === 'failure'}
                <CircleAlert size={14} />
              {:else if e.type === 'instance'}
                <Boxes size={14} />
              {:else if e.type === 'operation'}
                <Package size={14} />
              {:else}
                <CircleCheck size={14} />
              {/if}
            </span>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-[13px] font-medium">{e.payload?.instance ?? e.payload?.description ?? e.type}</span>
                <TypeBadge label={e.payload?.action ?? e.status} tone={e.status === 'failure' ? 'warning' : e.type === 'instance' ? 'info' : 'default'} />
                <TypeBadge label={e.type} />
              </div>
              {#if e.payload?.operation_id}
                <div class="mt-0.5 truncate font-mono text-[10.5px] text-muted-foreground">op: {e.payload.operation_id}</div>
              {/if}
            </div>
            <span class="font-num shrink-0 text-[10.5px] text-muted-foreground/70">{timeOf(e.timestamp)}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </Card>

  <p class="text-center text-[11px] text-muted-foreground">
    通过 SSE 订阅 /api/events，最多保留最近 {MAX} 条 · 每类事件：<Badge variant="secondary" class="mx-0.5 px-1.5 text-[10px]">instance</Badge>
    实例 · <Badge variant="secondary" class="mx-0.5 px-1.5 text-[10px]">operation</Badge> 异步操作
  </p>
</div>
