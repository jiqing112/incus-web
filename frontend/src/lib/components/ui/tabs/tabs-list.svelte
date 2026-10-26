<script lang="ts">
  let {
    class: className = '',
    children,
  }: {
    class?: string;
    children?: import('svelte').Snippet;
  } = $props();
</script>

<div
  data-slot="tabs-list"
  role="tablist"
  data-variant="default"
  class="rounded-lg p-[3px] group-data-horizontal/tabs:h-8 group/tabs-list inline-flex w-fit items-center justify-center text-muted-foreground group-data-vertical/tabs:h-fit group-data-vertical/tabs:flex-col bg-muted {className}"
>
  {@render children?.()}
</div>
