<script lang="ts">
  import { onMount } from 'svelte';
  import { Camera, History, Trash2, Plus, Clock, Pencil, CalendarClock, Save, Info } from '@lucide/svelte';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Label } from '$lib/components/ui/label';
  import { Badge } from '$lib/components/ui/badge';
  import * as Dialog from '$lib/components/ui/dialog';
  import { toast } from 'svelte-sonner';
  import EmptyState from '$lib/components/common/EmptyState.svelte';
  import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
  import { api, pollOperation } from '$lib/api';
  import type { SnapshotInfo, ServerFeatures } from '$lib/types';
  import { fmtDateTime, relativeTime } from '$lib/utils';
  import { bus } from '$lib/components/layout/state.svelte';

  let { name }: { name: string } = $props();

  let snaps = $state<SnapshotInfo[]>([]);
  let loading = $state(true);

  async function load() {
    loading = true;
    try {
      snaps = await api.snapshots(name);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
    void loadSchedule();
    void loadFeatures();
  });
  $effect(() => {
    void bus.refreshTick;
    if (bus.refreshTick > 0) load();
  });

  function expiryLabel(expires: string): string {
    if (!expires) return '';
    const t = new Date(expires).getTime();
    if (Number.isNaN(t)) return '';
    const diffS = Math.round((t - Date.now()) / 1000);
    if (diffS <= 0) return '已过期';
    const d = Math.floor(diffS / 86400);
    const h = Math.floor((diffS % 86400) / 3600);
    if (d > 0) return `${d} 天后过期`;
    if (h > 0) return `${h} 小时后过期`;
    return `${Math.max(Math.floor(diffS / 60), 1)} 分钟后过期`;
  }

  /* 创建 */
  let createOpen = $state(false);
  let snapName = $state('');
  let stateful = $state(false);
  let creating = $state(false);
  /** 自动过期：'' 表示永久保留 */
  let expiresChoice = $state('');

  const EXPIRY_OPTIONS = [
    { value: '', label: '永久保留' },
    { value: '1h', label: '1 小时后自动删除' },
    { value: '1d', label: '1 天后自动删除' },
    { value: '7d', label: '7 天后自动删除' },
    { value: '30d', label: '30 天后自动删除' },
  ];

  /** 把「1h / 7d」这类相对时长换算成 Incus 需要的 RFC3339 绝对时间 */
  function expiryToRFC3339(choice: string): string | undefined {
    const table: Record<string, number> = {
      '1h': 3600 * 1000,
      '1d': 86400 * 1000,
      '7d': 7 * 86400 * 1000,
      '30d': 30 * 86400 * 1000,
    };
    const ms = table[choice];
    if (!ms) return undefined;
    return new Date(Date.now() + ms).toISOString();
  }

  function suggestSnapName() {
    const d = new Date();
    const p = (n: number) => String(n).padStart(2, '0');
    return `snap-${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}`;
  }

  async function submitCreate(e: SubmitEvent) {
    e.preventDefault();
    if (creating) return;
    if (stateful && !statefulSupported) {
      toast.error('当前宿主机不支持运行时快照', { description: statefulHint });
      return;
    }
    creating = true;
    try {
      const op = await api.createSnapshot(name, {
        name: snapName,
        stateful,
        expires_at: expiryToRFC3339(expiresChoice),
      });
      toast.info(`正在创建快照 ${snapName}…`);
      await pollOperation(op.operation_id);
      toast.success(`快照「${snapName}」已创建`);
      createOpen = false;
      snapName = '';
      expiresChoice = '';
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      creating = false;
    }
  }

  /* 重命名 */
  let renameTarget = $state<SnapshotInfo | null>(null);
  let renameDraft = $state('');
  let renameOpen = $state(false);
  let renaming = $state(false);
  function openRename(s: SnapshotInfo) {
    renameTarget = s;
    renameDraft = s.name;
    renameOpen = true;
  }
  async function doRename() {
    if (!renameTarget || !renameDraft.trim() || renameDraft === renameTarget.name) {
      renameOpen = false;
      return;
    }
    renaming = true;
    try {
      const op = await api.renameSnapshot(name, renameTarget.name, renameDraft.trim());
      await pollOperation(op.operation_id);
      toast.success(`快照已重命名为「${renameDraft.trim()}」`);
      renameOpen = false;
      renameTarget = null;
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      renaming = false;
    }
  }

  /* 自动快照计划（实例 config 的 snapshots.* 键） */
  let sched = $state({ schedule: '', scheduleStopped: '', expiry: '', pattern: '' });
  let schedLoaded = $state(false);
  let schedSaving = $state(false);

  /* 宿主机能力：容器 stateful 快照依赖 CRIU；缺失时 Incus 会长时间挂起而非报错 */
  let instType = $state<'container' | 'virtual-machine'>('container');
  let features = $state<ServerFeatures | null>(null);

  async function loadFeatures() {
    try {
      features = await api.serverFeatures();
    } catch {
      /* 探测失败时按「支持」处理，保持与旧行为一致 */
    }
  }

  const statefulSupported = $derived(
    instType === 'virtual-machine'
      ? (features?.vm_stateful ?? true)
      : (features?.container_stateful ?? true),
  );

  const statefulHint = $derived.by(() => {
    if (instType === 'virtual-machine') return '虚拟机由 QEMU 原生支持，无需 CRIU。';
    if (features === null) return '正在检测宿主机是否安装 CRIU…';
    if (features.container_stateful) return `容器依赖宿主机 CRIU（${features.criu_version ?? '已安装'}）。`;
    return '宿主机未安装 CRIU，容器无法保存运行时状态（Incus 会长时间挂起而非报错）。如需该能力，请在宿主机执行 apt install criu 后重试。';
  });

  async function loadSchedule() {
    try {
      const inst = await api.instance(name);
      const c = inst?.config ?? {};
      if (inst?.type === 'virtual-machine' || inst?.type === 'container') instType = inst.type;
      sched = {
        schedule: c['snapshots.schedule'] ?? '',
        scheduleStopped: c['snapshots.schedule.stopped'] ?? '',
        expiry: c['snapshots.expiry'] ?? '',
        pattern: c['snapshots.pattern'] ?? '',
      };
    } catch {
      /* 读取失败时保持空表单，不影响快照列表 */
    } finally {
      schedLoaded = true;
    }
  }

  const scheduleActive = $derived(!!sched.schedule.trim());

  async function saveSchedule() {
    schedSaving = true;
    try {
      // 空串在 Incus PATCH 语义下等价于「删除该键」
      const config: Record<string, string> = {
        'snapshots.schedule': sched.schedule.trim(),
        'snapshots.schedule.stopped': sched.scheduleStopped,
        'snapshots.expiry': sched.expiry.trim(),
        'snapshots.pattern': sched.pattern.trim(),
      };
      const op = await api.updateInstanceConfig(name, { config });
      await pollOperation(op?.operation_id);
      toast.success('自动快照计划已保存');
      await loadSchedule();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      schedSaving = false;
    }
  }

  /* 恢复 */
  let restoreTarget = $state<SnapshotInfo | null>(null);
  let restoreOpen = $state(false);
  let restoring = $state(false);
  async function doRestore() {
    if (!restoreTarget) return;
    restoring = true;
    try {
      // 若目标快照是 stateful 的，则连同运行时内存状态一起恢复
      const op = await api.restoreSnapshot(name, restoreTarget.name, restoreTarget.stateful);
      toast.info(`正在恢复到 ${restoreTarget.name}…`, {
        description: restoreTarget.stateful ? '将连同运行时内存状态一起恢复' : '实例将短暂不可用'
      });
      await pollOperation(op.operation_id);
      toast.success(`已恢复到快照「${restoreTarget.name}」`);
      restoreOpen = false;
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      restoring = false;
    }
  }

  /* 删除 */
  let delTarget = $state<SnapshotInfo | null>(null);
  let delOpen = $state(false);
  let delBusy = $state(false);
  async function doDelete() {
    if (!delTarget) return;
    delBusy = true;
    try {
      const op = await api.deleteSnapshot(name, delTarget.name);
      await pollOperation(op.operation_id);
      toast.success(`快照「${delTarget.name}」已删除`);
      delOpen = false;
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      delBusy = false;
    }
  }
</script>

<Card class="shadow-soft">
  <div class="flex items-center justify-between gap-3 border-b border-border/60 px-5 py-4">
    <div>
      <h2 class="text-sm font-semibold tracking-tight">快照管理</h2>
      <p class="mt-0.5 text-xs text-muted-foreground">为「{name}」创建时间点副本，可随时恢复</p>
    </div>
    <Button size="sm" class="h-8.5 gap-1.5" onclick={() => (createOpen = true)}>
      <Plus size={15} />
      创建快照
    </Button>
  </div>

  <div class="p-5">
    {#if loading}
      <div class="space-y-3">
        {#each Array(3) as _, i (i)}
          <div class="h-11 animate-pulse rounded-lg bg-foreground/[0.04]"></div>
        {/each}
      </div>
    {:else if snaps.length === 0}
      <EmptyState title="还没有快照" desc="在重大变更（升级、改配置）前创建快照，出问题可一键回滚。">
        {#snippet icon()}<Camera size={20} />{/snippet}
        {#snippet action()}
          <Button size="sm" variant="outline" class="gap-1.5" onclick={() => (createOpen = true)}>
            <Plus size={15} />创建第一个快照
          </Button>
        {/snippet}
      </EmptyState>
    {:else}
      <ul class="space-y-2">
        {#each snaps as s (s.name)}
          <li class="group flex items-center gap-3 rounded-xl border border-border/60 px-4 py-3 transition-colors hover:bg-foreground/[0.02]">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Camera size={16} />
            </span>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="truncate font-mono text-[13px] font-medium">{s.name}</span>
                {#if s.stateful}
                  <Badge variant="outline" class="px-1.5 text-[10px] text-info ring-info/25">stateful</Badge>
                {/if}
                {#if s.expires_at}
                  <Badge variant="outline" class="gap-1 px-1.5 text-[10px] text-warning ring-warning/25">
                    <Clock size={10} />
                    {expiryLabel(s.expires_at)}
                  </Badge>
                {/if}
              </div>
              <div class="font-num mt-0.5 text-[11px] text-muted-foreground">
                {fmtDateTime(s.created_at)} · {relativeTime(s.created_at)}
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-0.5 opacity-70 transition-opacity group-hover:opacity-100">
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" title="恢复到此快照" onclick={() => { restoreTarget = s; restoreOpen = true; }}>
                <History size={14} />
              </Button>
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" title="重命名" onclick={() => openRename(s)}>
                <Pencil size={14} />
              </Button>
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground hover:text-destructive" title="删除快照" onclick={() => { delTarget = s; delOpen = true; }}>
                <Trash2 size={14} />
              </Button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</Card>

<!-- 自动快照计划 -->
<Card class="shadow-soft">
  <div class="flex items-center justify-between gap-3 border-b border-border/60 px-5 py-4">
    <div>
      <h2 class="flex items-center gap-2 text-sm font-semibold tracking-tight">
        <CalendarClock size={15} class="text-primary" />
        自动快照计划
      </h2>
      <p class="mt-0.5 text-xs text-muted-foreground">
        通过实例配置项 snapshots.* 让 Incus 按计划自动打快照（与上面的手动快照相互独立）
      </p>
    </div>
    {#if scheduleActive}
      <Badge variant="outline" class="shrink-0 gap-1 px-2 py-0.5 text-[10.5px] text-success ring-success/25">
        <Clock size={10} /> 已启用
      </Badge>
    {:else}
      <Badge variant="outline" class="shrink-0 px-2 py-0.5 text-[10.5px] text-muted-foreground ring-border">未启用</Badge>
    {/if}
  </div>

  <div class="space-y-4 p-5">
    {#if !schedLoaded}
      <div class="space-y-3">
        {#each Array(3) as _, i (i)}
          <div class="h-9 animate-pulse rounded-lg bg-foreground/[0.04]"></div>
        {/each}
      </div>
    {:else}
      <div class="grid gap-3 sm:grid-cols-2">
        <div class="grid gap-1.5">
          <Label for="scSchedule">计划（cron / 快捷值）</Label>
          <Input
            id="scSchedule"
            bind:value={sched.schedule}
            placeholder="@daily 或 0 3 * * *"
            class="h-9 font-mono text-[12.5px]"
          />
          <p class="text-[11px] text-muted-foreground">留空即关闭自动快照。示例：@hourly、@daily、0 */6 * * *</p>
        </div>
        <div class="grid gap-1.5">
          <Label for="scPattern">命名模式</Label>
          <Input
            id="scPattern"
            bind:value={sched.pattern}
            placeholder="auto-%Y%m%d-%H%M"
            class="h-9 font-mono text-[12.5px]"
          />
          <p class="text-[11px] text-muted-foreground">支持 strftime 占位符；留空则使用 Incus 默认 snap%d</p>
        </div>
        <div class="grid gap-1.5">
          <Label for="scExpiry">快照保留时长</Label>
          <Input
            id="scExpiry"
            bind:value={sched.expiry}
            placeholder="1w 或 7d"
            class="h-9 font-mono text-[12.5px]"
          />
          <p class="text-[11px] text-muted-foreground">如 1w / 3d；留空表示永久保留</p>
        </div>
        <div class="grid gap-1.5">
          <Label for="scStopped">实例停止时是否也执行</Label>
          <select
            id="scStopped"
            bind:value={sched.scheduleStopped}
            class="h-9 w-full rounded-lg border border-input bg-transparent px-2 text-[13px] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            <option value="">默认（跟随 Incus 行为）</option>
            <option value="false">否，仅在运行时快照</option>
            <option value="true">是，停止状态也快照</option>
          </select>
          <p class="text-[11px] text-muted-foreground">对应 snapshots.schedule.stopped</p>
        </div>
      </div>

      <div class="flex items-start gap-2 rounded-lg bg-foreground/[0.02] px-3 py-2.5 text-[11.5px] text-muted-foreground">
        <Info size={13} class="mt-0.5 shrink-0" />
        <span>计划生效需要 incusd 常驻运行；若实例处于停止状态且未开启上方开关，Incus 会跳过该次快照。</span>
      </div>

      <div class="flex justify-end">
        <Button size="sm" class="h-9 gap-1.5" disabled={schedSaving} onclick={saveSchedule}>
          {#if schedSaving}
            <span class="inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
            保存中…
          {:else}
            <Save size={14} /> 保存计划
          {/if}
        </Button>
      </div>
    {/if}
  </div>
</Card>

<!-- 创建快照 -->
<Dialog.Root bind:open={createOpen}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title>创建快照</Dialog.Title>
      <Dialog.Description>为实例当前状态创建一个可恢复的时间点。</Dialog.Description>
    </Dialog.Header>
    <form id="snapshot-form" class="grid gap-4" onsubmit={submitCreate}>
      <div class="grid gap-1.5">
        <Label for="snapName">快照名称</Label>
        <div class="flex gap-2">
          <Input id="snapName" required bind:value={snapName} placeholder="pre-upgrade" class="h-9 flex-1 font-mono text-[13px]" />
          <Button type="button" variant="outline" size="sm" class="h-9 shrink-0 text-[12px]" onclick={() => (snapName = suggestSnapName())}>
            按时间生成
          </Button>
        </div>
      </div>
      <div class="grid gap-1.5">
        <Label for="snapExpiry">自动过期</Label>
        <select
          id="snapExpiry"
          bind:value={expiresChoice}
          class="h-9 w-full rounded-lg border border-input bg-transparent px-2 text-[13px] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          {#each EXPIRY_OPTIONS as o (o.value)}
            <option value={o.value}>{o.label}</option>
          {/each}
        </select>
        <p class="text-[11px] text-muted-foreground">到期后由 Incus 自动清理，适合定期备份类快照。</p>
      </div>
      <label
        class="flex cursor-pointer items-start gap-2.5 rounded-lg border px-3 py-2.5 text-[13px] {statefulSupported
          ? 'border-border/60'
          : 'cursor-not-allowed border-warning/30 bg-warning/[0.04] opacity-80'}"
      >
        <input
          type="checkbox"
          bind:checked={stateful}
          disabled={!statefulSupported}
          class="mt-0.5 size-4 accent-[var(--primary)] disabled:cursor-not-allowed"
        />
        <span>
          <span class="flex items-center gap-1.5">
            包含运行时状态（stateful）
            {#if !statefulSupported}
              <Badge variant="outline" class="px-1.5 text-[10px] text-warning ring-warning/25">不可用</Badge>
            {/if}
          </span>
          <span class="mt-0.5 block text-[11px] leading-relaxed {statefulSupported ? 'text-muted-foreground' : 'text-warning'}">
            {statefulHint}
          </span>
        </span>
      </label>
    </form>
    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" disabled={creating} onclick={() => (createOpen = false)}>取消</Button>
      <Button type="submit" form="snapshot-form" disabled={creating}>
        {#if creating}<span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>{/if}
        创建
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

{#if restoreTarget}
  <ConfirmDialog
    bind:open={restoreOpen}
    title="恢复到快照「{restoreTarget.name}」"
    description="实例当前磁盘状态将被快照内容覆盖，恢复期间实例会短暂中断。"
    confirmLabel="确认恢复"
    busy={restoring}
    onconfirm={doRestore}
  />
{/if}

{#if delTarget}
  <ConfirmDialog
    bind:open={delOpen}
    title="删除快照「{delTarget.name}」"
    description="删除后将无法再恢复到此时间点。"
    busy={delBusy}
    onconfirm={doDelete}
  />
{/if}

<!-- 重命名快照 -->
<Dialog.Root bind:open={renameOpen}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title class="flex items-center gap-2">
        <Pencil size={15} class="text-primary" />
        重命名快照
      </Dialog.Title>
      <Dialog.Description class="font-mono">{renameTarget?.name}</Dialog.Description>
    </Dialog.Header>
    <form id="rename-snapshot-form" class="grid gap-1.5" onsubmit={(e) => { e.preventDefault(); doRename(); }}>
      <Label for="snapNewName">新名称</Label>
      <Input id="snapNewName" required bind:value={renameDraft} class="h-9 font-mono text-[13px]" />
      <p class="text-[11px] text-muted-foreground">重命名不影响快照内容与创建时间。</p>
    </form>
    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" disabled={renaming} onclick={() => (renameOpen = false)}>取消</Button>
      <Button type="submit" form="rename-snapshot-form" disabled={renaming || !renameDraft.trim()}>
        {#if renaming}<span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>{/if}
        保存
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
