<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Stethoscope, CheckCircle2, AlertTriangle, XCircle, MinusCircle, Wrench, RefreshCw,
    ShieldCheck, Wifi, WifiOff, Loader2, CircleAlert,
  } from '@lucide/svelte';
  import { fade } from 'svelte/transition';
  import { Card } from '$lib/components/ui/card';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import { ConfirmDialog } from '$lib/components/common';
  import { api } from '$lib/api';
  import type { NetCheck, NetworkDiagResult } from '$lib/types';
  import { toast } from 'svelte-sonner';

  let result = $state<NetworkDiagResult | null>(null);
  let loading = $state(true);
  let repairing = $state('');
  let confirmFix = $state<NetCheck | null>(null);
  let confirmOpen = $state(false);

  async function run() {
    loading = true;
    try {
      result = await api.networkDiagnose();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      loading = false;
    }
  }

  onMount(run);

  async function repair(ck: NetCheck) {
    if (!ck.fix_id) return;
    repairing = ck.fix_id;
    try {
      const r = await api.networkRepair(ck.fix_id);
      toast.success(r.message || '修复完成');
      confirmFix = null;
      confirmOpen = false;
      await run();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      repairing = '';
    }
  }

  const statusMeta: Record<string, { icon: typeof CheckCircle2; cls: string; label: string }> = {
    ok: { icon: CheckCircle2, cls: 'text-success', label: '正常' },
    warn: { icon: AlertTriangle, cls: 'text-warning', label: '警告' },
    fail: { icon: XCircle, cls: 'text-destructive', label: '异常' },
    skip: { icon: MinusCircle, cls: 'text-muted-foreground', label: '跳过' },
  };

  const problems = $derived(result?.checks.filter((c) => c.status === 'fail' || c.status === 'warn') ?? []);
  const fixables = $derived(problems.filter((c) => c.fixable));
  const okItems = $derived(result?.checks.filter((c) => c.status === 'ok' || c.status === 'skip') ?? []);
</script>

<div class="animate-fade-up space-y-4">
  <!-- 顶部操作条 -->
  <div class="flex flex-wrap items-center gap-3">
    <div class="min-w-0 flex-1">
      <p class="text-[13px] text-muted-foreground">
        检测宿主机转发链、Docker 转发链冲突、Incus 网桥放行、MASQUERADE 与容器连通性，
        发现异常可一键修复。
      </p>
    </div>
    <Button size="sm" class="h-9 gap-1.5" disabled={loading} onclick={run}>
      <RefreshCw size={14} class={loading ? 'animate-spin' : ''} />
      {loading ? '诊断中…' : '重新诊断'}
    </Button>
  </div>

  {#if loading && !result}
    <div class="space-y-3">
      {#each Array(4) as _, i (i)}
        <div class="h-16 animate-pulse rounded-xl bg-foreground/[0.04]"></div>
      {/each}
    </div>
  {:else if result}
    <!-- 汇总横幅 -->
    <Card class="p-5 shadow-soft">
      <div class="flex flex-wrap items-center gap-4">
        <span
          class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl {result.fail > 0
            ? 'bg-destructive/10 text-destructive'
            : result.warn > 0
              ? 'bg-warning/15 text-warning'
              : 'bg-success/10 text-success'}"
        >
          {#if result.fail > 0}
            <CircleAlert size={22} />
          {:else if result.warn > 0}
            <AlertTriangle size={22} />
          {:else}
            <ShieldCheck size={22} />
          {/if}
        </span>
        <div class="min-w-0 flex-1">
          <div class="text-[15px] font-semibold tracking-tight">
            {#if result.fail > 0}
              发现 {result.fail} 项异常{result.warn > 0 ? `、${result.warn} 项警告` : ''}
            {:else if result.warn > 0}
              发现 {result.warn} 项警告
            {:else}
              网络状态良好
            {/if}
          </div>
          <div class="font-num mt-0.5 text-[11.5px] text-muted-foreground">
            共 {result.checks.length} 项检查 · {result.checked_at}
          </div>
        </div>
        {#if fixables.length > 0}
          <Button
            size="sm"
            class="h-9 gap-1.5"
            disabled={repairing !== ''}
            onclick={async () => {
              let failed = 0;
              for (const ck of fixables) {
                repairing = ck.fix_id!;
                try {
                  await api.networkRepair(ck.fix_id!);
                } catch (e) {
                  failed++;
                  toast.error(`${ck.title}: ${e instanceof Error ? e.message : String(e)}`);
                }
              }
              repairing = '';
              const ok = fixables.length - failed;
              if (failed === 0) toast.success('全部可修复项已处理');
              else if (ok === 0) toast.error('所有修复项均执行失败，请查看错误信息');
              else toast.warning(`已修复 ${ok} 项，${failed} 项失败`);
              await run();
            }}
          >
            {#if repairing}
              <Loader2 size={14} class="animate-spin" />
            {:else}
              <Wrench size={14} />
            {/if}
            一键修复全部（{fixables.length}）
          </Button>
        {/if}
      </div>
    </Card>

    <!-- 异常项：醒目大卡（含修复按钮与详细说明） -->
    {#if problems.length > 0}
      <div class="space-y-2" in:fade={{ duration: 150 }}>
        {#each problems as ck (ck.id + ck.title)}
          {@const meta = statusMeta[ck.status] ?? statusMeta.skip}
          <Card class="p-4 shadow-soft {ck.status === 'fail' ? 'border-destructive/30' : 'border-warning/40'}">
            <div class="flex items-start gap-3">
              <span class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-foreground/[0.03]">
                <meta.icon size={16} class={meta.cls} />
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-[13.5px] font-medium">{ck.title}</span>
                  <Badge variant="outline" class="px-1.5 text-[10.5px] {meta.cls} ring-current/20">{meta.label}</Badge>
                </div>
                <p class="mt-1 text-[12.5px] leading-relaxed text-muted-foreground">{ck.detail}</p>
                {#if ck.fixable && ck.fix_desc}
                  <p class="mt-1.5 flex items-start gap-1 text-[11.5px] text-muted-foreground/80">
                    <Wrench size={11} class="mt-0.5 shrink-0" />
                    修复动作：{ck.fix_desc}
                  </p>
                {/if}
              </div>
              {#if ck.fixable}
                <Button
                  size="sm"
                  variant="outline"
                  class="h-8 shrink-0 gap-1.5"
                  disabled={repairing !== ''}
                  onclick={() => { confirmFix = ck; confirmOpen = true; }}
                >
                  {#if repairing === ck.fix_id}
                    <Loader2 size={13} class="animate-spin" />
                  {:else}
                    <Wrench size={13} />
                  {/if}
                  一键修复
                </Button>
              {/if}
            </div>
          </Card>
        {/each}
      </div>
    {/if}

    <!-- 正常 / 跳过项：单卡紧凑清单 -->
    {#if okItems.length > 0}
      <Card class="shadow-soft">
        <div class="flex items-center gap-2 border-b border-border/60 px-4 py-3">
          <h2 class="text-sm font-semibold tracking-tight">通过的检查项</h2>
          <span class="font-num ml-auto text-[11.5px] text-muted-foreground">{okItems.length} 项</span>
        </div>
        <ul class="divide-y divide-border/40">
          {#each okItems as ck (ck.id + ck.title)}
            {@const meta = statusMeta[ck.status] ?? statusMeta.skip}
            <li class="flex items-center gap-2.5 px-4 py-2.5">
              <meta.icon size={14} class="shrink-0 {ck.status === 'skip' ? 'text-muted-foreground/50' : meta.cls}" />
              <span class="shrink-0 text-[12.5px] font-medium {ck.status === 'skip' ? 'text-muted-foreground' : ''}">{ck.title}</span>
              <span class="min-w-0 flex-1 truncate text-right font-mono text-[11.5px] text-muted-foreground/70" title={ck.detail}>{ck.detail}</span>
              <Badge variant="outline" class="shrink-0 px-1.5 text-[10.5px] text-muted-foreground ring-border">{meta.label}</Badge>
            </li>
          {/each}
        </ul>
      </Card>
    {/if}
  {/if}
</div>

{#if confirmFix}
  <ConfirmDialog
    bind:open={confirmOpen}
    title="确认执行修复"
    description={confirmFix.title + '：' + (confirmFix.fix_desc ?? '')}
    confirmLabel="执行修复"
    danger={false}
    busy={repairing !== ''}
    onconfirm={() => repair(confirmFix!)}
  />
{/if}
