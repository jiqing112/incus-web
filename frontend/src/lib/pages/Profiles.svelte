<script lang="ts">
  import { fade } from 'svelte/transition';
  import { Search, IdCard, Trash2, Plus, Pencil, Boxes } from '@lucide/svelte';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Badge } from '$lib/components/ui/badge';
  import * as Dialog from '$lib/components/ui/dialog';
  import { Label } from '$lib/components/ui/label';
  import { toast } from 'svelte-sonner';
  import EmptyState from '$lib/components/common/EmptyState.svelte';
  import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
  import KeyValues from '$lib/components/common/KeyValues.svelte';
  import { api, pollOperation } from '$lib/api';
  import type { ProfileInfo } from '$lib/types';
  import { bus } from '$lib/components/layout/state.svelte';

  let profiles = $state<ProfileInfo[]>([]);
  let loading = $state(true);
  let keyword = $state('');

  async function load() {
    loading = true;
    try {
      profiles = (await api.profiles()) ?? [];
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
    if (!kw) return profiles;
    return profiles.filter((p) => p.name.toLowerCase().includes(kw) || (p.description ?? '').toLowerCase().includes(kw));
  });

  /* 详情 */
  let detail = $state<ProfileInfo | null>(null);

  /* 简易 KV 文本编辑：每行 key=value */
  function parseKV(text: string): Record<string, string> {
    const out: Record<string, string> = {};
    for (const line of text.split('\n')) {
      const t = line.trim();
      if (!t) continue;
      const i = t.indexOf('=');
      if (i > 0) out[t.slice(0, i).trim()] = t.slice(i + 1).trim();
    }
    return out;
  }
  function stringifyKV(obj: Record<string, string> | undefined): string {
    return Object.entries(obj ?? {}).map(([k, v]) => `${k}=${v}`).join('\n');
  }

  /* 创建/编辑 */
  let editorOpen = $state(false);
  let editingName = $state<string | null>(null); // null = 新建
  let pEditorTab = $state('configText');
  let pform = $state({ name: '', description: '', configText: '', devicesText: '' });
  let submitting = $state(false);

  function openCreate() {
    editingName = null;
    pform = { name: '', description: '', configText: 'limits.cpu=2\nlimits.memory=2GiB', devicesText: '' };
    editorOpen = true;
  }
  function openEdit(p: ProfileInfo) {
    editingName = p.name;
    pform = {
      name: p.name,
      description: p.description ?? '',
      configText: stringifyKV(p.config),
      devicesText: Object.entries(p.devices ?? {})
        .map(([dn, dv]) => `${dn}: ${Object.entries(dv).map(([k, v]) => `${k}=${v}`).join(' ')}`)
        .join('\n'),
    };
    editorOpen = true;
  }

  function parseDevices(text: string): Record<string, Record<string, string>> {
    const out: Record<string, Record<string, string>> = {};
    for (const line of text.split('\n')) {
      const t = line.trim();
      if (!t) continue;
      const ci = t.indexOf(':');
      if (ci <= 0) continue;
      const devName = t.slice(0, ci).trim();
      const rest = t.slice(ci + 1);
      const dev: Record<string, string> = {};
      // 解析 key=value 序列（值可能含点号/斜杠）
      const re = /(\S+?)=(\S+)/g;
      let m: RegExpExecArray | null;
      while ((m = re.exec(rest)) !== null) dev[m[1]] = m[2];
      out[devName] = dev;
    }
    return out;
  }

  async function submitEditor(e: SubmitEvent) {
    e.preventDefault();
    submitting = true;
    try {
      const body = {
        name: pform.name.trim(),
        description: pform.description,
        config: parseKV(pform.configText),
        devices: parseDevices(pform.devicesText),
      };
      if (editingName) {
        await api.updateProfile(editingName, body);
        toast.success(`配置文件「${editingName}」已更新`);
      } else {
        const op = await api.createProfile(body);
        await pollOperation(op.operation_id);
        toast.success(`配置文件「${body.name}」已创建`);
      }
      editorOpen = false;
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      submitting = false;
    }
  }

  /* 删除 */
  let delTarget = $state<ProfileInfo | null>(null);
  let delOpen = $state(false);
  let delBusy = $state(false);
  async function doDelete() {
    if (!delTarget) return;
    delBusy = true;
    try {
      const op = await api.deleteProfile(delTarget.name);
      await pollOperation(op.operation_id);
      toast.success(`配置文件「${delTarget.name}」已删除`);
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
      <Input bind:value={keyword} placeholder="搜索配置文件…" class="h-9 pl-8.5 text-[13px]" />
    </div>
    <div class="ml-auto flex items-center gap-2.5">
      <span class="font-num text-xs text-muted-foreground">{filtered.length} 个配置文件</span>
      <Button size="sm" class="h-9 gap-1.5" onclick={openCreate}>
        <Plus size={15} />
        新建配置文件
      </Button>
    </div>
  </div>

  {#if loading}
    <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
      {#each Array(4) as _, i (i)}
        <div class="h-36 animate-pulse rounded-xl bg-foreground/[0.04]"></div>
      {/each}
    </div>
  {:else if filtered.length === 0}
    <Card class="p-5 shadow-soft">
      <EmptyState title="没有匹配的配置文件" desc="配置文件是可复用的实例模板：资源限制、设备、启动参数。">
        {#snippet icon()}<IdCard size={20} />{/snippet}
      </EmptyState>
    </Card>
  {:else}
    <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3" in:fade={{ duration: 150 }}>
      {#each filtered as p (p.name)}
        <div class="lift-card group flex flex-col rounded-xl border border-border/70 bg-card p-5 transition-transform duration-200 hover:-translate-y-0.5">
          <div class="flex items-start justify-between gap-3">
            <div class="flex min-w-0 items-center gap-3">
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl {p.name === 'default' ? 'bg-primary/10 text-primary' : 'bg-foreground/[0.04] text-muted-foreground'}">
                <IdCard size={19} />
              </span>
              <div class="min-w-0">
                <div class="truncate font-mono text-[14px] font-semibold tracking-tight">{p.name}</div>
                <div class="mt-0.5 text-[10.5px] text-muted-foreground">{(p.used_by ?? []).length} 个实例引用</div>
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-0.5 opacity-60 transition-opacity group-hover:opacity-100">
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground" title="编辑" onclick={() => openEdit(p)}>
                <Pencil size={14} />
              </Button>
              <Button variant="ghost" size="icon" class="size-7.5 text-muted-foreground hover:text-destructive" title="删除" disabled={p.name === 'default'} onclick={() => { delTarget = p; delOpen = true; }}>
                <Trash2 size={14} />
              </Button>
            </div>
          </div>

          {#if p.description}
            <p class="mt-3 line-clamp-2 text-[12px] leading-relaxed text-muted-foreground">{p.description}</p>
          {:else}
            <p class="mt-3 text-[12px] italic text-muted-foreground/50">暂无描述</p>
          {/if}

          <div class="mt-auto pt-4">
            <div class="mb-1.5 text-[10.5px] text-muted-foreground">配置项</div>
            <div class="flex flex-wrap gap-1">
              {#each Object.keys(p.config ?? {}).slice(0, 4) as k (k)}
                <Badge variant="secondary" class="px-1.5 font-mono text-[10px] font-normal">{k}</Badge>
              {/each}
              {#if Object.keys(p.config ?? {}).length > 4}
                <Badge variant="outline" class="px-1.5 text-[10px] text-muted-foreground ring-border">+{Object.keys(p.config ?? {}).length - 4}</Badge>
              {/if}
            </div>
            <div class="mt-2 flex flex-wrap gap-1">
              {#each Object.keys(p.devices ?? {}) as dn (dn)}
                <Badge variant="outline" class="gap-1 bg-primary/6 px-1.5 font-mono text-[10px] text-primary ring-primary/15">
                  <Boxes size={9} />{dn}
                </Badge>
              {/each}
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- 详情 -->
<Dialog.Root open={!!detail} onOpenChange={(v) => !v && (detail = null)}>
  <Dialog.Content class="max-h-[85vh] sm:max-w-xl sm:overflow-y-auto">
    {#if detail}
      <Dialog.Header>
        <Dialog.Title class="font-mono">{detail.name}</Dialog.Title>
        <Dialog.Description>{detail.description || '配置文件详情'}</Dialog.Description>
      </Dialog.Header>
      <div class="space-y-5">
        <div>
          <div class="mb-2 text-[12px] font-medium text-muted-foreground">配置</div>
          <KeyValues entries={Object.entries(detail.config ?? {})} emptyText="无配置项" />
        </div>
        <div>
          <div class="mb-2 text-[12px] font-medium text-muted-foreground">设备</div>
          {#if Object.keys(detail.devices ?? {}).length > 0}
            <div class="space-y-2">
              {#each Object.entries(detail.devices ?? {}) as [dn, dv] (dn)}
                <div class="rounded-lg border border-border/60 px-3 py-2.5">
                  <div class="font-mono text-[12.5px] font-semibold">{dn} <span class="ml-1.5 text-[10.5px] font-normal text-muted-foreground">{dv.type}</span></div>
                  <div class="mt-1 font-mono text-[11px] text-muted-foreground">
                    {Object.entries(dv).filter(([k]) => k !== 'type').map(([k, v]) => `${k}=${v}`).join('  ')}
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-[12px] text-muted-foreground">无设备</p>
          {/if}
        </div>
        <div>
          <div class="mb-2 text-[12px] font-medium text-muted-foreground">被引用（{(detail.used_by ?? []).length}）</div>
          {#if (detail.used_by ?? []).length > 0}
            <ul class="space-y-1">
              {#each detail.used_by ?? [] as u (u)}
                <li class="rounded-md bg-foreground/[0.03] px-3 py-1.5 font-mono text-[11.5px] text-muted-foreground">{u}</li>
              {/each}
            </ul>
          {:else}
            <p class="text-[12px] text-muted-foreground">暂无实例引用</p>
          {/if}
        </div>
      </div>
    {/if}
  </Dialog.Content>
</Dialog.Root>

<!-- 创建/编辑 -->
<Dialog.Root bind:open={editorOpen}>
  <Dialog.Content class="max-h-[85vh] sm:max-w-xl sm:overflow-y-auto">
    <Dialog.Header>
      <Dialog.Title>{editingName ? `编辑「${editingName}」` : '新建配置文件'}</Dialog.Title>
      <Dialog.Description>
        配置项与设备均支持多行；格式 <code class="font-mono">key=value</code>，设备行格式 <code class="font-mono">名称: type=nic network=incusbr0</code>
      </Dialog.Description>
    </Dialog.Header>
    <form id="profile-form" class="grid gap-4" onsubmit={submitEditor}>
      {#if !editingName}
        <div class="grid gap-1.5">
          <Label for="pName">名称</Label>
          <Input id="pName" required bind:value={pform.name} placeholder="gpu-node" class="h-9 font-mono text-[13px]" />
        </div>
      {/if}
      <div class="grid gap-1.5">
        <Label for="pDesc">描述</Label>
        <Input id="pDesc" bind:value={pform.description} class="h-9 text-[13px]" placeholder="用途说明" />
      </div>
      <div class="flex items-center gap-1 rounded-lg bg-muted p-1">
        {#each [['configText', '配置项'], ['devicesText', '设备']] as [key, label] (key)}
          <button
            type="button"
            class="h-7 flex-1 rounded-md px-3 text-[12.5px] font-medium transition-colors {pEditorTab === key
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground'}"
            onclick={() => (pEditorTab = key)}
          >
            {label}
          </button>
        {/each}
      </div>
      {#if pEditorTab === 'configText'}
        <textarea
          bind:value={pform.configText}
          rows={5}
          spellcheck="false"
          class="w-full resize-y rounded-lg border border-input bg-transparent px-3 py-2 font-mono text-[12.5px] shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          placeholder="limits.cpu=4"
        ></textarea>
      {:else}
        <textarea
          bind:value={pform.devicesText}
          rows={5}
          spellcheck="false"
          class="w-full resize-y rounded-lg border border-input bg-transparent px-3 py-2 font-mono text-[12.5px] shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          placeholder="eth0: type=nic network=incusbr0&#10;root: type=disk path=/ pool=local"
        ></textarea>
      {/if}
    </form>
    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" type="button" disabled={submitting} onclick={() => (editorOpen = false)}>取消</Button>
      <Button type="submit" form="profile-form" disabled={submitting}>
        {#if submitting}<span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>{/if}
        {editingName ? '保存' : '创建'}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

{#if delTarget}
  <ConfirmDialog
    bind:open={delOpen}
    title="删除配置文件「{delTarget.name}」"
    description="已创建的实例不受影响，但引用该配置的模板设置将丢失。"
    busy={delBusy}
    onconfirm={doDelete}
  />
{/if}
