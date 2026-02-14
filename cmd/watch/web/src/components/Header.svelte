<script lang="ts">
  import SourceSelector from './SourceSelector.svelte';
  import Badge from '../lib/components/ui/badge.svelte';

  interface Props {
    pageTitle: string;
    connected: boolean;
  }

  let { pageTitle, connected }: Props = $props();

  const statusText = $derived(
    connected ? 'Connected' : 'Reconnecting...'
  );
</script>

<div class="px-4 py-2.5 bg-card border-b border-border flex items-center gap-3 text-sm">
  <h1 class="font-semibold">{pageTitle}</h1>
  <Badge variant={connected ? 'success' : 'destructive'}>
    <span class="w-1.5 h-1.5 rounded-full bg-current mr-1.5"></span>
    {statusText}
  </Badge>
  <span class="flex-1"></span>
  <label for="snapshot-source" class="text-xs text-muted-foreground">View:</label>
  <SourceSelector />
</div>
