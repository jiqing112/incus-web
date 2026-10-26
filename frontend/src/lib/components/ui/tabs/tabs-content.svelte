<script lang="ts">
  import { getContext } from 'svelte';

  let {
    value,
    class: className = '',
    children,
  }: {
    value: string;
    class?: string;
    children?: import('svelte').Snippet;
  } = $props();

  const root = getContext<{ value: string; select: (v: string) => void }>('incus-tabs');
  const isActive = $derived(root.value === value);
</script>

{#if isActive}
  <div data-slot="tabs-content" role="tabpanel" class="text-sm flex-1 outline-none {className}">
    {@render children?.()}
  </div>
{/if}
