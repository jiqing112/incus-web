<script lang="ts">
  let {
    icon,
    title,
    desc,
    action,
    compact = false,
  }: {
    icon?: import('svelte').Snippet;
    title: string;
    desc?: string;
    action?: import('svelte').Snippet;
    compact?: boolean;
  } = $props();
</script>

<div
  class="flex flex-col items-center justify-center rounded-xl border border-dashed border-border/80 bg-card/50 text-center {compact ? 'gap-2 px-6 py-8' : 'gap-3 px-6 py-14'}"
>
  {#if icon}
    <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-foreground/[0.04] text-muted-foreground">
      {@render icon()}
    </div>
  {/if}
  <div class={compact ? 'text-sm font-medium' : 'text-[15px] font-medium'}>{title}</div>
  {#if desc}
    <p class="max-w-sm text-[12.5px] leading-relaxed text-muted-foreground">{desc}</p>
  {/if}
  {#if action}
    <div class="mt-1.5">{@render action()}</div>
  {/if}
</div>
