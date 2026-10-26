<script lang="ts">
  import { fade } from 'svelte/transition';
  import { HardDrive, Trash2, Plus, Database, Layers3 } from '@lucide/svelte';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Badge } from '$lib/components/ui/badge';
  import * as Select from '$lib/components/ui/select';
  import * as Dialog from '$lib/components/ui/dialog';
  import { Label } from '$lib/components/ui/label';
  import { toast } from 'svelte-sonner';
  import EmptyState from '$lib/components/common/EmptyState.svelte';
  import TypeBadge from '$lib/components/common/TypeBadge.svelte';
  import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
  import KeyValues from '$lib/components/common/KeyValues.svelte';
  import { api, pollOperation } from '$lib/api';
  import type { StoragePoolInfo, StorageVolumeInfo } from '$lib/types';
  import { bus } from '$lib/components/layout/state.svelte';

  let pools = $state<StoragePoolInfo[]>([]);
  let volumes = $state<Record<string, StorageVolumeInfo[]>>({});
  let loading = $state(true);

  async function load() {
    loading = true;
    try {
      pools = (await api.storagePools()) ?? [];
      const results = await Promise.allSettled(pools.map((p) => api.storageVolumes(p.name)));
      const map: Record<string, StorageVolumeInfo[]> = {};
      pools.forEach((p, i) => {
        map[p.name] = results[i].status === 'fulfilled' ? results[i].value : [];
      });
      volumes = map;
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    void bus.refreshTick;
    if (bus.refreshTick > 0) load();
  });
  $effect(() => {
    load();
  });

  /* 创建池 */
  let createOpen = $state(false);
  let form = $state({ name: '', driver: 'dir', source: '' });
  let creating = $state(false);
  async function submitCreate(e: SubmitEvent) {
    e.preventDefault();
    creating = true;
    try {
      const op = await api.createStoragePool({
        name: form.name,
        driver: form.driver,
        config: form.source ? { source: form.source } : {},
      });
      toast.info(`正在创建存储池 ${form.name}…`);
      await pollOperation(op.operation_id);
      toast.success(`存储池「${form.name}」已创建`);
      createOpen = false;
      form = { name: '', driver: 'dir', source: '' };
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      creating = false;
    }
  }

  /* 创建卷 */
  let volOpen = $state(false);
  let volPool = $state('');
  let volForm = $state({ name: '', type: 'custom', size: '10GiB' });
  let volCreating = $state(false);
  function openCreateVolume(pool: string) {
    volPool = pool;
    volForm = { name: '', type: 'custom', size: '10GiB' };
    volOpen = true;
  }
  async function submitVolume(e: SubmitEvent) {
    e.preventDefault();
    volCreating = true;
    try {
      await api.createStorageVolume(volPool, {
        name: volForm.name,
        type: volForm.type,
        config: volForm.size ? { size: volForm.size } : {},
      });
      toast.success(`卷「${volForm.name}」已创建于 ${volPool}`);
      volOpen = false;
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      volCreating = false;
    }
  }

  /* 删除池 */
  let delTarget = $state<StoragePoolInfo | null>(null);
  let delOpen = $state(false);
  let delBusy = $state(false);
  async function doDelete() {
    if (!delTarget) return;
    delBusy = true;
    try {
      const op = await api.deleteStoragePool(delTarget.name);
      await pollOperation(op.operation_id);
      toast.success(`存储池「${delTarget.name}」已删除`);
      delOpen = false;
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      delBusy = false;
    }
  }

  function driverBadgeTone(driver: string): 'default' | 'info' | 'success' {
    if (driver === 'zfs' || driver === 'btrfs') return 'success';
    if (driver === 'ceph') return 'info';
    return 'default';
  }
</script>

<div class="animate-fade-up space-y-4">
  <div class="flex items-center gap-2.5">
    <span class="font-num ml-auto text-xs text-muted-foreground">{pools.length} 个存储池</span>
    <Button size="sm" class="h-9 gap-1.5" onclick={() => (createOpen = true)}>
      <Plus size={15} />
      创建存储池
    </Button>
  </div>

  {#if loading}
    <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
      {#each Array(2) as _, i (i)}
        <div class="h-52 animate-pulse rounded-xl bg-foreground/[0.04]"></div>
      {/each}
    </div>
  {:else if pools.length === 0}
    <Card class="p-5 shadow-soft">
      <EmptyState title="没有存储池" desc="存储池为实例的根磁盘提供后端，通常安装 Incus 时自动创建。">
        {#snippet icon()}<HardDrive size={20} />{/snippet}
      </EmptyState>
    </Card>
  {:else}
    <div class="grid grid-cols-1 gap-4 xl:grid-cols-2" in:fade={{ duration: 150 }}>
      {#each pools as p (p.name)}
        <div class="lift-card rounded-xl border border-border/70 bg-card p-5 transition-transform duration-200 hover:-translate-y-0.5">
          <div class="flex items-start justify-between gap-3">
            <div class="flex min-w-0 items-center gap-3">
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-success/15 to-primary/10 text-success">
                <HardDrive size={19} />
              </span>
              <div class="min-w-0">
                <div class="truncate font-mono text-[14px] font-semibold tracking-tight">{p.name}</div>
                <div class="mt-0.5 flex items-center gap-1.5">
                  <TypeBadge label={p.driver} tone={driverBadgeTone(p.driver)} />
                  <span class="text-[10.5px] text-muted-foreground">{p.status === 'Created' ? '已就绪' : p.status}</span>
                </div>
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-0.5">
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" title="创建卷" onclick={() => openCreateVolume(p.name)}>
                <Plus size={14} />
              </Button>
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground hover:text-destructive" title="删除存储池" onclick={() => { delTarget = p; delOpen = true; }}>
                <Trash2 size={14} />
              </Button>
            </div>
          </div>

          <div class="mt-4">
            <div class="mb-1.5 flex items-center justify-between text-[11.5px] text-muted-foreground">
              <span class="flex items-center gap-1"><Layers3 size={12} /> 卷（{(volumes[p.name] ?? []).length}）</span>
              <span>{(p.used_by ?? []).length} 项引用</span>
            </div>
            {#if (volumes[p.name] ?? []).length > 0}
              <div class="flex flex-wrap gap-1.5">
                {#each volumes[p.name].slice(0, 6) as v (v.name)}
                  <Badge variant="secondary" class="gap-1 px-2 py-1 font-mono text-[10.5px] font-normal">
                    <Database size={10} class={v.type === 'custom' ? 'text-info' : v.type === 'image' ? 'text-muted-foreground' : 'text-success'} />
                    {v.name.length > 26 ? v.name.slice(0, 24) + '…' : v.name}
                  </Badge>
                {/each}
                {#if (volumes[p.name] ?? []).length > 6}
                  <Badge variant="outline" class="px-2 py-1 text-[10.5px] text-muted-foreground ring-border">+{(volumes[p.name] ?? []).length - 6}</Badge>
                {/if}
              </div>
            {:else}
              <p class="rounded-lg border border-dashed border-border/70 px-3 py-3 text-center text-[11.5px] text-muted-foreground">暂无卷</p>
            {/if}
          </div>

          <div class="mt-4">
            <KeyValues entries={Object.entries(p.config ?? {}).slice(0, 4)} emptyText="默认配置" />
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- 创建存储池 -->
<Dialog.Root bind:open={createOpen}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title>创建存储池</Dialog.Title>
      <Dialog.Description>为实例根磁盘与自定义卷提供存储后端。</Dialog.Description>
    </Dialog.Header>
    <form id="pool-form" class="grid gap-4" onsubmit={submitCreate}>
      <div class="grid grid-cols-2 gap-3">
        <div class="grid gap-1.5">
          <Label for="poolName">名称</Label>
          <Input id="poolName" required bind:value={form.name} placeholder="ssd-pool" class="h-9 font-mono text-[13px]" />
        </div>
        <div class="grid gap-1.5">
          <Label>驱动</Label>
          <Select.Root type="single" bind:value={form.driver}>
            <Select.Trigger class="h-9 w-full text-[13px]">{form.driver}</Select.Trigger>
            <Select.Content>
              <Select.Item value="dir">dir（目录）</Select.Item>
              <Select.Item value="zfs">zfs</Select.Item>
              <Select.Item value="btrfs">btrfs</Select.Item>
              <Select.Item value="lvm">lvm</Select.Item>
            </Select.Content>
          </Select.Root>
        </div>
      </div>
      <div class="grid gap-1.5">
        <Label for="poolSource">source 路径 / 设备</Label>
        <Input id="poolSource" bind:value={form.source} class="h-9 font-mono text-[12.5px]" placeholder="/srv/incus/ssd 或 /dev/nvme0n1" />
      </div>
    </form>
    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" disabled={creating} onclick={() => (createOpen = false)}>取消</Button>
      <Button type="submit" form="pool-form" disabled={creating}>
        {#if creating}<span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>{/if}
        创建
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<!-- 创建卷 -->
<Dialog.Root bind:open={volOpen}>
  <Dialog.Content class="sm:max-w-sm">
    <Dialog.Header>
      <Dialog.Title>创建存储卷</Dialog.Title>
      <Dialog.Description class="font-mono">{volPool}</Dialog.Description>
    </Dialog.Header>
    <form id="vol-form" class="grid gap-4" onsubmit={submitVolume}>
      <div class="grid gap-1.5">
        <Label for="volName">卷名</Label>
        <Input id="volName" required bind:value={volForm.name} placeholder="pgdata" class="h-9 font-mono text-[13px]" />
      </div>
      <div class="grid grid-cols-2 gap-3">
        <div class="grid gap-1.5">
          <Label>类型</Label>
          <Select.Root type="single" bind:value={volForm.type}>
            <Select.Trigger class="h-9 w-full text-[13px]">{volForm.type}</Select.Trigger>
            <Select.Content>
              <Select.Item value="custom">custom</Select.Item>
              <Select.Item value="container">container</Select.Item>
              <Select.Item value="virtual-machine">virtual-machine</Select.Item>
              <Select.Item value="image">image</Select.Item>
            </Select.Content>
          </Select.Root>
        </div>
        <div class="grid gap-1.5">
          <Label for="volSize">容量</Label>
          <Input id="volSize" bind:value={volForm.size} class="h-9 font-mono text-[13px]" placeholder="10GiB" />
        </div>
      </div>
    </form>
    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" disabled={volCreating} onclick={() => (volOpen = false)}>取消</Button>
      <Button type="submit" form="vol-form" disabled={volCreating}>
        {#if volCreating}<span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>{/if}
        创建
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

{#if delTarget}
  <ConfirmDialog
    bind:open={delOpen}
    title="删除存储池「{delTarget.name}」"
    description="池中所有卷（含实例根磁盘）都将被销毁，此操作不可恢复。"
    busy={delBusy}
    onconfirm={doDelete}
  />
{/if}
