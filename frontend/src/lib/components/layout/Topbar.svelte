<script lang="ts">
  import { Moon, Sun, Menu, RefreshCw, LogOut } from '@lucide/svelte';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
  import { toggleTheme, theme } from '$lib/theme.svelte';
  import { layout } from './state.svelte';
  import { session, logout } from '$lib/stores/session.svelte';

  let { title, subtitle, onrefresh }: { title: string; subtitle?: string; onrefresh?: () => void } = $props();

  let refreshing = $state(false);
  function refresh() {
    onrefresh?.();
    refreshing = true;
    setTimeout(() => (refreshing = false), 600);
  }
</script>

<header class="sticky top-0 z-20 border-b border-border/70 bg-background/95 backdrop-blur-sm">
  <div class="flex h-14 items-center gap-3 px-5 lg:px-8">
    <Button variant="ghost" size="icon" class="lg:hidden" onclick={() => (layout.mobileMenuOpen = true)} aria-label="菜单">
      <Menu size={18} />
    </Button>

    <div class="min-w-0 flex-1">
      <h1 class="truncate text-[15px] font-semibold tracking-tight">{title}</h1>
      {#if subtitle}
        <p class="truncate text-xs text-muted-foreground">{subtitle}</p>
      {/if}
    </div>

    <div class="flex items-center gap-1.5">
      {#if session.authed}
        <Badge variant="secondary" class="hidden font-mono text-[11px] sm:inline-flex">
          {session.serverName} {session.serverVersion}
        </Badge>
      {/if}
      <Button variant="ghost" size="icon" class="text-muted-foreground" onclick={refresh} aria-label="刷新">
        <RefreshCw size={16} class={refreshing ? 'animate-spin' : ''} />
      </Button>
      <Button variant="ghost" size="icon" class="text-muted-foreground" onclick={toggleTheme} aria-label="切换主题">
        {#if theme.resolved === 'dark'}
          <Sun size={16} />
        {:else}
          <Moon size={16} />
        {/if}
      </Button>
      <DropdownMenu.Root>
        <DropdownMenu.Trigger
          class="ml-1 flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-sky-400 to-blue-600 text-xs font-semibold text-white shadow-soft select-none outline-none focus-visible:ring-2 focus-visible:ring-ring"
          title="admin"
        >
          A
        </DropdownMenu.Trigger>
        <DropdownMenu.Content align="end" class="w-40">
          <DropdownMenu.Label class="text-xs text-muted-foreground">Token 认证用户</DropdownMenu.Label>
          <DropdownMenu.Separator />
          <DropdownMenu.Item class="text-destructive focus:text-destructive" onclick={logout}>
            <LogOut size={14} class="mr-1.5" />
            退出登录
          </DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Root>
    </div>
  </div>
</header>
