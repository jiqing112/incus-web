<script lang="ts">
  /** 横向排行条：按数值占比着色，宽度动画入场。 */
  import { fmtNum } from '$lib/utils';

  export interface RankItem {
    label: string;
    sub?: string;
    value: number;
    flagged?: boolean;
  }

  let { items }: { items: RankItem[] } = $props();

  const max = $derived(Math.max(...items.map((i) => i.value), 1));

  // 首位强调主色，其余按排名递减透明度
  function barStyle(idx: number): string {
    if (items[idx].flagged) return 'background: var(--destructive)';
    const alpha = idx === 0 ? 1 : Math.max(0.28, 1 - idx * 0.16);
    return `background: color-mix(in oklch, var(--chart-1) ${Math.round(alpha * 100)}%, transparent)`;
  }
</script>

<ul class="space-y-3">
  {#each items as item, i (item.label)}
    <li class="group">
      <div class="mb-1.5 flex items-baseline justify-between gap-3">
        <div class="flex min-w-0 items-center gap-2">
          <span class="font-num w-4 shrink-0 text-right text-[10.5px] text-muted-foreground/70">{i + 1}</span>
          <span class="truncate text-[12.5px] font-medium {item.flagged ? 'text-destructive' : ''}">{item.label}</span>
          {#if item.sub}
            <span class="shrink-0 text-[10.5px] text-muted-foreground/80">{item.sub}</span>
          {/if}
        </div>
        <span class="font-num shrink-0 text-[12px] font-medium {item.flagged ? 'text-destructive' : 'text-foreground/85'}">
          {fmtNum(item.value)}
        </span>
      </div>
      <div class="ml-6 h-[6px] overflow-hidden rounded-full bg-foreground/[0.05]">
        <div
          class="h-full rounded-full transition-[width] duration-700 ease-out"
          style="width:{(item.value / max) * 100}%; {barStyle(i)}"
        ></div>
      </div>
    </li>
  {/each}
</ul>
