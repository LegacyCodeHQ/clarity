<script lang="ts">
  import SourceSelector from './SourceSelector.svelte';
  import ConnectionIndicator from './ConnectionIndicator.svelte';

  interface Props {
    pageTitle: string;
    connected: boolean;
  }

  let { pageTitle, connected }: Props = $props();
</script>

<div class="px-4 py-2.5 bg-card border-b border-border flex items-center gap-4">
  <h1 class="text-sm font-semibold text-foreground">{pageTitle}</h1>
  <span class="flex-1"></span>
  <SourceSelector />
  <ConnectionIndicator {connected} />
</div>
