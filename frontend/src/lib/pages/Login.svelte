<script lang="ts">
  import { Boxes, CircleAlert, KeyRound } from '@lucide/svelte';
  import { fade, fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Label } from '$lib/components/ui/label';
  import { loginWithToken, session } from '$lib/stores/session.svelte';
  import { navigate } from '$lib/router.svelte';

  let token = $state('');
  let error = $state('');
  let loading = $state(false);

  $effect(() => {
    if (session.authed) navigate('/dashboard', true);
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (loading || !token.trim()) return;
    error = '';
    loading = true;
    try {
      await loginWithToken(token);
    } catch (err) {
      error = err instanceof Error ? err.message : '认证失败，请检查 Token';
    } finally {
      loading = false;
    }
  }
</script>

<div class="relative flex min-h-svh items-center justify-center overflow-hidden px-4">
  <!-- 背景装饰：柔和渐变 + 点阵 -->
  <div
    class="pointer-events-none absolute inset-0"
    style="background:
      radial-gradient(720px 420px at 15% 8%, oklch(0.92 0.05 240 / 0.7), transparent 62%),
      radial-gradient(640px 480px at 88% 92%, oklch(0.93 0.04 210 / 0.55), transparent 60%);"
  ></div>
  <div
    class="pointer-events-none absolute inset-0 opacity-[0.35]"
    style="background-image: radial-gradient(circle, var(--border) 1px, transparent 1px); background-size: 22px 22px;"
  ></div>

  <div class="relative w-full max-w-[380px]" in:fly={{ y: 14, duration: 350, easing: cubicOut }}>
    <div class="mb-7 flex flex-col items-center gap-3 text-center">
      <Boxes size={46} strokeWidth={1.6} class="text-primary" />
      <div>
        <h1 class="text-[19px] font-semibold tracking-tight">IncusPanel</h1>
        <p class="mt-1 text-[12.5px] text-muted-foreground">容器与虚拟机可视化管理面板</p>
      </div>
    </div>

    <div class="rounded-2xl border border-border/70 bg-card p-6 shadow-lift">
      <h2 class="mb-5 text-[15px] font-semibold">连接到 Incus 服务器</h2>

      <form class="space-y-4" onsubmit={submit}>
        <div class="grid gap-2">
          <Label for="token" class="flex items-center gap-1.5">
            <KeyRound size={13} class="text-muted-foreground" />
            API Token
          </Label>
          <Input
            id="token"
            type="password"
            autocomplete="current-password"
            bind:value={token}
            class="h-10 font-mono"
            placeholder="请输入 API Token"
          />
          <p class="text-[11px] leading-relaxed text-muted-foreground">
            Token 由服务端 <code class="rounded bg-foreground/[0.05] px-1 py-0.5 font-mono text-[10.5px]">API_TOKEN</code> 环境变量配置。
            未设置时会随机生成并打印在启动日志中（仅本次运行有效），请从服务端日志获取。
          </p>
        </div>

        {#if error}
          <div
            class="flex items-center gap-1.5 rounded-lg bg-destructive/8 px-3 py-2 text-[12.5px] text-destructive"
            transition:fade={{ duration: 120 }}
          >
            <CircleAlert size={14} />
            {error}
          </div>
        {/if}

        <Button type="submit" class="h-10 w-full text-[13.5px]" disabled={loading || !token.trim()}>
          {#if loading}
            <span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
            连接中…
          {:else}
            连接服务器
          {/if}
        </Button>
      </form>
    </div>

    <p class="mt-5 text-center text-[11px] text-muted-foreground/70">
      IncusPanel · 只调用你自己的 Incus API，数据不出本机
    </p>
  </div>
</div>
