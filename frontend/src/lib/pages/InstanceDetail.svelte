<script lang="ts">
  import {
    ArrowLeft, X, Power, RotateCw, SquareTerminal, Gauge, Camera, FileArchive, FolderOpen, Settings,
  } from '@lucide/svelte';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import StatusDot from '$lib/components/common/StatusDot.svelte';
  import TypeBadge from '$lib/components/common/TypeBadge.svelte';
  import OverviewTab from './detail/OverviewTab.svelte';
  import SnapshotsTab from './detail/SnapshotsTab.svelte';
  import BackupsTab from './detail/BackupsTab.svelte';
  import FilesTab from './detail/FilesTab.svelte';
  import TerminalTab from './detail/TerminalTab.svelte';
  import SettingsPanel from './detail/SettingsPanel.svelte';
  import { api, pollOperation } from '$lib/api';
  import type { InstanceInfo } from '$lib/types';
  import { toast } from 'svelte-sonner';
  import { router, navigate, parsePath } from '$lib/router.svelte';
  import { relativeTime } from '$lib/utils';

  let {
    name,
    onclose,
    syncUrl = true,
    initialTab = 'overview',
  }: {
    name: string;
    /** 提供时以模态框形态呈现：关闭回调 + 不写 URL */
    onclose?: () => void;
    syncUrl?: boolean;
    initialTab?: string;
  } = $props();

  const tabs = [
    { value: 'overview', label: '概览', icon: Gauge },
    { value: 'snapshots', label: '快照', icon: Camera },
    { value: 'backups', label: '备份', icon: FileArchive },
    { value: 'files', label: '文件', icon: FolderOpen },
    { value: 'terminal', label: '终端', icon: SquareTerminal },
    { value: 'settings', label: '设置', icon: Settings },
  ];

  let instance = $state<InstanceInfo | null>(null);
  let error = $state('');
  let acting = $state('');

  const query = $derived(parsePath(router.path).query);
  const tabValues = ['overview', 'snapshots', 'backups', 'files', 'terminal', 'settings'];
  let tab = $state('overview');
  $effect(() => {
    if (!syncUrl) {
      tab = initialTab;
      return;
    }
    const q = query.get('tab');
    tab = q && tabValues.includes(q) ? q : initialTab;
  });
  const urlTab = $derived(tab);

  async function load() {
    error = '';
    try {
      instance = await api.instance(name);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  $effect(() => {
    void name;
    load();
  });

  async function doAction(action: 'start' | 'stop' | 'restart') {
    acting = action;
    try {
      const op = await api.setInstanceState(name, action);
      toast.info(`${action} ${name}…`, { description: '操作已提交，等待 Incus 执行' });
      await pollOperation(op.operation_id);
      toast.success(`${name} ${action === 'start' ? '已启动' : action === 'stop' ? '已停止' : '已重启'}`);
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      acting = '';
    }
  }

  function switchTab(v: string | number) {
    tab = String(v);
    if (syncUrl) navigate(`/instances/${name}?tab=${v}`, true);
  }
</script>

{#snippet tabBar()}
  <div class="inline-flex items-center gap-0.5 rounded-xl border border-border/60 bg-card p-1 shadow-soft">
    {#each tabs as tb (tb.value)}
      <button
        type="button"
        role="tab"
        aria-selected={urlTab === tb.value}
        class="flex items-center gap-1.5 rounded-lg px-3.5 py-1.5 text-[13px] font-medium transition-all duration-150 {urlTab === tb.value
          ? 'bg-primary/10 text-primary ring-1 ring-inset ring-primary/20'
          : 'text-muted-foreground hover:bg-foreground/[0.04] hover:text-foreground'}"
        onclick={() => switchTab(tb.value)}
      >
        {#if urlTab === tb.value}
          <span class="relative flex h-[6px] w-[6px]">
            <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary opacity-40"></span>
            <span class="relative inline-flex h-[6px] w-[6px] rounded-full bg-primary"></span>
          </span>
        {:else}
          <tb.icon size={14} strokeWidth={1.9} />
        {/if}
        {tb.label}
      </button>
    {/each}
  </div>
{/snippet}

<div class="animate-fade-up flex h-full min-h-0 flex-col gap-5">
  <!-- 头部 -->
  <div class="relative flex flex-wrap items-center gap-3">
    <Button
      variant="ghost"
      size="icon"
      class="size-8 shrink-0 text-muted-foreground"
      onclick={() => (onclose ? onclose() : navigate('/instances'))}
      aria-label={onclose ? '关闭' : '返回'}
    >
      {#if onclose}<X size={17} />{:else}<ArrowLeft size={17} />{/if}
    </Button>
    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2.5">
        <h2 class="truncate font-mono text-[17px] font-semibold tracking-tight">{name}</h2>
        {#if instance}
          <StatusDot state={instance.status === 'Running' ? 'active' : 'stopped'} label={instance.status === 'Running' ? '运行中' : '已停止'} />
          <TypeBadge label={instance.type === 'virtual-machine' ? 'VM' : 'CT'} />
          <Badge variant="outline" class="px-1.5 font-mono text-[10.5px] text-muted-foreground">{instance.architecture}</Badge>
        {/if}
      </div>
      {#if instance}
        <p class="mt-0.5 text-xs text-muted-foreground">
          创建于 {relativeTime(instance.created_at)} · 镜像 {instance.config?.['image.os'] ?? '—'} {instance.config?.['image.release'] ?? ''}
        </p>
      {/if}
    </div>

    <div class="flex items-center gap-1.5">
      {#if instance?.status === 'Stopped'}
        <Button size="sm" variant="outline" class="h-8.5 gap-1.5" disabled={acting !== ''} onclick={() => doAction('start')} title="开机">
          <Power size={14} class="text-success" /> 开机
        </Button>
      {:else if instance}
        <Button size="sm" variant="outline" class="h-8.5 gap-1.5" disabled={acting !== ''} onclick={() => doAction('stop')} title="关机">
          <Power size={14} class="text-destructive" /> 关机
        </Button>
      {/if}
      {#if instance}
        <Button size="sm" variant="outline" class="h-8.5 gap-1.5" disabled={acting !== ''} onclick={() => doAction('restart')}>
          <RotateCw size={14} /> 重启
        </Button>
      {/if}
    </div>

    <!-- 大屏：tab 栏悬浮于头部正中 -->
    {#if instance && !error}
      <div class="pointer-events-none absolute inset-y-0 left-1/2 hidden -translate-x-1/2 items-center xl:flex">
        <div class="pointer-events-auto">{@render tabBar()}</div>
      </div>
    {/if}
  </div>

  <div class="min-h-0 flex-1 overflow-y-auto">
    {#if error}
      <div class="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
        加载失败：{error}
        <Button variant="outline" size="sm" class="ml-3" onclick={load}>重试</Button>
      </div>
    {:else if !instance}
      <div class="space-y-3">
        {#each Array(3) as _, i (i)}
          <div class="h-16 animate-pulse rounded-xl bg-foreground/[0.04]"></div>
        {/each}
      </div>
    {:else}
      <!-- 窄屏：tab 栏落到内容区顶部 -->
      <div class="xl:hidden">{@render tabBar()}</div>

      <div class="mt-4">
        {#if urlTab === 'overview'}
          <OverviewTab instance={instance} onstart={() => doAction('start')} />
        {:else if urlTab === 'snapshots'}
          <SnapshotsTab name={name} />
        {:else if urlTab === 'backups'}
          <BackupsTab name={name} />
        {:else if urlTab === 'files'}
          <FilesTab name={name} running={instance.status === 'Running'} />
        {:else if urlTab === 'terminal'}
          <TerminalTab name={name} />
        {:else if urlTab === 'settings'}
          <SettingsPanel {name} instance={instance} onreload={load} />
        {/if}
      </div>
    {/if}
  </div>
</div>
