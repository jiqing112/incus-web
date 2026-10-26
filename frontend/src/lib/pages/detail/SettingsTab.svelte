<script lang="ts">
  /**
   * 实例设置：常用配置表单 + 网络/设备编辑 + 原始配置键值编辑。
   * 仅提交改动项；含设备变更时后端走全量 PUT（Incus PATCH 无法删除设备）。
   */
  import { onMount } from 'svelte';
  import { Save, RotateCcw, Plus, Trash2, AlertTriangle, Network, HardDrive, SlidersHorizontal } from '@lucide/svelte';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Label } from '$lib/components/ui/label';
  import { Switch } from '$lib/components/ui/switch';
  import { Badge } from '$lib/components/ui/badge';
  import { api, pollOperation } from '$lib/api';
  import type { InstanceInfo, NetworkInfo, UpdateInstanceConfigRequest } from '$lib/types';
  import { toast } from 'svelte-sonner';

  let {
    name,
    instance,
    onreload,
  }: { name: string; instance: InstanceInfo; onreload: () => void | Promise<void> } = $props();

  interface Row { k: string; v: string }
  interface DeviceDraft { name: string; type: string; rows: Row[] }

  const SYSTEM_PREFIX = ['volatile.'];
  const KNOWN_KEYS = new Set([
    'description',
    'limits.cpu',
    'limits.cpu.allowance',
    'limits.memory',
    'limits.memory.swap',
    'boot.autostart',
    'security.nesting',
    'security.privileged',
  ]);
  const DEVICE_TYPES = ['disk', 'nic', 'none', 'proxy', 'unix-char', 'unix-block', 'gpu', 'pci', 'usb', 'tpm', 'infiniband'];

  const selectCls =
    'h-8 w-full rounded-lg border border-input bg-transparent px-2 text-[12.5px] outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]';

  let cfgRows = $state<Row[]>([]);
  let devList = $state<DeviceDraft[]>([]);
  let baselineCfg = $state('');
  let baselineDevices = $state('');
  let networks = $state<string[]>([]);
  let saving = $state(false);

  function normalize(o: unknown): unknown {
    if (o && typeof o === 'object' && !Array.isArray(o)) {
      return Object.keys(o as Record<string, unknown>)
        .sort()
        .reduce<Record<string, unknown>>((acc, k) => {
          acc[k] = normalize((o as Record<string, unknown>)[k]);
          return acc;
        }, {});
    }
    return o;
  }
  const stable = (o: unknown) => JSON.stringify(normalize(o));

  function objFromRows(rows: Row[]): Record<string, string> {
    const o: Record<string, string> = {};
    for (const r of rows) {
      const k = r.k.trim();
      if (k) o[k] = r.v;
    }
    return o;
  }

  function devFromList(list: DeviceDraft[]): Record<string, Record<string, string>> {
    const o: Record<string, Record<string, string>> = {};
    for (const d of list) {
      const dn = d.name.trim();
      if (!dn) continue;
      const props: Record<string, string> = {};
      if (d.type) props.type = d.type;
      for (const r of d.rows) {
        const k = r.k.trim();
        if (k && k !== 'type') props[k] = r.v;
      }
      o[dn] = props;
    }
    return o;
  }

  let descDraft = $state('');
  let baselineDesc = $state('');

  let lastInst: InstanceInfo | null = null;

  function hydrate(inst: InstanceInfo) {
    descDraft = inst.description ?? '';
    baselineDesc = descDraft;
    const rows = Object.entries(inst.config ?? {})
      .filter(([k]) => k !== 'description' && !SYSTEM_PREFIX.some((p) => k.startsWith(p)))
      .map(([k, v]) => ({ k, v }));
    // 注意：effect 内只写状态、不回读（baseline 基于局部 rows 计算），避免依赖自触发死循环
    cfgRows = rows;
    baselineCfg = stable(objFromRows(rows));

    devList = Object.entries(inst.devices ?? {}).map(([dn, props]) => {
      const rows: Row[] = [];
      let type = '';
      for (const [k, v] of Object.entries(props)) {
        if (k === 'type') type = v;
        else rows.push({ k, v });
      }
      return { name: dn, type, rows };
    });
    baselineDevices = stable(inst.devices ?? {});
  }

  $effect(() => {
    if (instance !== lastInst) {
      lastInst = instance;
      hydrate(instance);
    }
  });

  onMount(async () => {
    try {
      const nets = await api.networks();
      networks = nets.map((n: NetworkInfo) => n.name);
    } catch {
      /* 忽略：网络列表不可用时退化为手动输入 */
    }
  });

  const dirty = $derived(
    descDraft !== baselineDesc ||
    stable(objFromRows(cfgRows)) !== baselineCfg ||
    stable(devFromList(devList)) !== baselineDevices,
  );
  const advancedRows = $derived(cfgRows.filter((r) => !KNOWN_KEYS.has(r.k)));

  function getVal(key: string): string {
    return cfgRows.find((r) => r.k === key)?.v ?? '';
  }
  function setVal(key: string, v: string) {
    const row = cfgRows.find((r) => r.k === key);
    if (row) row.v = v;
    else if (v !== '') cfgRows.push({ k: key, v });
  }
  const isOn = (key: string) => getVal(key) === 'true';
  function setOn(key: string, on: boolean) {
    const row = cfgRows.find((r) => r.k === key);
    if (on) {
      if (row) row.v = 'true';
      else cfgRows.push({ k: key, v: 'true' });
    } else if (row && row.k) {
      cfgRows = cfgRows.filter((r) => r !== row);
    }
  }

  function addAdvanced() {
    cfgRows.push({ k: '', v: '' });
  }
  function removeRow(row: Row) {
    cfgRows = cfgRows.filter((r) => r !== row);
  }

  function addDevice() {
    let i = 1;
    while (devList.some((d) => d.name === `new${i}`)) i++;
    devList.push({ name: `new${i}`, type: 'disk', rows: [] });
  }
  function removeDevice(d: DeviceDraft) {
    devList = devList.filter((x) => x !== d);
  }
  function addDevProp(d: DeviceDraft) {
    d.rows.push({ k: '', v: '' });
  }
  function removeDevProp(d: DeviceDraft, row: Row) {
    d.rows = d.rows.filter((r) => r !== row);
  }
  const nicParentKey = (d: DeviceDraft) => (d.rows.some((r) => r.k === 'network') ? 'network' : d.rows.some((r) => r.k === 'parent') ? 'parent' : 'network');

  function diffConfig(): Record<string, string> {
    const cur = objFromRows(cfgRows);
    const base = JSON.parse(baselineCfg) as Record<string, unknown>;
    const flatBase: Record<string, string> = {};
    for (const [k, v] of Object.entries(base)) flatBase[k] = String(v);
    const out: Record<string, string> = {};
    for (const k of new Set([...Object.keys(cur), ...Object.keys(flatBase)])) {
      if ((cur[k] ?? '') !== (flatBase[k] ?? '')) out[k] = cur[k] ?? '';
    }
    return out;
  }

  async function save() {
    const changedConfig = diffConfig();
    const devicesChanged = stable(devFromList(devList)) !== baselineDevices;
    const descChanged = descDraft !== baselineDesc;
    if (Object.keys(changedConfig).length === 0 && !devicesChanged && !descChanged) {
      toast.info('没有需要保存的改动');
      return;
    }
    saving = true;
    try {
      const body: UpdateInstanceConfigRequest = { config: changedConfig };
      if (devicesChanged) body.devices = devFromList(devList);
      if (descChanged) body.description = descDraft;
      const op = await api.updateInstanceConfig(name, body);
      await pollOperation(op?.operation_id);
      toast.success('配置已保存', {
        description: devicesChanged ? '设备变更通常需要重启实例后生效' : undefined,
      });
      await onreload();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      saving = false;
    }
  }

  function reset() {
    hydrate(instance);
  }
</script>

<div class="space-y-5">
  <!-- 基础与资源 -->
  <Card class="p-5 shadow-soft">
    <div class="mb-4 flex items-center gap-2">
      <SlidersHorizontal size={15} class="text-primary" />
      <h3 class="text-sm font-semibold tracking-tight">基础与资源</h3>
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <div class="grid gap-1.5 sm:col-span-2">
        <Label>描述</Label>
        <Input bind:value={descDraft} class="h-9 text-[13px]" placeholder="可选，用于说明该实例" />
      </div>
      <div class="grid gap-1.5">
        <Label>CPU 限制</Label>
        <Input value={getVal('limits.cpu')} oninput={(e) => setVal('limits.cpu', e.currentTarget.value)} class="h-9 font-mono text-[13px]" placeholder="如 2 或 0-3（留空=不限）" />
      </div>
      <div class="grid gap-1.5">
        <Label>CPU allowance</Label>
        <Input value={getVal('limits.cpu.allowance')} oninput={(e) => setVal('limits.cpu.allowance', e.currentTarget.value)} class="h-9 font-mono text-[13px]" placeholder="如 50% 或 25ms/100ms" />
      </div>
      <div class="grid gap-1.5">
        <Label>内存限制</Label>
        <Input value={getVal('limits.memory')} oninput={(e) => setVal('limits.memory', e.currentTarget.value)} class="h-9 font-mono text-[13px]" placeholder="如 1GiB（留空=不限）" />
      </div>
      <div class="grid gap-1.5">
        <Label>内存 swap</Label>
        <Input value={getVal('limits.memory.swap')} oninput={(e) => setVal('limits.memory.swap', e.currentTarget.value)} class="h-9 font-mono text-[13px]" placeholder="true / false 或大小" />
      </div>
    </div>

    <div class="mt-5 space-y-3 border-t border-border/60 pt-4">
      <div class="flex items-center justify-between">
        <div>
          <div class="text-[13px] font-medium">开机自启</div>
          <div class="text-[11.5px] text-muted-foreground">boot.autostart · 宿主机启动时自动运行</div>
        </div>
        <Switch checked={isOn('boot.autostart')} onCheckedChange={(v: boolean) => setOn('boot.autostart', v)} />
      </div>
      <div class="flex items-center justify-between">
        <div>
          <div class="text-[13px] font-medium">允许嵌套</div>
          <div class="text-[11.5px] text-muted-foreground">security.nesting · 容器内可运行容器/Docker</div>
        </div>
        <Switch checked={isOn('security.nesting')} onCheckedChange={(v: boolean) => setOn('security.nesting', v)} />
      </div>
      <div class="flex items-center justify-between">
        <div>
          <div class="text-[13px] font-medium">特权容器</div>
          <div class="text-[11.5px] text-muted-foreground">security.privileged · 降低隔离，修改需重启</div>
        </div>
        <Switch checked={isOn('security.privileged')} onCheckedChange={(v: boolean) => setOn('security.privileged', v)} />
      </div>
    </div>
  </Card>

  <!-- 设备（含网络） -->
  <Card class="p-5 shadow-soft">
    <div class="mb-4 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <HardDrive size={15} class="text-primary" />
        <h3 class="text-sm font-semibold tracking-tight">设备</h3>
        <span class="text-[11.5px] text-muted-foreground">磁盘 / 网卡 / 挂载等</span>
      </div>
      <Button size="sm" variant="outline" class="h-8 gap-1" onclick={addDevice}>
        <Plus size={13} /> 添加设备
      </Button>
    </div>

    {#if devList.length === 0}
      <p class="text-[12.5px] text-muted-foreground">没有自定义设备。</p>
    {:else}
      <div class="space-y-3">
        {#each devList as d, di (di)}
          <div class="rounded-xl border border-border/60 p-3">
            <div class="mb-2 flex items-center gap-2">
              <Input bind:value={d.name} class="h-8 w-40 font-mono text-[12.5px]" aria-label="设备名" />
              <select bind:value={d.type} class={selectCls + ' w-36'} aria-label="设备类型">
                {#each DEVICE_TYPES as t (t)}<option value={t}>{t}</option>{/each}
              </select>
              {#if d.type === 'nic'}<Badge variant="secondary" class="gap-1 text-[10.5px]"><Network size={11} />{d.name}</Badge>{/if}
              <Button size="icon" variant="ghost" class="ml-auto size-8 text-muted-foreground hover:text-destructive" onclick={() => removeDevice(d)} aria-label="删除设备">
                <Trash2 size={14} />
              </Button>
            </div>

            <div class="space-y-2">
              {#each d.rows as row (row)}
                <div class="flex items-center gap-2">
                  <Input bind:value={row.k} class="h-8 w-40 font-mono text-[12px]" placeholder="键" aria-label="属性名" />
                  {#if d.type === 'nic' && (row.k === 'network' || row.k === 'parent')}
                    <select bind:value={row.v} class={selectCls + ' flex-1'} aria-label="网络">
                      <option value="">（选择网络）</option>
                      {#if row.v && !networks.includes(row.v)}<option value={row.v}>{row.v}</option>{/if}
                      {#each networks as n (n)}<option value={n}>{n}</option>{/each}
                    </select>
                  {:else}
                    <Input bind:value={row.v} class="h-8 flex-1 font-mono text-[12px]" placeholder="值" aria-label="属性值" />
                  {/if}
                  <Button size="icon" variant="ghost" class="size-8 text-muted-foreground hover:text-destructive" onclick={() => removeDevProp(d, row)} aria-label="删除属性">
                    <Trash2 size={13} />
                  </Button>
                </div>
              {/each}
              <Button size="sm" variant="ghost" class="h-7 gap-1 text-[12px] text-muted-foreground" onclick={() => addDevProp(d)}>
                <Plus size={12} /> 添加属性
              </Button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </Card>

  <!-- 高级配置 -->
  <Card class="p-5 shadow-soft">
    <div class="mb-4 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <SlidersHorizontal size={15} class="text-primary" />
        <h3 class="text-sm font-semibold tracking-tight">高级配置</h3>
        <span class="text-[11.5px] text-muted-foreground">任意 config 键值（volatile.* 已隐藏）</span>
      </div>
      <Button size="sm" variant="outline" class="h-8 gap-1" onclick={addAdvanced}>
        <Plus size={13} /> 添加
      </Button>
    </div>
    <div class="space-y-2">
      {#each advancedRows as row (row)}
        <div class="flex items-center gap-2">
          <Input bind:value={row.k} class="h-8 w-64 font-mono text-[12px]" placeholder="键，如 user.foo" aria-label="配置键" />
          <Input bind:value={row.v} class="h-8 flex-1 font-mono text-[12px]" placeholder="值" aria-label="配置值" />
          <Button size="icon" variant="ghost" class="size-8 text-muted-foreground hover:text-destructive" onclick={() => removeRow(row)} aria-label="删除配置项">
            <Trash2 size={13} />
          </Button>
        </div>
      {/each}
      {#if advancedRows.length === 0}
        <p class="text-[12.5px] text-muted-foreground">没有额外的配置项。</p>
      {/if}
    </div>
  </Card>

  <!-- 保存栏 -->
  <div class="sticky bottom-0 -mx-5 flex items-center gap-2 border-t border-border/60 bg-popover/95 px-5 py-3 backdrop-blur">
    <span class="flex items-center gap-1.5 text-[11.5px] text-muted-foreground">
      <AlertTriangle size={13} class="text-warning" /> 资源限制热生效；安全项与设备变更需重启实例
    </span>
    <div class="ml-auto flex items-center gap-2">
      <Button size="sm" variant="outline" class="h-8.5 gap-1.5" disabled={!dirty || saving} onclick={reset}>
        <RotateCcw size={13} /> 重置
      </Button>
      <Button size="sm" class="h-8.5 gap-1.5" disabled={!dirty || saving} onclick={save}>
        {#if saving}
          <span class="inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
          保存中…
        {:else}
          <Save size={13} /> 保存
        {/if}
      </Button>
    </div>
  </div>
</div>
