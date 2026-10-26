<script lang="ts">
  /** 通用危险/普通确认弹窗：标题、描述、确认回调。 */
  import { TriangleAlert } from '@lucide/svelte';
  import * as Dialog from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';

  let {
    open = $bindable(false),
    title,
    description,
    confirmLabel = '确认删除',
    danger = true,
    busy = false,
    onconfirm,
  }: {
    open?: boolean;
    title: string;
    description?: string;
    confirmLabel?: string;
    danger?: boolean;
    busy?: boolean;
    onconfirm: () => void | Promise<void>;
  } = $props();
</script>

<Dialog.Root {open} onOpenChange={(v) => (open = v)}>
  <Dialog.Content class="sm:max-w-sm">
    <Dialog.Header>
      <Dialog.Title class="flex items-center gap-2 {danger ? 'text-destructive' : ''}">
        {#if danger}
          <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-destructive/10">
            <TriangleAlert size={17} class="text-destructive" />
          </span>
        {/if}
        {title}
      </Dialog.Title>
      {#if description}
        <Dialog.Description class="text-left">{description}</Dialog.Description>
      {/if}
    </Dialog.Header>
    <Dialog.Footer class="gap-2 sm:justify-end">
      <Button variant="outline" disabled={busy} onclick={() => (open = false)}>取消</Button>
      <Button variant={danger ? 'destructive' : 'default'} disabled={busy} onclick={onconfirm}>
        {#if busy}
          <span class="mr-1 inline-block size-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
        {/if}
        {confirmLabel}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
