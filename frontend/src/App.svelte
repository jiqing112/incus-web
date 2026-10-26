<script lang="ts">
  import { initTheme } from '$lib/theme.svelte';
  import { router, navigate, parsePath, initRouter } from '$lib/router.svelte';
  import { session, verify } from '$lib/stores/session.svelte';
  import { navTitleMap } from '$lib/nav';
  import AppShell from '$lib/components/layout/AppShell.svelte';
  import InstanceDetailDialog from '$lib/components/common/InstanceDetailDialog.svelte';
  import { detailModal, openDetail } from '$lib/stores/detailModal.svelte';
  import Dashboard from '$lib/pages/Dashboard.svelte';
  import Instances from '$lib/pages/Instances.svelte';
  import Images from '$lib/pages/Images.svelte';
  import Networks from '$lib/pages/Networks.svelte';
  import Storage from '$lib/pages/Storage.svelte';
  import Profiles from '$lib/pages/Profiles.svelte';
  import Events from '$lib/pages/Events.svelte';
  import NetworkDiagnostics from '$lib/pages/NetworkDiagnostics.svelte';
  import InstallWizard from '$lib/pages/InstallWizard.svelte';
  import Login from '$lib/pages/Login.svelte';
  import { installState, refreshInstallStatus } from '$lib/stores/install.svelte';
  import { Toaster } from '$lib/components/ui/sonner';
  import { TooltipProvider } from '$lib/components/ui/tooltip';
  import { triggerRefresh } from '$lib/components/layout/state.svelte';

  let booted = $state(false);

  $effect(() => {
    initTheme();
    initRouter();
    (async () => {
      await verify();
      booted = true;
    })();
  });

  // 检测宿主机 incus 安装状态，未安装则引导到安装向导
  // （状态在共享 store installState 中，安装向导完成后会同步更新，避免再次误跳）
  $effect(() => {
    if (!booted || !session.authed) return;
    (async () => {
      const ready = await refreshInstallStatus();
      if (!ready && parsePath(router.path).pathname !== '/install') {
        navigate('/install', true);
      }
    })();
  });

  const { pathname, query } = $derived(parsePath(router.path));
  const tabValues = ['overview', 'snapshots', 'backups', 'files', 'terminal', 'settings'];

  // 深链 /instances/:name 以模态框呈现（不再全屏），并把地址规范化为 /instances
  const routeDetail = $derived.by(() => {
    const m = pathname.match(/^\/instances\/([^/]+)$/);
    if (!m) return null;
    const t = query.get('tab') ?? 'overview';
    return { name: decodeURIComponent(m[1]), tab: tabValues.includes(t) ? t : 'overview' };
  });

  const isAuthPage = $derived(pathname === '/login');

  // 路由守卫
  $effect(() => {
    if (!booted) return;
    if (!session.authed && !isAuthPage) {
      navigate('/login', true);
      return;
    }
    if (session.authed && (pathname === '/login' || pathname === '/' || pathname === '')) {
      if (installState.checked && !installState.ready) {
        navigate('/install', true);
      } else {
        navigate('/dashboard', true);
      }
    }
  });

  // 打开详情模态框并清理 URL
  $effect(() => {
    if (!booted || !session.authed) return;
    if (routeDetail) {
      openDetail(routeDetail.name, routeDetail.tab);
      navigate('/instances', true);
    }
  });

  function pageMeta(path: string): { title: string; sub: string } {
    return navTitleMap[path] ?? { title: 'IncusPanel', sub: '' };
  }

  const meta = $derived(pageMeta(pathname));
</script>

<TooltipProvider delayDuration={200}>
  {#if !booted}
    <div class="flex min-h-svh items-center justify-center bg-background">
      <div class="flex flex-col items-center gap-3">
        <span class="inline-block size-7 animate-spin rounded-full border-2 border-primary border-t-transparent"></span>
        <span class="text-xs text-muted-foreground">正在连接 Incus…</span>
      </div>
    </div>
  {:else if !session.authed}
    <Login />
  {:else}
    <AppShell title={meta.title} subtitle={meta.sub} onrefresh={triggerRefresh}>
      <div>
        {#if pathname === '/dashboard'}
          <Dashboard />
        {:else if pathname === '/instances'}
          <Instances />
        {:else if pathname === '/images'}
          <Images />
        {:else if pathname === '/networks'}
          <Networks />
        {:else if pathname === '/storage'}
          <Storage />
        {:else if pathname === '/profiles'}
          <Profiles />
        {:else if pathname === '/events'}
          <Events />
        {:else if pathname === '/network-diagnostics'}
          <NetworkDiagnostics />
        {:else if pathname === '/install'}
          <InstallWizard />
        {:else}
          <div class="flex flex-col items-center gap-2 py-24 text-center">
            <div class="text-[15px] font-medium">页面不存在</div>
            <p class="text-[12.5px] text-muted-foreground">检查地址是否正确，或从左侧导航进入。</p>
          </div>
        {/if}
      </div>
    </AppShell>
  {/if}

  <InstanceDetailDialog bind:open={detailModal.open} name={detailModal.name} initialTab={detailModal.tab} />
</TooltipProvider>

<Toaster position="top-center" richColors closeButton />
