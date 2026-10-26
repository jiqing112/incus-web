<script lang="ts">
  /** 键值对展示列表（config / devices）。 */
  import { Copy, Check } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';

  let {
    entries,
    mono = true,
    emptyText = '暂无配置',
  }: {
    entries: [string, string][];
    mono?: boolean;
    emptyText?: string;
  } = $props();

  let copied = $state('');

  async function copy(v: string) {
    try {
      await navigator.clipboard.writeText(v);
      copied = v;
      setTimeout(() => (copied = ''), 1200);
    } catch {
      toast.error('复制失败');
    }
  }
</script>

{#if entries.length === 0}
  <div class="rounded-lg border border-dashed border-border/70 px-4 py-5 text-center text-[12.5px] text-muted-foreground">
    {emptyText}
  </div>
{:else}
  <div class="overflow-hidden rounded-xl border border-border/70">
    {#each entries as [k, v], i (k + i)}
      <div class="group flex items-center gap-3 border-b border-border/40 px-3.5 py-2.5 transition-colors last:border-0 hover:bg-foreground/[0.02]">
        <div class="w-[38%] min-w-0 shrink-0">
          <div class="truncate text-[11.5px] text-muted-foreground">{k}</div>
        </div>
        <div class="min-w-0 flex-1 {mono ? 'font-mono' : ''}">
          <div class="truncate text-[12.5px]">{v}</div>
        </div>
        <button
          class="shrink-0 rounded-md p-1 text-muted-foreground/0 transition-colors group-hover:text-muted-foreground/60 hover:!text-foreground"
          onclick={() => copy(v)}
          aria-label="复制"
        >
          {#if copied === v}<Check size={13} />{:else}<Copy size={13} />{/if}
        </button>
      </div>
    {/each}
  </div>
{/if}
