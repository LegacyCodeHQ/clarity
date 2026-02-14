<script lang="ts">
  import { onMount } from 'svelte';
  import { viewModel } from '../lib/stores/graphStore';
  import { initGraphviz, renderDot } from '../lib/graphviz';
  import Card from '../lib/components/ui/card.svelte';
  import Skeleton from '../lib/components/ui/skeleton.svelte';

  let container: HTMLDivElement;
  let graphvizReady = $state(false);
  let renderError = $state<string | null>(null);

  onMount(async () => {
    try {
      await initGraphviz();
      graphvizReady = true;
    } catch (err) {
      console.error('Failed to initialize Graphviz:', err);
      renderError = 'Failed to load Graphviz';
    }
  });

  async function renderGraph(dot: string) {
    if (!graphvizReady || !container) return;

    try {
      const svg = await renderDot(dot);
      container.innerHTML = svg;
      renderError = null;
    } catch (err) {
      console.error('Graphviz render error:', err);
      renderError = 'Render error';
    }
  }

  $effect(() => {
    if ($viewModel.renderDot && graphvizReady) {
      renderGraph($viewModel.renderDot);
    } else if (!$viewModel.renderDot && container) {
      container.innerHTML = '';
    }
  });
</script>

<div class="flex-1 overflow-auto p-4 bg-background">
  <Card class="h-full flex items-center justify-center bg-white [&_svg]:max-w-full [&_svg]:max-h-full">
    <div bind:this={container} class="w-full h-full flex items-center justify-center">
      {#if !graphvizReady}
        <div class="flex flex-col items-center gap-4">
          <Skeleton class="h-32 w-64" />
          <p class="text-muted-foreground text-sm">Loading Graphviz...</p>
        </div>
      {:else if renderError}
        <p class="text-destructive text-sm">{renderError}</p>
      {:else if !$viewModel.renderDot}
        <p class="text-muted-foreground text-sm">No uncommitted changes. Waiting for file changes...</p>
      {/if}
    </div>
  </Card>
</div>
