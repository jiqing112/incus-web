<script lang="ts">
  import { fade } from 'svelte/transition';
  import {
    Search, Plus, Power, RotateCw, SquareTerminal, Trash2, Boxes, ChevronRight, Pencil,
  } from '@lucide/svelte';
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
  import { api, pollOperation } from '$lib/api';
  import type { InstanceInfo, ImageInfo, StoragePoolInfo, NetworkInfo } from '$lib/types';
  import { fmtBytes, relativeTime } from '$lib/utils';
  import { bus } from '$lib/components/layout/state.svelte';
  import { openDetail } from '$lib/stores/detailModal.svelte';

  let instances = $state<InstanceInfo[]>([]);
  let loading = $state(true);
  let keyword = $state('');
  let noteTarget = $state<InstanceInfo | null>(null);
  let noteDraft = $state('');
  let noteSaving = $state(false);

  function openNote(i: InstanceInfo) {
    noteTarget = i;
    noteDraft = i.description ?? '';
  }

  async function saveNote() {
    if (!noteTarget) return;
    if (noteDraft === (noteTarget.description ?? '')) {
      noteTarget = null;
      return;
    }
    noteSaving = true;
    try {
      const op = await api.updateInstanceConfig(noteTarget.name, { config: {}, description: noteDraft });
      await pollOperation(op?.operation_id);
      toast.success(`「${noteTarget.name}」备注已保存`);
      noteTarget = null;
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      noteSaving = false;
    }
  }
  let statusFilter = $state<'all' | 'Running' | 'Stopped'>('all');
  let typeFilter = $state<'all' | 'container' | 'virtual-machine'>('all');

  async function load() {
    loading = true;
    try {
      instances = (await api.instances()) ?? [];
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
    return instances.filter((i) => {
      if (statusFilter !== 'all' && i.status !== statusFilter) return false;
      if (typeFilter !== 'all' && i.type !== typeFilter) return false;
      if (!kw) return true;
      return (
        i.name.toLowerCase().includes(kw) ||
        i.profiles.join(',').toLowerCase().includes(kw) ||
        ipsOf(i).toLowerCase().includes(kw) ||
        osOf(i).toLowerCase().includes(kw) ||
        (i.description ?? '').toLowerCase().includes(kw)
      );
    });
  });

  /** 实例镜像的系统名（取自实例 config 的 image.os / image.release） */
  function osOf(i: InstanceInfo): string {
    const c = i.config ?? {};
    let os = (c['image.os'] ?? '').trim();
    const rel = (c['image.release'] ?? '').trim();
    if (!os) return '';
    // 镜像元数据里的 image.os 大小写不统一（有的写 Debian，有的写 debian），统一首字母大写
    if (os === os.toLowerCase()) os = os.charAt(0).toUpperCase() + os.slice(1);
    return rel ? `${os} ${rel}` : os;
  }

  /**
   * 提取实例的全局网络地址。
   * 只取 scope=global（排除回环 local 与 IPv6 链路本地 link），IPv4 与 IPv6 分开返回。
   */
  function addrsOf(i: InstanceInfo): { v4: string[]; v6: string[] } {
    const v4: string[] = [];
    const v6: string[] = [];
    if (!i.state?.network) return { v4, v6 };
    for (const net of Object.values(i.state.network)) {
      for (const a of net.addresses ?? []) {
        if (a.scope !== 'global') continue;
        if (a.family === 'inet') v4.push(a.address);
        else if (a.family === 'inet6') v6.push(a.address);
      }
    }
    return { v4, v6 };
  }

  /** 供搜索过滤使用的扁平地址串 */
  function ipsOf(i: InstanceInfo): string {
    const { v4, v6 } = addrsOf(i);
    return [...v4, ...v6].join(' ');
  }

  /* 生命周期操作 */
  let acting = $state('');
  async function doAction(name: string, action: 'start' | 'stop' | 'restart') {
    acting = name + action;
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

  /* 删除 */
  let delTarget = $state<InstanceInfo | null>(null);
  let delOpen = $state(false);
  let delBusy = $state(false);
  async function doDelete() {
    if (!delTarget) return;
    delBusy = true;
    try {
      // Incus 拒绝直接删除运行中的实例（带 ?force=true 也会返回 "Instance is running"），
      // 因此先停止再删除；停止失败则终止，避免误删。
      if (delTarget.status === 'Running') {
        toast.info(`正在停止 ${delTarget.name}…`);
        const stopOp = await api.setInstanceState(delTarget.name, 'stop');
        await pollOperation(stopOp.operation_id);
      }
      const op = await api.deleteInstance(delTarget.name);
      toast.info(`删除 ${delTarget.name}…`);
      await pollOperation(op.operation_id);
      toast.success(`实例「${delTarget.name}」已删除`);
      delOpen = false;
      delTarget = null;
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      delBusy = false;
    }
  }

  /* 创建实例 */
  let createOpen = $state(false);
  let creating = $state(false);
  let form = $state({
    name: '',
    type: 'container',
    image_source: 'remote' as 'remote' | 'local',
    image_alias: 'debian/12',
    image_fingerprint: '',
    image_server: 'https://images.linuxcontainers.org',
    cpu_limit: '2',
    memory_limit: '2GB',
    disk_size: '10GB',
    storage_pool: 'default',
    network: 'incusbr0',
  });

  /* 创建弹窗所需的下拉数据源 */
  let localImages = $state<ImageInfo[]>([]);
  let localImagesLoading = $state(false);
  let pools = $state<StoragePoolInfo[]>([]);
  let nets = $state<NetworkInfo[]>([]);

  const localImagesForType = $derived(
    localImages.filter((img) => img.type === form.type),
  );

  async function openCreate() {
    createOpen = true;
    // 并行拉取本地镜像 / 存储池 / 网络，失败不阻断弹窗
    localImagesLoading = true;
    const [imgs, ps, ns] = await Promise.allSettled([
      api.images(),
      api.storagePools(),
      api.networks(),
    ]);
    if (imgs.status === 'fulfilled') localImages = imgs.value ?? [];
    if (ps.status === 'fulfilled' && ps.value?.length) {
      pools = ps.value;
      if (!pools.some((p) => p.name === form.storage_pool)) form.storage_pool = pools[0].name;
    }
    if (ns.status === 'fulfilled') nets = (ns.value ?? []).filter((n) => n.managed);
    localImagesLoading = false;
  }

  function selectLocalImage(img: ImageInfo) {
    form.image_fingerprint = img.fingerprint;
    form.image_alias = img.aliases[0]?.name ?? img.fingerprint.slice(0, 12);
  }

  function switchSource(src: 'remote' | 'local') {
    form.image_source = src;
  }

  const selectedLocal = $derived(
    localImages.find((i) => i.fingerprint === form.image_fingerprint) ?? null,
  );

  // 容器/虚拟机切换后旧镜像指纹可能不再适用，自动重选，避免提交类型不匹配的镜像
  $effect(() => {
    if (form.image_source !== 'local') return;
    const list = localImagesForType;
    if (list.length === 0) {
      form.image_fingerprint = '';
      return;
    }
    if (!list.some((img) => img.fingerprint === form.image_fingerprint)) {
      selectLocalImage(list[0]);
    }
  });

  async function submitCreate(e: SubmitEvent) {
    e.preventDefault();
    if (creating) return;
    if (form.image_source === 'local' && !form.image_fingerprint) {
      toast.error('请选择一个本地镜像');
      return;
    }
    creating = true;
    try {
      const op = await api.createInstance({
        name: form.name,
        type: form.type,
        image_source: form.image_source,
        image_alias: form.image_alias,
        image_fingerprint: form.image_source === 'local' ? form.image_fingerprint : '',
        image_server: form.image_source === 'remote' ? form.image_server : '',
        image_protocol: 'simplestreams',
        cpu_limit: form.cpu_limit,
        memory_limit: form.memory_limit,
        disk_size: form.disk_size,
        storage_pool: form.storage_pool,
        network: form.network,
        profiles: ['default'],
      });
      createOpen = false;
      toast.info(`正在创建 ${form.name}`, {
        description:
          form.image_source === 'local'
            ? `本地镜像 ${form.image_alias} · CPU ${form.cpu_limit} · 内存 ${form.memory_limit}`
            : `镜像 ${form.image_alias} · CPU ${form.cpu_limit} · 内存 ${form.memory_limit}`,
      });
      await pollOperation(op.operation_id);
      toast.success(`实例「${form.name}」创建成功`);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      creating = false;
    }
  }
</script>

<div class="animate-fade-up space-y-4">
  <!-- IP 地址单元格：IPv4 主显示，IPv6 次要显示；无全局地址时显示占位符 -->
  {#snippet addrCell(i: InstanceInfo)}
    {@const a = addrsOf(i)}
    {#if a.v4.length === 0 && a.v6.length === 0}
      <span class="font-mono text-[12px] text-muted-foreground">—</span>
    {:else}
      <div class="flex flex-col gap-0.5">
        {#each a.v4 as ip (ip)}
          <span class="font-mono text-[12px] whitespace-nowrap">{ip}</span>
        {/each}
        {#each a.v6 as ip (ip)}
          <span class="font-mono text-[11px] whitespace-nowrap text-muted-foreground" title="IPv6">{ip}</span>
        {/each}
      </div>
    {/if}
  {/snippet}

  <!-- 工具栏 -->
  <div class="flex flex-wrap items-center gap-2.5">
    <div class="relative min-w-[220px] flex-1 sm:max-w-xs">
      <Search size={14} class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground" />
      <Input bind:value={keyword} placeholder="搜索名称 / 配置文件 / IP…" class="h-9 pl-8.5 text-[13px]" />
    </div>

    <Select.Root type="single" bind:value={statusFilter}>
      <Select.Trigger class="h-9 w-[108px] text-[13px]">
        {statusFilter === 'all' ? '全部状态' : statusFilter === 'Running' ? '运行中' : '已停止'}
      </Select.Trigger>
      <Select.Content>
        <Select.Item value="all">全部状态</Select.Item>
        <Select.Item value="Running">运行中</Select.Item>
        <Select.Item value="Stopped">已停止</Select.Item>
      </Select.Content>
    </Select.Root>

    <Select.Root type="single" bind:value={typeFilter}>
      <Select.Trigger class="h-9 w-[118px] text-[13px]">
        {typeFilter === 'all' ? '全部类型' : typeFilter === 'container' ? '容器' : '虚拟机'}
      </Select.Trigger>
      <Select.Content>
        <Select.Item value="all">全部类型</Select.Item>
        <Select.Item value="container">容器</Select.Item>
        <Select.Item value="virtual-machine">虚拟机</Select.Item>
      </Select.Content>
    </Select.Root>

    <div class="ml-auto flex items-center gap-2.5">
      <span class="font-num text-xs text-muted-foreground">{filtered.length} / {instances.length}</span>
      <Button size="sm" class="h-9 gap-1.5" onclick={openCreate}>
        <Plus size={15} />
        创建实例
      </Button>
    </div>
  </div>

  <!-- 列表 -->
  <Card class="shadow-soft">
    {#if loading}
      <div class="space-y-3 p-5">
        {#each Array(5) as _, i (i)}
          <div class="h-10 animate-pulse rounded-lg bg-foreground/[0.04]"></div>
        {/each}
      </div>
    {:else if filtered.length === 0}
      <div class="p-5">
        <EmptyState
          title={instances.length === 0 ? '还没有实例' : '没有匹配的实例'}
          desc={instances.length === 0 ? '创建第一个容器或虚拟机，开始使用 Incus。' : '调整搜索关键字或过滤器试试。'}
        >
          {#snippet icon()}<Boxes size={20} />{/snippet}
          {#snippet action()}
            {#if instances.length === 0}
              <Button size="sm" class="gap-1.5" onclick={openCreate}>
                <Plus size={15} />创建实例
              </Button>
            {/if}
          {/snippet}
        </EmptyState>
      </div>
    {:else}
      <div class="overflow-x-auto" in:fade={{ duration: 150 }}>
        <table class="w-full text-left text-[13px]">
          <thead>
            <tr class="border-b border-border/70 text-[11.5px] text-muted-foreground">
              <th class="py-3 pr-3 pl-5 font-medium">名称</th>
              <th class="py-3 pr-3 font-medium">状态</th>
              <th class="py-3 pr-3 font-medium">类型</th>
              <th class="py-3 pr-3 font-medium">系统</th>
              <th class="py-3 pr-3 font-medium">IP 地址</th>
              <th class="py-3 pr-3 font-medium">配置文件</th>
              <th class="py-3 pr-3 font-medium">创建时间</th>
              <th class="py-3 pr-3 font-medium">备注</th>
              <th class="py-3 pr-5 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each filtered as i (i.name)}
              <tr class="group border-b border-border/40 transition-colors last:border-0 hover:bg-foreground/[0.02]">
                <td class="py-3 pr-3 pl-5">
                  <button
                    class="flex items-center gap-1 font-medium hover:text-primary"
                    onclick={() => openDetail(i.name)}
                  >
                    {i.name}
                    <ChevronRight size={13} class="opacity-0 transition-opacity group-hover:opacity-60" />
                  </button>
                </td>
                <td class="py-3 pr-3">
                  <StatusDot state={i.status === 'Running' ? 'active' : 'stopped'} label={i.status === 'Running' ? '运行中' : '已停止'} ping={i.status === 'Running'} />
                </td>
                <td class="py-3 pr-3">
                  <TypeBadge label={i.type === 'virtual-machine' ? 'VM' : 'CT'} />
                </td>
                <td class="py-3 pr-3">
                  {#if osOf(i)}
                    <span class="whitespace-nowrap text-[12.5px]" title={i.config?.['image.description'] ?? osOf(i)}>
                      {osOf(i)}
                    </span>
                  {:else}
                    <span class="text-[12px] text-muted-foreground">—</span>
                  {/if}
                </td>
                <td class="py-3 pr-3">
                  {@render addrCell(i)}
                </td>
                <td class="py-3 pr-3">
                  <div class="flex flex-wrap gap-1">
                    {#each i.profiles as p (p)}
                      <Badge variant="secondary" class="px-1.5 py-0 text-[10.5px] font-normal">{p}</Badge>
                    {/each}
                  </div>
                </td>
                <td class="font-num py-3 pr-3 text-[12px] text-muted-foreground">{relativeTime(i.created_at)}</td>
                <td class="w-44 py-3 pr-3">
                  <button
                    class="group/note flex max-w-52 items-center gap-1 text-left"
                    title={(i.description ?? '') || '点击添加备注'}
                    onclick={() => openNote(i)}
                  >
                    {#if i.description}
                      <span class="truncate text-[12.5px] text-foreground/85 group-hover/note:text-primary">{i.description}</span>
                      <Pencil size={12} class="shrink-0 text-muted-foreground/35 transition-colors group-hover/note:text-primary" />
                    {:else}
                      <span class="truncate text-[12px] text-transparent transition-colors group-hover/note:text-muted-foreground">+ 添加备注</span>
                    {/if}
                  </button>
                </td>
                <td class="py-3 pr-5">
                  <div class="flex items-center justify-end gap-0.5">
                    {#if i.status === 'Stopped'}
                      <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" disabled={acting === i.name + 'start'} onclick={() => doAction(i.name, 'start')} aria-label="开机" title="开机">
                        <Power size={14} class="text-success" />
                      </Button>
                    {:else}
                      <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" disabled={acting === i.name + 'stop'} onclick={() => doAction(i.name, 'stop')} aria-label="关机" title="关机">
                        <Power size={14} class="text-destructive" />
                      </Button>
                    {/if}
                    <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" disabled={acting === i.name + 'restart'} onclick={() => doAction(i.name, 'restart')} aria-label="重启" title="重启">
                      <RotateCw size={14} />
                    </Button>
                    <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" onclick={() => openDetail(i.name, 'terminal')} aria-label="终端" title="终端">
                      <SquareTerminal size={14} />
                    </Button>
                    <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground hover:text-destructive" onclick={() => { delTarget = i; delOpen = true; }} aria-label="删除" title="删除">
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

<!-- 创建实例弹窗 -->
<Dialog.Root bind:open={createOpen}>
  <Dialog.Content class="sm:max-w-lg">
    <Dialog.Header>
      <Dialog.Title>创建实例</Dialog.Title>
      <Dialog.Description>可直接引用本地已有镜像，或从远程镜像源拉取后创建容器 / 虚拟机。</Dialog.Description>
    </Dialog.Header>

    <form id="create-instance-form" class="grid gap-4" onsubmit={submitCreate}>
      <div class="grid grid-cols-2 gap-3">
        <div class="grid gap-1.5">
          <Label for="fName">名称</Label>
          <Input id="fName" required bind:value={form.name} placeholder="my-instance" class="h-9 font-mono text-[13px]" />
        </div>
        <div class="grid gap-1.5">
          <Label>类型</Label>
          <Select.Root type="single" bind:value={form.type}>
            <Select.Trigger class="h-9 w-full text-[13px]">
              {form.type === 'container' ? '容器 container' : '虚拟机 VM'}
            </Select.Trigger>
            <Select.Content>
              <Select.Item value="container">容器 container</Select.Item>
              <Select.Item value="virtual-machine">虚拟机 virtual-machine</Select.Item>
            </Select.Content>
          </Select.Root>
        </div>
      </div>

      <div class="grid gap-1.5">
        <Label>镜像来源</Label>
        <div class="flex items-center gap-1 rounded-lg bg-muted p-1">
          <button
            type="button"
            class="h-7 flex-1 rounded-md px-3 text-[12.5px] font-medium transition-colors {form.image_source === 'local'
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground'}"
            onclick={() => switchSource('local')}
          >
            引用本地镜像{localImages.length ? `（${localImages.length}）` : ''}
          </button>
          <button
            type="button"
            class="h-7 flex-1 rounded-md px-3 text-[12.5px] font-medium transition-colors {form.image_source === 'remote'
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground'}"
            onclick={() => switchSource('remote')}
          >
            从远程源拉取
          </button>
        </div>
      </div>

      {#if form.image_source === 'local'}
        <div class="grid gap-1.5">
          <Label for="fLocalImg">本地镜像</Label>
          {#if localImagesLoading}
            <div class="flex h-9 items-center gap-2 rounded-lg border border-input px-3 text-[12.5px] text-muted-foreground">
              <span class="inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
              读取本地镜像…
            </div>
          {:else if localImagesForType.length === 0}
            <div class="rounded-lg border border-dashed border-border/80 px-3 py-2.5 text-[12px] text-muted-foreground">
              本地没有可用于{form.type === 'container' ? '容器' : '虚拟机'}的镜像。请先到「镜像」页导入，或切换到远程源拉取。
            </div>
          {:else}
            <select
              id="fLocalImg"
              class="h-9 w-full rounded-lg border border-input bg-transparent px-2 font-mono text-[12.5px] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              value={form.image_fingerprint}
              onchange={(e) => {
                const img = localImages.find((i) => i.fingerprint === e.currentTarget.value);
                if (img) selectLocalImage(img);
              }}
            >
              {#each localImagesForType as img (img.fingerprint)}
                <option value={img.fingerprint}>
                  {img.aliases[0]?.name ?? img.fingerprint.slice(0, 12)} · {img.architecture} · {fmtBytes(img.size, 0)}
                </option>
              {/each}
            </select>
            {#if selectedLocal}
              <p class="font-mono text-[10.5px] text-muted-foreground">
                指纹 {selectedLocal.fingerprint.slice(0, 16)}… · {selectedLocal.type === 'virtual-machine' ? 'VM' : 'CT'}
                {#if selectedLocal.aliases.length > 1}
                  · 别名 {selectedLocal.aliases.map((a) => a.name).join(', ')}
                {/if}
              </p>
            {/if}
          {/if}
        </div>
      {:else}
        <div class="grid gap-1.5">
          <Label for="fAlias">镜像别名</Label>
          <Input id="fAlias" required bind:value={form.image_alias} placeholder="debian/12" class="h-9 font-mono text-[13px]" />
        </div>
        <div class="grid gap-1.5">
          <Label for="fServer">镜像服务器</Label>
          <Input id="fServer" bind:value={form.image_server} class="h-9 font-mono text-[12.5px]" />
        </div>
      {/if}

      <div class="grid grid-cols-3 gap-3">
        <div class="grid gap-1.5">
          <Label for="fCPU">CPU 限制</Label>
          <Input id="fCPU" bind:value={form.cpu_limit} class="h-9 text-[13px]" />
        </div>
        <div class="grid gap-1.5">
          <Label for="fMem">内存</Label>
          <Input id="fMem" bind:value={form.memory_limit} class="h-9 text-[13px]" />
        </div>
        <div class="grid gap-1.5">
          <Label for="fDisk">磁盘</Label>
          <Input id="fDisk" bind:value={form.disk_size} class="h-9 text-[13px]" />
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div class="grid gap-1.5">
          <Label for="fPool">存储池</Label>
          {#if pools.length > 0}
            <select
              id="fPool"
              class="h-9 w-full rounded-lg border border-input bg-transparent px-2 font-mono text-[12.5px] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              bind:value={form.storage_pool}
            >
              {#each pools as p (p.name)}
                <option value={p.name}>{p.name}（{p.driver}）</option>
              {/each}
            </select>
          {:else}
            <Input id="fPool" bind:value={form.storage_pool} class="h-9 font-mono text-[13px]" />
          {/if}
        </div>
        <div class="grid gap-1.5">
          <Label for="fNet">网络</Label>
          {#if nets.length > 0}
            <select
              id="fNet"
              class="h-9 w-full rounded-lg border border-input bg-transparent px-2 font-mono text-[12.5px] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              bind:value={form.network}
            >
              {#each nets as n (n.name)}
                <option value={n.name}>{n.name}</option>
              {/each}
            </select>
          {:else}
            <Input id="fNet" bind:value={form.network} class="h-9 font-mono text-[13px]" />
          {/if}
        </div>
      </div>
    </form>

    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" disabled={creating} onclick={() => (createOpen = false)}>取消</Button>
      <Button type="submit" form="create-instance-form" disabled={creating}>
        {#if creating}
          <span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
          创建中…
        {:else}
          创建
        {/if}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

{#if delTarget}
  <ConfirmDialog
    bind:open={delOpen}
    title="删除实例「{delTarget.name}」"
    description={delTarget.status === 'Running'
      ? '实例将先被停止再删除；其根磁盘与所有快照都将被永久删除，此操作不可恢复。'
      : '实例的根磁盘与所有快照都将被永久删除，此操作不可恢复。'}
    busy={delBusy}
    onconfirm={doDelete}
  />
{/if}

<!-- 编辑备注模态框 -->
<Dialog.Root open={!!noteTarget} onOpenChange={(v) => !v && (noteTarget = null)}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title class="flex items-center gap-2">
        <Pencil size={16} class="text-primary" />
        编辑备注
      </Dialog.Title>
      <Dialog.Description class="font-mono">{noteTarget?.name}</Dialog.Description>
    </Dialog.Header>

    <textarea
      bind:value={noteDraft}
      rows={4}
      class="w-full resize-y rounded-lg border border-input bg-transparent px-3 py-2 text-[13px] shadow-xs outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
      placeholder="备注内容（保存为实例 description，incus list 可见）"
      disabled={noteSaving}
      onkeydown={(e) => {
        if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) saveNote();
      }}
    ></textarea>
    <p class="-mt-2 text-[11px] text-muted-foreground">Ctrl + Enter 快速保存</p>

    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" disabled={noteSaving} onclick={() => (noteTarget = null)}>取消</Button>
      <Button disabled={noteSaving} onclick={saveNote}>
        {#if noteSaving}
          <span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
          保存中…
        {:else}
          保存
        {/if}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
