<script lang="ts">
  /** 实例类型 / 驱动等中性徽标。 */
  import { Badge } from '$lib/components/ui/badge';

  let { label, tone = 'default' }: { label: string; tone?: 'default' | 'info' | 'success' | 'warning' } = $props();
</script>

<Badge variant="outline" class="gap-1 px-1.5 font-mono text-[10.5px] font-medium {tone === 'info' ? 'bg-info/8 text-info ring-info/20' : tone === 'success' ? 'bg-success/8 text-success ring-success/20' : tone === 'warning' ? 'bg-warning/10 text-warning ring-warning/25' : 'bg-foreground/[0.04] text-muted-foreground ring-border'}">
  {label}
</Badge>
