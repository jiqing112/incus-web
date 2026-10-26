<script lang="ts">
  import { fade } from 'svelte/transition';
  import { Search, Disc3, Trash2, Plus, RefreshCw, Download, Fingerprint, Globe, Loader2, ExternalLink, Upload, Database, Check } from '@lucide/svelte';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Badge } from '$lib/components/ui/badge';
  import * as Dialog from '$lib/components/ui/dialog';
  import { Label } from '$lib/components/ui/label';
  import { toast } from 'svelte-sonner';
  import EmptyState from '$lib/components/common/EmptyState.svelte';
  import TypeBadge from '$lib/components/common/TypeBadge.svelte';
  import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
  import { api, pollOperation, getToken } from '$lib/api';
  import type { ImageRemote, RemoteImage } from '$lib/types';
  import type { ImageInfo } from '$lib/types';
  import { fmtBytes, fmtDateTime, relativeTime } from '$lib/utils';
  import { bus } from '$lib/components/layout/state.svelte';

  let images = $state<ImageInfo[]>([]);
  let loading = $state(true);
  let keyword = $state('');

  async function load() {
    loading = true;
    try {
      images = (await api.images()) ?? [];
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
    if (!kw) return images;
    return images.filter(
      (img) =>
        img.aliases.some((a) => a.name.toLowerCase().includes(kw)) ||
        img.fingerprint.toLowerCase().includes(kw) ||
        (img.properties?.os ?? '').toLowerCase().includes(kw),
    );
  });

  function primaryAlias(img: ImageInfo): string {
    return img.aliases[0]?.name ?? img.fingerprint.slice(0, 12);
  }

  /* 删除 */
  let delTarget = $state<ImageInfo | null>(null);
  let delOpen = $state(false);
  let delBusy = $state(false);
  async function doDelete() {
    if (!delTarget) return;
    delBusy = true;
    try {
      await api.deleteImage(delTarget.fingerprint);
      toast.success(`镜像「${primaryAlias(delTarget)}」已删除`);
      delOpen = false;
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      delBusy = false;
    }
  }

  /* 导入：远程拉取 / 本地上传 */
  let importMode = $state<'remote' | 'local'>('remote');
  let uploadInputEl = $state<HTMLInputElement | null>(null);
  let uploadFile = $state<File | null>(null);
  let uploadProgress = $state(0);
  let uploadingLocal = $state(false);

  function onPickUploadFile(e: Event) {
    const input = e.target as HTMLInputElement;
    uploadFile = input.files?.[0] ?? null;
    uploadProgress = 0;
  }

  function uploadImageFile(file: File): Promise<{ operation_id: string }> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('POST', '/api/images/upload');
      xhr.setRequestHeader('Authorization', `Bearer ${getToken()}`);
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) uploadProgress = Math.round((e.loaded / e.total) * 100);
      };
      xhr.onload = () => {
        try {
          const j = JSON.parse(xhr.responseText);
          if (j.code === 200) resolve(j.data);
          else reject(new Error(j.error || `上传失败（HTTP ${xhr.status}）`));
        } catch {
          reject(new Error(`上传失败（HTTP ${xhr.status}）`));
        }
      };
      xhr.onerror = () => reject(new Error('网络错误，上传中断'));
      const form = new FormData();
      form.append('file', file);
      xhr.send(form);
    });
  }

  async function doUploadLocal() {
    if (!uploadFile || uploadingLocal) return;
    uploadingLocal = true;
    uploadProgress = 0;
    try {
      const op = await uploadImageFile(uploadFile);
      toast.info('上传完成，Incus 正在后台解包导入', { description: '完成后镜像会出现在列表中' });
      try {
        await pollOperation(op?.operation_id);
        toast.success('镜像导入成功');
      } catch {
        toast.warning('导入仍在后台进行中', { description: '完成后会自动出现在列表，可稍后刷新查看' });
      }
      uploadFile = null;
      if (uploadInputEl) uploadInputEl.value = '';
      importOpen = false;
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      uploadingLocal = false;
    }
  }

  /* 导入 */
  const COMMON_ALIASES = ['debian/12', 'debian/13', 'ubuntu/24.04', 'ubuntu/22.04', 'alpine/3.20', 'fedora/40'];
  let importOpen = $state(false);
  let impServer = $state('https://images.linuxcontainers.org');
  let impAlias = $state('debian/13');
  let importing = $state(false);
  async function submitImport(e: SubmitEvent) {
    e.preventDefault();
    importing = true;
    try {
      const op = await api.importImage(impServer, impAlias);
      importOpen = false;
      toast.info(`导入任务已提交：${impAlias}`, { description: '取决于网络与镜像大小，可能需要几分钟；完成后会出现在列表中' });
      try {
        await pollOperation(op?.operation_id);
        toast.success(`镜像 ${impAlias} 已就绪`);
      } catch (err) {
        // 拉取大镜像可能超过等待上限：任务仍在后台进行
        toast.warning('导入仍在后台进行中', { description: '完成后镜像会自动出现在列表，可稍后刷新查看' });
      }
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      importing = false;
    }
  }

  /* 远程源与远程镜像搜索 */
  // 注：清华/中科大等国内高校镜像站的 lxc-images 服务已陆续下线，官方源当前可直连使用。
  // 如有自建或内网 simplestreams 源，可在此添加预设供一键配置。
  const PRESET_REMOTES: { name: string; label: string; url: string }[] = [];
  let remotes = $state<ImageRemote[]>([]);
  let remotesLoading = $state(false);
  let remotesOpen = $state(false);
  let newRemote = $state({ name: '', url: '' });
  let remoteBusy = $state(false);

  let searchOpen = $state(false);
  let searchRemote = $state('');
  let searchKeyword = $state('');
  let searchResults = $state<RemoteImage[] | null>(null);
  let searching = $state(false);
  let importingFp = $state('');
  let searchInputEl = $state<HTMLInputElement | null>(null);
  /** 服务端缓存元信息（是否命中缓存 / 后台是否在刷新 / 缓存时间） */
  let cacheMeta = $state<{ cached: boolean; refreshing: boolean; updated_at: string; total: number } | null>(null);
  let forceRefreshing = $state(false);
  let recheckTimer: ReturnType<typeof setTimeout> | null = null;

  function focusSearchInput() {
    // 等待 Dialog 完成挂载与 bits-ui 的默认聚焦后，再把焦点交还给搜索框
    setTimeout(() => searchInputEl?.focus(), 120);
  }

  async function loadRemotes() {
    remotesLoading = true;
    try {
      const res = await api.imageRemotes();
      remotes = res?.remotes ?? [];
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      remotesLoading = false;
    }
  }

  /** 删除远程源（此前模板里调用了未定义的 removeRemote，点击即抛 ReferenceError） */
  async function removeRemote(r: ImageRemote) {
    if (remoteBusy) return;
    if (!window.confirm(`删除远程源「${r.name}」？其缓存索引也会一并清除。`)) return;
    remoteBusy = true;
    try {
      await api.removeImageRemote(r.name);
      toast.success(`已删除远程源 ${r.name}`);
      if (searchRemote === r.name) {
        searchRemote = '';
        searchResults = null;
        cacheMeta = null;
      }
      await loadRemotes();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      remoteBusy = false;
    }
  }

  async function openSearch() {
    searchOpen = true;
    focusSearchInput();
    if (remotes.length === 0) await loadRemotes();
    if (!searchRemote) {
      const preferred = remotes.find((r) => r.name === 'tuna') ?? remotes.find((r) => r.name === 'images') ?? remotes[0];
      if (preferred) {
        searchRemote = preferred.name;
        // 命中服务端缓存时这里会立即返回，无需等待十几秒
        void doSearch();
      }
    }
  }

  /**
   * 搜索远程镜像。
   * 默认走缓存（服务端命中磁盘缓存即秒回）；force=true 时绕过缓存强制拉取最新索引。
   */
  async function doSearch(opts: { force?: boolean; silent?: boolean } = {}) {
    if (!searchRemote) return;
    const force = opts.force ?? false;
    const silent = opts.silent ?? false;
    if (force) forceRefreshing = true;
    else if (!silent) {
      searching = true;
      searchResults = null;
      cacheMeta = null;
    }
    const target = searchRemote;
    try {
      const res = force
        ? await api.refreshRemoteImages(target, searchKeyword)
        : await api.remoteImages(target, searchKeyword);
      if (searchRemote !== target) return; // 期间切换了远程源，丢弃过期响应
      searchResults = res?.images ?? [];
      cacheMeta = {
        cached: res?.cached ?? false,
        refreshing: res?.refreshing ?? false,
        updated_at: res?.updated_at ?? '',
        total: res?.total ?? 0,
      };
      if (res?.error && !silent) {
        toast.warning(`索引刷新失败：${res.error}`, { description: '当前展示的是最近一次成功的缓存数据' });
      }
      // 服务端正在后台刷新时，稍后静默重取一次拿到最新数据
      if (res?.refreshing) scheduleRecheck();
    } catch (e) {
      if (!silent) toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      if (!silent) searching = false;
      forceRefreshing = false;
      if (!silent) focusSearchInput();
    }
  }

  function scheduleRecheck() {
    if (recheckTimer) clearTimeout(recheckTimer);
    recheckTimer = setTimeout(() => {
      if (searchOpen && searchRemote) void doSearch({ silent: true });
    }, 6000);
  }

  async function importFromRemote(img: RemoteImage) {
    const r = remotes.find((x) => x.name === searchRemote);
    if (!r) return;
    importingFp = img.fingerprint;
    try {
      const op = await api.importImage(r.addr, img.alias);
      toast.info(`导入任务已提交：${img.alias}`, { description: '完成后会出现在本地列表' });
      try {
        await pollOperation(op?.operation_id);
        toast.success(`镜像 ${img.alias} 已就绪`);
      } catch {
        toast.warning('导入仍在后台进行中', { description: '完成后会自动出现在本地列表' });
      }
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      importingFp = '';
    }
  }

  function fmtSize(n: number): string {
    if (!n) return '—';
    const units = ['B', 'KB', 'MB', 'GB'];
    let v = n, i = 0;
    while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
    return `${v.toFixed(0)} ${units[i]}`;
  }

  /* 导出 */
  let exporting = $state('');
  function doExport(img: ImageInfo) {
    exporting = img.fingerprint;
    try {
      // 后端返回 tar 流，走 JSON 请求会直接报错；用隐藏链接触发浏览器原生下载
      const a = document.createElement('a');
      a.href = api.imageExportURL(img.fingerprint, 'tar.gz');
      a.download = `${img.fingerprint.slice(0, 12)}.tar.gz`;
      a.rel = 'noopener';
      document.body.appendChild(a);
      a.click();
      a.remove();
      toast.success(`已开始下载 ${primaryAlias(img)}`, { description: '浏览器将保存到本地下载目录' });
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      exporting = '';
    }
  }
</script>

<div class="animate-fade-up space-y-4">
  <div class="flex flex-wrap items-center gap-2.5">
    <div class="relative min-w-[220px] flex-1 sm:max-w-xs">
      <Search size={14} class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground" />
      <Input bind:value={keyword} placeholder="搜索别名 / 指纹 / 系统…" class="h-9 pl-8.5 text-[13px]" />
    </div>
    <div class="ml-auto flex items-center gap-2.5">
      <span class="font-num text-xs text-muted-foreground">{filtered.length} 个镜像</span>
      <Button size="sm" variant="outline" class="h-9 gap-1.5" onclick={openSearch}>
        <Search size={14} />
        搜索远程镜像
      </Button>
      <Button size="sm" variant="outline" class="h-9 gap-1.5" onclick={() => { remotesOpen = true; loadRemotes(); }}>
        <Globe size={14} />
        远程源
      </Button>
      <Button size="sm" class="h-9 gap-1.5" onclick={() => (importOpen = true)}>
        <Plus size={15} />
        导入镜像
      </Button>
    </div>
  </div>

  {#if loading}
    <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
      {#each Array(6) as _, i (i)}
        <div class="h-40 animate-pulse rounded-xl bg-foreground/[0.04]"></div>
      {/each}
    </div>
  {:else if filtered.length === 0}
    <Card class="p-5 shadow-soft">
      <EmptyState
        title={images.length === 0 ? '本地没有镜像' : '没有匹配的镜像'}
        desc={images.length === 0 ? '从远程镜像源导入一个系统镜像，即可创建实例。' : '调整搜索关键字试试。'}
      >
        {#snippet icon()}<Disc3 size={20} />{/snippet}
        {#snippet action()}
          <Button size="sm" variant="outline" class="gap-1.5" onclick={() => (importOpen = true)}>
            <Plus size={15} />导入镜像
          </Button>
        {/snippet}
      </EmptyState>
    </Card>
  {:else}
    <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3" in:fade={{ duration: 150 }}>
      {#each filtered as img (img.fingerprint)}
        <div class="lift-card group rounded-xl border border-border/70 bg-card p-5 transition-transform duration-200 hover:-translate-y-0.5">
          <div class="flex items-start justify-between gap-3">
            <div class="flex min-w-0 items-center gap-3">
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-primary/15 to-info/15 text-primary">
                <Disc3 size={19} />
              </span>
              <div class="min-w-0">
                <div class="truncate font-mono text-[13.5px] font-semibold tracking-tight">{primaryAlias(img)}</div>
                <div class="mt-0.5 flex items-center gap-1.5">
                  <TypeBadge label={img.type === 'virtual-machine' ? 'VM' : 'CT'} />
                  <span class="font-mono text-[10.5px] text-muted-foreground">{img.architecture}</span>
                </div>
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-0.5 opacity-60 transition-opacity group-hover:opacity-100">
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" title="导出" disabled={exporting === img.fingerprint} onclick={() => doExport(img)}>
                <Download size={14} />
              </Button>
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground hover:text-destructive" title="删除" onclick={() => { delTarget = img; delOpen = true; }}>
                <Trash2 size={14} />
              </Button>
            </div>
          </div>

          <div class="mt-4 flex flex-wrap gap-1.5">
            {#if img.cached}<Badge variant="outline" class="bg-info/8 px-1.5 text-[10px] text-info ring-info/20">缓存</Badge>{/if}
            {#if img.public}<Badge variant="outline" class="bg-success/8 px-1.5 text-[10px] text-success ring-success/20">公开</Badge>{/if}
            {#if img.auto_update}<Badge variant="outline" class="bg-primary/8 px-1.5 text-[10px] text-primary ring-primary/20">自动更新</Badge>{/if}
            {#each img.aliases.slice(1, 3) as a (a.name)}
              <Badge variant="secondary" class="px-1.5 font-mono text-[10px] font-normal">{a.name}</Badge>
            {/each}
          </div>

          <dl class="mt-4 space-y-1.5 border-t border-border/60 pt-3 text-[11.5px]">
            <div class="flex items-center justify-between gap-3">
              <dt class="flex items-center gap-1 text-muted-foreground"><Fingerprint size={11} />指纹</dt>
              <dd class="truncate font-mono text-foreground/75" title={img.fingerprint}>{img.fingerprint.slice(0, 16)}…</dd>
            </div>
            <div class="flex items-center justify-between">
              <dt class="text-muted-foreground">大小</dt>
              <dd class="font-num font-medium">{fmtBytes(img.size, 0)}</dd>
            </div>
            <div class="flex items-center justify-between">
              <dt class="text-muted-foreground">构建时间</dt>
              <dd class="font-num text-foreground/75">{fmtDateTime(img.created_at)}</dd>
            </div>
          </dl>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- 导入镜像 -->
<Dialog.Root bind:open={importOpen}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title>导入远程镜像</Dialog.Title>
      <Dialog.Description>从公共镜像源（simplestreams）拉取系统镜像。</Dialog.Description>
    </Dialog.Header>
    <div class="mb-4 flex items-center gap-1 rounded-lg bg-muted p-1">
      <button
        type="button"
        class="h-7 flex-1 rounded-md px-3 text-[12.5px] font-medium transition-colors {importMode === 'remote'
          ? 'bg-background text-foreground shadow-sm'
          : 'text-muted-foreground hover:text-foreground'}"
        onclick={() => (importMode = 'remote')}
      >
        从远程源拉取
      </button>
      <button
        type="button"
        class="h-7 flex-1 rounded-md px-3 text-[12.5px] font-medium transition-colors {importMode === 'local'
          ? 'bg-background text-foreground shadow-sm'
          : 'text-muted-foreground hover:text-foreground'}"
        onclick={() => (importMode = 'local')}
      >
        上传本地镜像文件
      </button>
    </div>

    {#if importMode === 'remote'}
    <form id="import-form" class="grid gap-4" onsubmit={submitImport}>
      <div class="grid gap-1.5">
        <Label for="impServer">镜像服务器</Label>
        <Input id="impServer" bind:value={impServer} class="h-9 font-mono text-[12.5px]" />
      </div>
      <div class="grid gap-1.5">
        <Label for="impAlias">镜像别名</Label>
        <Input id="impAlias" required bind:value={impAlias} placeholder="debian/13" class="h-9 font-mono text-[13px]" />
        <div class="flex flex-wrap gap-1 pt-0.5">
          {#each COMMON_ALIASES as a (a)}
            <button
              type="button"
              class="rounded-md border px-2 py-0.5 font-mono text-[11px] transition-colors {impAlias === a
                ? 'border-primary/40 bg-primary/10 text-primary'
                : 'border-border/70 text-muted-foreground hover:border-primary/30 hover:text-foreground'}"
              onclick={() => (impAlias = a)}
            >
              {a}
            </button>
          {/each}
        </div>
      </div>
    </form>
    {:else}
    <div class="space-y-4">
      <input
        type="file"
        bind:this={uploadInputEl}
        onchange={onPickUploadFile}
        accept=".tar.gz,.tgz,.tar.xz,.tar.bz2,.tar,.qcow2,.img"
        class="hidden"
      />
      <button
        type="button"
        class="flex w-full flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border/80 px-4 py-8 transition-colors hover:border-primary/40 hover:bg-foreground/[0.02]"
        onclick={() => uploadInputEl?.click()}
      >
        <Upload size={22} class="text-primary" />
        <span class="text-[13px] font-medium">
          {uploadFile ? uploadFile.name : '点击选择镜像文件'}
        </span>
        <span class="text-[11px] text-muted-foreground">
          {#if uploadFile}
            {fmtSize(uploadFile.size)} · 支持合并格式 tarball（tar.gz / tar.xz）与 qcow2
          {:else}
            支持合并格式镜像包：tar.gz / tar.xz / tar.bz2 / qcow2
          {/if}
        </span>
      </button>

      {#if uploadingLocal}
        <div>
          <div class="mb-1 flex justify-between text-[11.5px] text-muted-foreground">
            <span>上传中…</span>
            <span class="font-num">{uploadProgress}%</span>
          </div>
          <div class="h-[6px] overflow-hidden rounded-full bg-foreground/[0.05]">
            <div class="h-full rounded-full bg-primary transition-[width] duration-200" style="width:{uploadProgress}%"></div>
          </div>
        </div>
      {/if}

      <p class="text-[11px] leading-relaxed text-muted-foreground">
        上传到面板所在机器后由 Incus 解包导入，导入完成后镜像会以指纹标识出现在列表中。
      </p>
    </div>
    {/if}

    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" disabled={importing || uploadingLocal} onclick={() => (importOpen = false)}>取消</Button>
      {#if importMode === 'remote'}
        <Button type="submit" form="import-form" disabled={importing}>
          {#if importing}<span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>{/if}
          开始导入
        </Button>
      {:else}
        <Button disabled={!uploadFile || uploadingLocal} onclick={doUploadLocal}>
          {#if uploadingLocal}
            <span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
            上传中（{uploadProgress}%）
          {:else}
            <Upload size={14} class="mr-1" /> 开始上传并导入
          {/if}
        </Button>
      {/if}
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

{#if delTarget}
  <ConfirmDialog
    bind:open={delOpen}
    title="删除镜像「{primaryAlias(delTarget)}」"
    description="已创建的实例不受影响，但新实例将无法使用该镜像。"
    busy={delBusy}
    onconfirm={doDelete}
  />
{/if}

<!-- 远程镜像搜索 -->
<Dialog.Root bind:open={searchOpen}>
  <Dialog.Content class="sm:max-w-3xl">
    <Dialog.Header>
      <Dialog.Title class="flex items-center gap-2">
        <Globe size={16} class="text-primary" />
        搜索远程镜像
      </Dialog.Title>
      <Dialog.Description>从已配置的镜像源搜索系统镜像，点击导入即自动拉取到本地。</Dialog.Description>
    </Dialog.Header>

    <div class="flex flex-wrap items-center gap-2">
      <select
        bind:value={searchRemote}
        onchange={() => doSearch()}
        class="h-9 rounded-lg border border-input bg-transparent px-2 text-[13px] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
      >
        {#each remotes as r (r.name)}
          <option value={r.name}>{r.name}（{r.addr.replace('https://', '').replace('http://', '')}）</option>
        {/each}
      </select>
      <div class="relative min-w-[180px] flex-1">
        <Search size={14} class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground" />
        <input
          bind:this={searchInputEl}
          bind:value={searchKeyword}
          onkeydown={(e) => { if (e.key === 'Enter') doSearch(); }}
          placeholder="搜索系统名 / 版本，如 debian、alpine、ubuntu 24"
          class="h-9 w-full rounded-lg border border-input bg-transparent pl-8.5 pr-3 text-[13px] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
        />
      </div>
      <Button size="sm" class="h-9 gap-1.5" disabled={searching} onclick={() => doSearch()}>
        {#if searching}
          <Loader2 size={13} class="animate-spin" /> 搜索中…
        {:else}
          <Search size={13} /> 搜索
        {/if}
      </Button>
      <Button
        size="sm"
        variant="outline"
        class="h-9 gap-1.5"
        disabled={forceRefreshing || searching || !searchRemote}
        onclick={() => doSearch({ force: true })}
        title="忽略缓存，重新拉取该远程源的最新索引"
      >
        {#if forceRefreshing}
          <Loader2 size={13} class="animate-spin" /> 刷新中…
        {:else}
          <RefreshCw size={13} /> 刷新索引
        {/if}
      </Button>
    </div>

    {#if cacheMeta}
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
        {#if cacheMeta.refreshing}
          <span class="inline-flex items-center gap-1 text-primary">
            <Loader2 size={11} class="animate-spin" /> 后台刷新中，稍后自动更新
          </span>
        {:else if cacheMeta.cached}
          <span class="inline-flex items-center gap-1">
            <Database size={11} /> 缓存命中（秒开）
          </span>
        {:else}
          <span class="inline-flex items-center gap-1 text-success">
            <Check size={11} /> 已实时更新
          </span>
        {/if}
        {#if cacheMeta.updated_at}
          <span>索引更新于 {relativeTime(cacheMeta.updated_at)}</span>
        {/if}
        <span>原始索引 {cacheMeta.total} 条 · 展示 {searchResults?.length ?? 0} 条</span>
      </div>
    {/if}

    <div class="max-h-[46vh] overflow-y-auto rounded-xl border border-border/60">
      {#if searching}
        <div class="flex items-center justify-center gap-2 py-10 text-[13px] text-muted-foreground">
          <Loader2 size={15} class="animate-spin" />
          正在拉取远程镜像索引（首次约十几秒，之后命中缓存秒开）…
        </div>
      {:else if searchResults === null}
        <div class="py-10 text-center text-[13px] text-muted-foreground">输入关键字后点击搜索。</div>
      {:else if searchResults.length === 0}
        <div class="py-10 text-center text-[13px] text-muted-foreground">没有匹配的镜像。</div>
      {:else}
        <table class="w-full text-left text-[12.5px]">
          <thead>
            <tr class="border-b border-border/60 bg-foreground/[0.02] text-[10.5px] text-muted-foreground">
              <th class="px-3 py-2 font-medium">别名</th>
              <th class="px-3 py-2 font-medium">描述</th>
              <th class="px-3 py-2 text-right font-medium">大小</th>
              <th class="px-3 py-2 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each searchResults as img (img.fingerprint)}
              <tr class="border-b border-border/40 transition-colors last:border-0 hover:bg-foreground/[0.02]">
                <td class="px-3 py-2 font-mono text-[12px] font-medium">{img.alias}</td>
                <td class="px-3 py-2 text-muted-foreground">
                  <span class="block max-w-[320px] truncate" title={img.description}>{img.description || img.os + ' ' + img.release}</span>
                </td>
                <td class="font-num px-3 py-2 text-right">{fmtSize(img.size)}</td>
                <td class="px-3 py-2 text-right">
                  <Button size="sm" variant="outline" class="h-7 gap-1 text-[12px]" disabled={importingFp !== ''} onclick={() => importFromRemote(img)}>
                    {#if importingFp === img.fingerprint}
                      <Loader2 size={12} class="animate-spin" /> 导入中
                    {:else}
                      <Download size={12} /> 导入
                    {/if}
                  </Button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  </Dialog.Content>
</Dialog.Root>

<!-- 远程源管理 -->
<Dialog.Root bind:open={remotesOpen}>
  <Dialog.Content class="sm:max-w-lg">
    <Dialog.Header>
      <Dialog.Title class="flex items-center gap-2">
        <Globe size={16} class="text-primary" />
        镜像远程源
      </Dialog.Title>
      <Dialog.Description>添加 simplestreams 协议的镜像源（官方源已内置；国内高校镜像站的 lxc-images 服务已陆续下线）。</Dialog.Description>
    </Dialog.Header>

    <div class="rounded-xl border border-border/60">
      {#if remotesLoading}
        <div class="flex items-center justify-center gap-2 py-6 text-[13px] text-muted-foreground">
          <Loader2 size={14} class="animate-spin" /> 加载中…
        </div>
      {:else}
        <ul class="divide-y divide-border/40">
          {#each remotes as r (r.name)}
            <li class="flex items-center gap-3 px-3.5 py-2.5">
              <span class="font-mono text-[12.5px] font-medium">{r.name}</span>
              <span class="min-w-0 flex-1 truncate font-mono text-[11.5px] text-muted-foreground" title={r.addr}>{r.addr}</span>
              {#if r.static}
                <Badge variant="outline" class="shrink-0 px-1.5 text-[10px] text-muted-foreground ring-border">内置</Badge>
              {:else}
                <Button variant="ghost" size="icon" class="size-7 shrink-0 text-muted-foreground hover:text-destructive" disabled={remoteBusy} onclick={() => removeRemote(r)} aria-label="删除">
                  <Trash2 size={13} />
                </Button>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <form
      class="space-y-3"
      onsubmit={async (e) => {
        e.preventDefault();
        if (!newRemote.name || !newRemote.url || remoteBusy) return;
        remoteBusy = true;
        try {
          const r = await api.addImageRemote(newRemote.name, newRemote.url);
          toast.success(r.message || '已添加');
          newRemote = { name: '', url: '' };
          await loadRemotes();
          searchRemote = '';
        } catch (err) {
          toast.error(err instanceof Error ? err.message : String(err));
        } finally {
          remoteBusy = false;
        }
      }}
    >
      {#if PRESET_REMOTES.length > 0}
      <div class="flex flex-wrap gap-1">
        {#each PRESET_REMOTES as pr (pr.name)}
          <button
            type="button"
            class="flex items-center gap-1 rounded-md border px-2 py-1 text-[11.5px] transition-colors {newRemote.url === pr.url
              ? 'border-primary/40 bg-primary/10 text-primary'
              : 'border-border/70 text-muted-foreground hover:border-primary/30 hover:text-foreground'}"
            onclick={() => { newRemote.name = pr.name; newRemote.url = pr.url; }}
          >
            <ExternalLink size={10} /> {pr.label}
          </button>
        {/each}
      </div>
      {/if}
      <div class="flex gap-2">
        <Input bind:value={newRemote.name} required pattern="[a-zA-Z0-9][a-zA-Z0-9_-]*" placeholder="名称，如 tuna" class="h-9 w-32 font-mono text-[12.5px]" />
        <Input bind:value={newRemote.url} required placeholder="https://mirrors.tuna.tsinghua.edu.cn/lxc-images/" class="h-9 flex-1 font-mono text-[12px]" />
        <Button type="submit" size="sm" class="h-9 shrink-0 gap-1" disabled={remoteBusy}>
          <Plus size={13} /> 添加
        </Button>
      </div>
    </form>
  </Dialog.Content>
</Dialog.Root>
