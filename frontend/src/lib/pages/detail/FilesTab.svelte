<script lang="ts">
  import {
    Folder, File as FileIcon, ChevronRight, Upload, Download, Trash2, ArrowUp, Loader2,
  } from '@lucide/svelte';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { toast } from 'svelte-sonner';
  import EmptyState from '$lib/components/common/EmptyState.svelte';
  import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
  import { api } from '$lib/api';
  import type { FileInfo } from '$lib/types';
  import { fmtBytes, fmtDateTime } from '$lib/utils';

  let { name, running }: { name: string; running: boolean } = $props();

  let path = $state('/');
  let files = $state<FileInfo[]>([]);
  let loading = $state(false);

  const segments = $derived.by(() => {
    const parts = path.split('/').filter(Boolean);
    return parts.map((p, i) => ({ name: p, path: '/' + parts.slice(0, i + 1).join('/') }));
  });

  let loadSeq = 0;
  async function load(inst = name, dir = path) {
    const seq = ++loadSeq;
    loading = true;
    try {
      const res = await api.files(inst, dir);
      if (seq === loadSeq) files = res;
    } catch (e) {
      if (seq === loadSeq) {
        toast.error(e instanceof Error ? e.message : String(e));
        files = [];
      }
    } finally {
      if (seq === loadSeq) loading = false;
    }
  }

  // 切换目录只改 path，由 effect 统一加载（此前 enter/up 里再调一次 load，会重复请求）
  $effect(() => {
    void name;
    void path;
    if (!running) {
      loadSeq++;
      files = [];
      loading = false;
      return;
    }
    load(name, path);
  });

  function enter(f: FileInfo) {
    if (f.type === 'directory') path = f.path;
  }

  function up() {
    if (path === '/') return;
    path = segments.length <= 1 ? '/' : segments[segments.length - 2].path;
  }

  /* 上传 */
  let fileInput: HTMLInputElement | null = $state(null);
  let uploading = $state(false);
  async function onPick(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    uploading = true;
    try {
      await api.uploadFile(name, path, file);
      toast.success(`已上传 ${file.name}`);
      await load(name, path);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      uploading = false;
      input.value = '';
    }
  }

  /* 删除 */
  let delTarget = $state<FileInfo | null>(null);
  let delOpen = $state(false);
  let delBusy = $state(false);
  async function doDelete() {
    if (!delTarget) return;
    delBusy = true;
    try {
      await api.deleteFile(name, delTarget.path);
      toast.success(`已删除 ${delTarget.name}`);
      delOpen = false;
      await load(name, path);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      delBusy = false;
    }
  }
</script>

<Card class="shadow-soft">
  <!-- 路径工具栏 -->
  <div class="flex flex-wrap items-center gap-2 border-b border-border/60 px-4 py-3">
    <nav class="flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto text-[12.5px]" aria-label="路径">
      <button
        class="shrink-0 rounded-md px-2 py-1 font-mono font-medium transition-colors hover:bg-accent {path === '/' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground'}"
        onclick={() => (path = '/')}
      >
        /{name}
      </button>
      {#each segments as seg (seg.path)}
        <ChevronRight size={13} class="shrink-0 text-muted-foreground/50" />
        <button
          class="shrink-0 rounded-md px-2 py-1 font-mono transition-colors hover:bg-accent {seg.path === path ? 'bg-accent font-medium text-accent-foreground' : 'text-muted-foreground'}"
          onclick={() => (path = seg.path)}
        >
          {seg.name}
        </button>
      {/each}
    </nav>

    <div class="flex shrink-0 items-center gap-1.5">
      <Button variant="ghost" size="icon" class="size-8 text-muted-foreground" disabled={path === '/'} onclick={up} aria-label="上一级" title="上一级">
        <ArrowUp size={15} />
      </Button>
      <input type="file" bind:this={fileInput} onchange={onPick} class="hidden" aria-hidden="true" />
      <Button size="sm" variant="outline" class="h-8.5 gap-1.5" disabled={uploading} onclick={() => fileInput?.click()}>
        {#if uploading}
          <Loader2 size={14} class="animate-spin" />
        {:else}
          <Upload size={14} />
        {/if}
        上传
      </Button>
    </div>
  </div>

  <div class="p-4">
    {#if !running}
      <EmptyState title="实例未运行" desc="文件系统需要实例处于运行状态才能访问。" />
    {:else if loading}
      <div class="space-y-2">
        {#each Array(6) as _, i (i)}
          <div class="h-10 animate-pulse rounded-lg bg-foreground/[0.04]"></div>
        {/each}
      </div>
    {:else if files.length === 0}
      <EmptyState compact title="空目录" desc="该目录下没有文件，试试上传一个。" />
    {:else}
      <ul class="space-y-0.5">
        {#each files as f (f.path)}
          <li class="group flex items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-foreground/[0.03]">
            <button
              class="flex min-w-0 flex-1 cursor-{f.type === 'directory' ? 'pointer' : 'default'} items-center gap-3 text-left"
              onclick={() => enter(f)}
              disabled={f.type !== 'directory'}
            >
              <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg {f.type === 'directory' ? 'bg-primary/10 text-primary' : 'bg-foreground/[0.04] text-muted-foreground'}">
                {#if f.type === 'directory'}
                  <Folder size={15} />
                {:else}
                  <FileIcon size={15} />
                {/if}
              </span>
              <span class="min-w-0 flex-1">
                <span class="block truncate font-mono text-[12.5px] font-medium">{f.name}</span>
                <span class="block font-mono text-[10.5px] text-muted-foreground">{f.mode}</span>
              </span>
              <span class="font-num hidden w-24 shrink-0 text-right text-[11.5px] text-muted-foreground sm:block">
                {f.type === 'directory' ? '—' : fmtBytes(f.size, 1)}
              </span>
              <span class="font-num hidden w-36 shrink-0 text-right text-[11px] text-muted-foreground md:block">
                {fmtDateTime(f.modified)}
              </span>
            </button>
            <div class="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
              {#if f.type === 'file'}
                <a
                  class="inline-flex size-7.5 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                  href={api.fileDownloadURL(name, f.path)}
                  title="下载"
                  download
                >
                  <Download size={14} />
                </a>
              {/if}
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground hover:text-destructive" title="删除" onclick={() => { delTarget = f; delOpen = true; }}>
                <Trash2 size={14} />
              </Button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</Card>

{#if delTarget}
  <ConfirmDialog
    bind:open={delOpen}
    title="删除 {delTarget.type === 'directory' ? '目录' : '文件'}「{delTarget.name}」"
    description="该路径将从实例文件系统中永久删除。"
    busy={delBusy}
    onconfirm={doDelete}
  />
{/if}
