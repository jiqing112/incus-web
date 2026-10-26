<script lang="ts">
  import Sidebar from './Sidebar.svelte';
  import Topbar from './Topbar.svelte';
  import { layout } from './state.svelte';

  let {
    title,
    subtitle,
    onrefresh,
    children,
  }: {
    title: string;
    subtitle?: string;
    onrefresh?: () => void;
    children: import('svelte').Snippet;
  } = $props();
</script>

<div class="min-h-svh">
  <Sidebar bind:open={layout.mobileMenuOpen} />
  <div class="lg:pl-[236px]">
    <Topbar {title} {subtitle} {onrefresh} />
    <main class="mx-auto max-w-[1440px] px-5 py-6 lg:px-8">
      {@render children()}
    </main>
  </div>
</div>
