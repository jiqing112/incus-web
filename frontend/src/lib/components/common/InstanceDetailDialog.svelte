<script lang="ts">
  /** 实例详情模态框：复用 InstanceDetail 内容，居中浮层 + 内部滚动。 */
  import * as Dialog from '$lib/components/ui/dialog';
  import InstanceDetail from '$lib/pages/InstanceDetail.svelte';

  let {
    open = $bindable(false),
    name = '',
    initialTab = 'overview',
  }: { open?: boolean; name?: string; initialTab?: string } = $props();
</script>

<Dialog.Root bind:open>
  <Dialog.Content
    class="flex h-[86vh] w-[94vw] max-w-[1180px] flex-col gap-0 overflow-hidden p-5 sm:max-w-[1180px]"
    showCloseButton={false}
  >
    <Dialog.Title class="sr-only">{name || '实例'} 详情</Dialog.Title>
    <Dialog.Description class="sr-only">实例概览、快照、备份、文件与终端</Dialog.Description>
    {#if name}
      {#key name + ':' + initialTab}
        <InstanceDetail {name} {initialTab} syncUrl={false} onclose={() => (open = false)} />
      {/key}
    {/if}
  </Dialog.Content>
</Dialog.Root>
