<script lang="ts">
  import { fade } from 'svelte/transition';
  import { Search, Network, Trash2, Plus, Pencil, Waypoints } from '@lucide/svelte';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Badge } from '$lib/components/ui/badge';
  import * as Select from '$lib/components/ui/select';
  import * as Dialog from '$lib/components/ui/dialog';
  import { Label } from '$lib/components/ui/label';
  import { toast } from 'svelte-sonner';
  import EmptyState from '$lib/components/common/EmptyState.svelte';
  import StatusDot from '$lib/components/common/StatusDot.svelte';
  import TypeBadge from '$lib/components/common/TypeBadge.svelte';
  import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
  import KeyValues from '$lib/components/common/KeyValues.svelte';
  import { api, pollOperation } from '$lib/api';
  import type { NetworkInfo } from '$lib/types';
  import { bus } from '$lib/components/layout/state.svelte';

  let networks = $state<NetworkInfo[]>([]);
  let loading = $state(true);
  let keyword = $state('');

  async function load() {
    loading = true;
    try {
      networks = (await api.networks()) ?? [];
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

  const filtered = $derived.by(() => {
    const kw = keyword.trim().toLowerCase();
    if (!kw) return networks;
    return networks.filter((n) => n.name.toLowerCase().includes(kw) || n.type.toLowerCase().includes(kw));
  });

  /* 详情查看：列表数据不含 DHCP 租约，打开时再取一次详情 */
  let detail = $state<NetworkInfo | null>(null);
  let detailLoading = $state(false);
  let detailReq = 0;
  async function openDetail(n: NetworkInfo) {
    const req = ++detailReq;
    detail = n;
    detailLoading = true;
    try {
      const full = await api.network(n.name);
      if (req === detailReq) detail = full;
    } catch {
      // 详情接口失败时保留列表行数据，不阻断查看
    } finally {
      if (req === detailReq) detailLoading = false;
    }
  }
  function closeDetail() {
    detailReq++;
    detail = null;
    detailLoading = false;
  }

  /* 创建 */
  let createOpen = $state(false);
  let form = $state({ name: '', type: 'bridge', ipv4: '10.100.0.1/24', ipv4Nat: true });
  let creating = $state(false);
  async function submitCreate(e: SubmitEvent) {
    e.preventDefault();
    creating = true;
    try {
      const op = await api.createNetwork({
        name: form.name,
        type: form.type,
        config: {
          'ipv4.address': form.ipv4,
          ...(form.ipv4Nat ? { 'ipv4.nat': 'true' } : {}),
        },
      });
      toast.info(`正在创建网络 ${form.name}…`);
      await pollOperation(op.operation_id);
      toast.success(`网络「${form.name}」已创建`);
      createOpen = false;
      form = { name: '', type: 'bridge', ipv4: '10.100.0.1/24', ipv4Nat: true };
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      creating = false;
    }
  }

  /* 编辑描述 */
  let editTarget = $state<NetworkInfo | null>(null);
  let editOpen = $state(false);
  let editDesc = $state('');
  let editing = $state(false);
  function openEdit(n: NetworkInfo) {
    editTarget = n;
    editDesc = n.description;
    editOpen = true;
  }
  async function submitEdit(e: SubmitEvent) {
    e.preventDefault();
    if (!editTarget) return;
    editing = true;
    try {
      await api.updateNetwork(editTarget.name, { description: editDesc, config: editTarget.config });
      toast.success(`网络「${editTarget.name}」已更新`);
      editOpen = false;
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      editing = false;
    }
  }

  /* 删除 */
  let delTarget = $state<NetworkInfo | null>(null);
  let delOpen = $state(false);
  let delBusy = $state(false);
  async function doDelete() {
    if (!delTarget) return;
    delBusy = true;
    try {
      const op = await api.deleteNetwork(delTarget.name);
      await pollOperation(op.operation_id);
      toast.success(`网络「${delTarget.name}」已删除`);
      delOpen = false;
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      delBusy = false;
    }
  }
</script>

<div class="animate-fade-up space-y-4">
  <div class="flex flex-wrap items-center gap-2.5">
    <div class="relative min-w-[220px] flex-1 sm:max-w-xs">
      <Search size={14} class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground" />
      <Input bind:value={keyword} placeholder="搜索网络…" class="h-9 pl-8.5 text-[13px]" />
    </div>
    <div class="ml-auto flex items-center gap-2.5">
      <span class="font-num text-xs text-muted-foreground">{filtered.length} 个网络</span>
      <Button size="sm" class="h-9 gap-1.5" onclick={() => (createOpen = true)}>
        <Plus size={15} />
        创建网络
      </Button>
    </div>
  </div>

  <Card class="shadow-soft">
    {#if loading}
      <div class="space-y-3 p-5">
        {#each Array(3) as _, i (i)}
          <div class="h-10 animate-pulse rounded-lg bg-foreground/[0.04]"></div>
        {/each}
      </div>
    {:else if filtered.length === 0}
      <div class="p-5">
        <EmptyState title="没有匹配的网络" desc="创建网桥以供实例使用的隔离网络。">
          {#snippet icon()}<Network size={20} />{/snippet}
        </EmptyState>
      </div>
    {:else}
      <div class="overflow-x-auto" in:fade={{ duration: 150 }}>
        <table class="w-full text-left text-[13px]">
          <thead>
            <tr class="border-b border-border/70 text-[11.5px] text-muted-foreground">
              <th class="py-3 pr-3 pl-5 font-medium">名称</th>
              <th class="py-3 pr-3 font-medium">类型</th>
              <th class="py-3 pr-3 font-medium">状态</th>
              <th class="py-3 pr-3 font-medium">托管</th>
              <th class="py-3 pr-3 font-medium">子网 (IPv4)</th>
              <th class="py-3 pr-3 font-medium">引用</th>
              <th class="py-3 pr-5 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each filtered as n (n.name)}
              {@const subnet = n.config?.['ipv4.address'] ?? '—'}
              <tr class="group cursor-pointer border-b border-border/40 transition-colors last:border-0 hover:bg-foreground/[0.02]" onclick={() => openDetail(n)}>
                <td class="py-3 pr-3 pl-5 font-medium">{n.name}</td>
                <td class="py-3 pr-3"><TypeBadge label={n.type} /></td>
                <td class="py-3 pr-3">
                  <StatusDot state="active" label={n.status === 'Created' ? '已创建' : n.status} ping={false} />
                </td>
                <td class="py-3 pr-3">
                  {#if n.managed}
                    <Badge variant="outline" class="bg-success/8 px-1.5 text-[10.5px] text-success ring-success/20">managed</Badge>
                  {:else}
                    <span class="text-[12px] text-muted-foreground">外部</span>
                  {/if}
                </td>
                <td class="py-3 pr-3 font-mono text-[12px] text-muted-foreground">{subnet}</td>
                <td class="font-num py-3 pr-3 text-muted-foreground">{n.used_by?.length ?? 0} 项</td>
                <td class="py-3 pr-5" onclick={(e) => e.stopPropagation()}>
                  <div class="flex items-center justify-end gap-0.5">
                    <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" title="编辑描述" onclick={() => openEdit(n)}>
                      <Pencil size={14} />
                    </Button>
                    <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground hover:text-destructive" title="删除" disabled={!n.managed} onclick={() => { delTarget = n; delOpen = true; }}>
                      <Trash2 size={14} />
                    </Button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </Card>
</div>

<!-- 网络详情 -->
<Dialog.Root open={!!detail} onOpenChange={(v) => !v && closeDetail()}>
  <Dialog.Content class="max-h-[85vh] sm:max-w-xl sm:overflow-y-auto">
    {#if detail}
      <Dialog.Header>
        <Dialog.Title class="flex items-center gap-2 font-mono">
          <Network size={17} class="text-primary" />
          {detail.name}
        </Dialog.Title>
        <Dialog.Description>{detail.description || '网络详情与配置'}</Dialog.Description>
      </Dialog.Header>

      <div class="space-y-5">
        {#if detailLoading}
          <div class="flex items-center gap-2 text-[12px] text-muted-foreground">
            <span class="inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
            正在读取 DHCP 租约…
          </div>
        {:else if detail.leases && detail.leases.length > 0}
          <div>
            <div class="mb-2 flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
              <Waypoints size={13} /> DHCP 租约
            </div>
            <div class="overflow-hidden rounded-xl border border-border/70">
              <table class="w-full text-left text-[12px]">
                <thead>
                  <tr class="border-b border-border/60 bg-foreground/[0.02] text-[10.5px] text-muted-foreground">
                    <th class="px-3 py-2 font-medium">主机</th>
                    <th class="px-3 py-2 font-medium">MAC</th>
                    <th class="px-3 py-2 font-medium">IP</th>
                    <th class="px-3 py-2 font-medium">类型</th>
                  </tr>
                </thead>
                <tbody>
                  {#each detail.leases as l (l.hwaddr)}
                    <tr class="border-b border-border/40 last:border-0">
                      <td class="px-3 py-2 font-mono">{l.hostname}</td>
                      <td class="px-3 py-2 font-mono text-muted-foreground">{l.hwaddr}</td>
                      <td class="px-3 py-2 font-mono">{l.address}</td>
                      <td class="px-3 py-2 text-muted-foreground">{l.type}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>
        {/if}

        <div>
          <div class="mb-2 text-[12px] font-medium text-muted-foreground">配置</div>
          <KeyValues entries={Object.entries(detail.config ?? {})} emptyText="默认配置" />
        </div>

        <div>
          <div class="mb-2 text-[12px] font-medium text-muted-foreground">被引用（{detail.used_by?.length ?? 0}）</div>
          {#if (detail.used_by?.length ?? 0) > 0}
            <ul class="space-y-1">
              {#each detail.used_by ?? [] as u (u)}
                <li class="rounded-md bg-foreground/[0.03] px-3 py-1.5 font-mono text-[11.5px] text-muted-foreground">{u}</li>
              {/each}
            </ul>
          {:else}
            <p class="text-[12px] text-muted-foreground">暂无引用</p>
          {/if}
        </div>
      </div>
    {/if}
  </Dialog.Content>
</Dialog.Root>

<!-- 创建网络 -->
<Dialog.Root bind:open={createOpen}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title>创建网桥</Dialog.Title>
      <Dialog.Description>创建托管网桥，实例网卡可接入并自动分配地址。</Dialog.Description>
    </Dialog.Header>
    <form id="net-form" class="grid gap-4" onsubmit={submitCreate}>
      <div class="grid grid-cols-2 gap-3">
        <div class="grid gap-1.5">
          <Label for="netName">名称</Label>
          <Input id="netName" required bind:value={form.name} placeholder="incusbr1" class="h-9 font-mono text-[13px]" />
        </div>
        <div class="grid gap-1.5">
          <Label>类型</Label>
          <Select.Root type="single" bind:value={form.type}>
            <Select.Trigger class="h-9 w-full text-[13px]">bridge</Select.Trigger>
            <Select.Content>
              <Select.Item value="bridge">bridge</Select.Item>
              <Select.Item value="ovn">ovn</Select.Item>
            </Select.Content>
          </Select.Root>
        </div>
      </div>
      <div class="grid gap-1.5">
        <Label for="netIPv4">IPv4 子网</Label>
        <Input id="netIPv4" bind:value={form.ipv4} class="h-9 font-mono text-[13px]" placeholder="10.100.0.1/24" />
      </div>
      <label class="flex cursor-pointer items-center gap-2.5 rounded-lg border border-border/60 px-3 py-2.5 text-[13px]">
        <input type="checkbox" bind:checked={form.ipv4Nat} class="size-4 accent-[var(--primary)]" />
        <span>启用 NAT（IPv4 出站伪装）</span>
      </label>
    </form>
    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" disabled={creating} onclick={() => (createOpen = false)}>取消</Button>
      <Button type="submit" form="net-form" disabled={creating}>
        {#if creating}<span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>{/if}
        创建
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<!-- 编辑描述 -->
<Dialog.Root bind:open={editOpen}>
  <Dialog.Content class="sm:max-w-sm">
    <Dialog.Header>
      <Dialog.Title>编辑网络描述</Dialog.Title>
      <Dialog.Description class="font-mono">{editTarget?.name}</Dialog.Description>
    </Dialog.Header>
    <form id="net-edit-form" class="grid gap-4" onsubmit={submitEdit}>
      <div class="grid gap-1.5">
        <Label for="netDesc">描述</Label>
        <Input id="netDesc" bind:value={editDesc} class="h-9 text-[13px]" placeholder="用途说明" />
      </div>
    </form>
    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" onclick={() => (editOpen = false)}>取消</Button>
      <Button type="submit" form="net-edit-form" disabled={editing}>保存</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

{#if delTarget}
  <ConfirmDialog
    bind:open={delOpen}
    title="删除网络「{delTarget.name}」"
    description="正在使用该网络的实例网卡将失去连接，请先确认引用列表。"
    busy={delBusy}
    onconfirm={doDelete}
  />
{/if}
