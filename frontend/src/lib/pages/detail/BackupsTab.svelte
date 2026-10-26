<script lang="ts">
  import { onMount } from 'svelte';
  import { PackageOpen, Download, Trash2, Plus, FileArchive } from '@lucide/svelte';
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
  import type { BackupInfo } from '$lib/types';
  import { fmtDateTime, relativeTime } from '$lib/utils';
  import { bus } from '$lib/components/layout/state.svelte';

  let { name }: { name: string } = $props();

  let backups = $state<BackupInfo[]>([]);
  let loading = $state(true);

  async function load() {
    loading = true;
    try {
      backups = await api.backups(name);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      loading = false;
    }
  }

  onMount(load);
  $effect(() => {
    void bus.refreshTick;
    if (bus.refreshTick > 0) load();
  });

  /* 创建 */
  let createOpen = $state(false);
  let bname = $state('');
  let instanceOnly = $state(true);
  let optimized = $state(false);
  let creating = $state(false);

  async function submitCreate(e: SubmitEvent) {
    e.preventDefault();
    creating = true;
    try {
      const finalName = bname || `${name}-${new Date().toISOString().slice(0, 10)}`;
      const op = await api.createBackup(name, { name: finalName, instance_only: instanceOnly, optimized_storage: optimized });
      toast.info(`正在创建备份 ${finalName}…`, { description: '大实例可能需要较长时间' });
      await pollOperation(op.operation_id);
      toast.success(`备份「${finalName}」已完成`);
      createOpen = false;
      bname = '';
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      creating = false;
    }
  }

  /* 删除 */
  let delTarget = $state<BackupInfo | null>(null);
  let delOpen = $state(false);
  let delBusy = $state(false);
  async function doDelete() {
    if (!delTarget) return;
    delBusy = true;
    try {
      await api.deleteBackup(name, delTarget.name);
      toast.success(`备份「${delTarget.name}」已删除`);
      delOpen = false;
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      delBusy = false;
    }
  }

  /* 导出为镜像 */
  let exporting = $state(false);
  async function exportAsImage() {
    exporting = true;
    try {
      const op = await api.exportInstance(name, `${name}-export-${Date.now().toString(36)}`);
      await pollOperation(op.operation_id);
      toast.success('已发布为镜像', { description: '可在镜像页查看并导出' });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      exporting = false;
    }
  }
</script>

<Card class="shadow-soft">
  <div class="flex items-center justify-between gap-3 border-b border-border/60 px-5 py-4">
    <div>
      <h2 class="text-sm font-semibold tracking-tight">备份管理</h2>
      <p class="mt-0.5 text-xs text-muted-foreground">打包导出实例（含根磁盘），可下载到本地保存</p>
    </div>
    <div class="flex items-center gap-2">
      <Button size="sm" variant="outline" class="h-8.5 gap-1.5" disabled={exporting} onclick={exportAsImage}>
        <FileArchive size={14} />
        发布为镜像
      </Button>
      <Button size="sm" class="h-8.5 gap-1.5" onclick={() => (createOpen = true)}>
        <Plus size={15} />
        创建备份
      </Button>
    </div>
  </div>

  <div class="p-5">
    {#if loading}
      <div class="space-y-3">
        {#each Array(2) as _, i (i)}
          <div class="h-11 animate-pulse rounded-lg bg-foreground/[0.04]"></div>
        {/each}
      </div>
    {:else if backups.length === 0}
      <EmptyState title="还没有备份" desc="定期备份是最后的防线；备份文件可下载到面板所在机器之外保存。">
        {#snippet icon()}<PackageOpen size={20} />{/snippet}
        {#snippet action()}
          <Button size="sm" variant="outline" class="gap-1.5" onclick={() => (createOpen = true)}>
            <Plus size={15} />创建第一个备份
          </Button>
        {/snippet}
      </EmptyState>
    {:else}
      <ul class="space-y-2">
        {#each backups as b (b.name)}
          <li class="group flex items-center gap-3 rounded-xl border border-border/60 px-4 py-3 transition-colors hover:bg-foreground/[0.02]">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-warning/15 text-warning">
              <FileArchive size={16} />
            </span>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="truncate font-mono text-[13px] font-medium">{b.name}</span>
                {#if b.optimized_storage}
                  <Badge variant="outline" class="px-1.5 text-[10px] text-info ring-info/25">优化存储</Badge>
                {/if}
                <Badge variant="outline" class="px-1.5 text-[10px] text-muted-foreground ring-border">
                  {b.instance_only ? '仅实例' : '含快照'}
                </Badge>
              </div>
              <div class="font-num mt-0.5 text-[11px] text-muted-foreground">
                {fmtDateTime(b.created_at)} · {relativeTime(b.created_at)}
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-0.5 opacity-70 transition-opacity group-hover:opacity-100">
              <a
                class="inline-flex size-7.5 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                href={api.backupDownloadURL(name, b.name)}
                title="下载备份"
                download
              >
                <Download size={14} />
              </a>
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground hover:text-destructive" title="删除备份" onclick={() => { delTarget = b; delOpen = true; }}>
                <Trash2 size={14} />
              </Button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</Card>

<!-- 创建备份 -->
<Dialog.Root bind:open={createOpen}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title>创建备份</Dialog.Title>
      <Dialog.Description>备份将保存到存储池的 backups 目录，可随时下载。</Dialog.Description>
    </Dialog.Header>
    <form id="backup-form" class="grid gap-4" onsubmit={submitCreate}>
      <div class="grid gap-1.5">
        <Label for="bname">备份名称</Label>
        <Input id="bname" bind:value={bname} placeholder="留空自动按日期命名" class="h-9 font-mono text-[13px]" />
      </div>
      <label class="flex cursor-pointer items-center gap-2.5 rounded-lg border border-border/60 px-3 py-2.5 text-[13px]">
        <input type="checkbox" bind:checked={instanceOnly} class="size-4 accent-[var(--primary)]" />
        <span>仅备份实例本体（不含快照）</span>
      </label>
      <label class="flex cursor-pointer items-center gap-2.5 rounded-lg border border-border/60 px-3 py-2.5 text-[13px]">
        <input type="checkbox" bind:checked={optimized} class="size-4 accent-[var(--primary)]" />
        <span>
          使用存储后端优化格式
          <span class="block text-[11px] text-muted-foreground">仅 zfs/btrfs 池有效，速度更快但跨后端不可恢复</span>
        </span>
      </label>
    </form>
    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" disabled={creating} onclick={() => (createOpen = false)}>取消</Button>
      <Button type="submit" form="backup-form" disabled={creating}>
        {#if creating}<span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>{/if}
        创建备份
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

{#if delTarget}
  <ConfirmDialog
    bind:open={delOpen}
    title="删除备份「{delTarget.name}」"
    description="备份文件将从存储池中永久删除。"
    busy={delBusy}
    onconfirm={doDelete}
  />
{/if}
