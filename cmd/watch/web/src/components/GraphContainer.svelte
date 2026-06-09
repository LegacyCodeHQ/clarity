<script lang="ts">
  import { onMount } from 'svelte';
  import { viewModel } from '../lib/stores/graphStore';
  import { initGraphviz, renderDot } from '../lib/graphviz';
  import { renderMermaid } from '../lib/mermaid';
  import {
    beginRender,
    cancelPendingRenders,
    clearRenderedDot,
    completeRender,
    createRenderState,
    shouldRenderDot,
  } from '../lib/viewer/renderSequence';
  import Skeleton from '../lib/components/ui/skeleton.svelte';

  let graphContainer: HTMLDivElement;
  let graphvizReady = $state(false);
  let renderError = $state<string | null>(null);
  // Internal render bookkeeping; this should not participate in Svelte reactivity.
  let renderState = createRenderState();

  onMount(async () => {
    try {
      await initGraphviz();
      graphvizReady = true;
    } catch (err) {
      console.error('Failed to initialize Graphviz:', err);
      renderError = 'Failed to load Graphviz';
    }
  });

  // Mermaid is rendered lazily and needs no eager init, so the dot renderer's
  // readiness only gates the dot path.
  let rendererReady = $derived(
    ($viewModel.renderFormat ?? 'dot') === 'mermaid' ? true : graphvizReady,
  );

  async function renderGraph(source: string, format: string) {
    if (!graphContainer) return;
    if (format !== 'mermaid' && !graphvizReady) return;

    const started = beginRender(renderState);
    renderState = started.state;
    const requestID = started.requestID;

    try {
      const svg = format === 'mermaid' ? await renderMermaid(source) : await renderDot(source);
      renderState = completeRender(renderState, requestID, source);
      if (requestID !== renderState.activeRequestID) {
        return;
      }

      graphContainer.innerHTML = svg;
      renderError = null;
    } catch (err) {
      if (requestID !== renderState.activeRequestID) {
        return;
      }

      console.error('Graph render error:', err);
      renderError = 'Render error';
    }
  }

  $effect(() => {
    const source = $viewModel.renderDot;
    const format = $viewModel.renderFormat ?? 'dot';
    if (source && rendererReady) {
      if (shouldRenderDot(renderState, source)) {
        renderGraph(source, format);
      }
    } else if (!source && graphContainer) {
      renderState = cancelPendingRenders(renderState);
      renderState = clearRenderedDot(renderState);
      graphContainer.innerHTML = '';
    }
  });
</script>

<div class="flex-1 overflow-auto bg-background">
  <div class="h-full flex items-center justify-center bg-[#2a2a2a] shadow-[inset_0_2px_8px_rgba(0,0,0,0.3)] [&_svg]:max-w-full [&_svg]:max-h-full relative">
    <!-- Graph rendering container (DOM manipulated) -->
    <div bind:this={graphContainer} class="w-full h-full flex items-center justify-center p-12 transition-opacity duration-300 [&_svg]:transition-all [&_svg]:duration-300"></div>

    <!-- Message container (Svelte managed) -->
    {#if !rendererReady}
      <div class="absolute inset-0 flex items-center justify-center">
        <div class="flex flex-col items-center gap-4 animate-fade-in">
          <Skeleton class="h-24 w-48" />
          <p class="text-muted-foreground text-sm">Loading renderer...</p>
        </div>
      </div>
    {:else if renderError}
      <div class="absolute inset-0 flex items-center justify-center">
        <p class="text-destructive text-sm font-medium">{renderError}</p>
      </div>
    {:else if !$viewModel.renderDot}
      <div class="absolute inset-0 flex items-center justify-center">
        <div class="flex flex-col items-center gap-3 text-center animate-fade-in">
          <svg class="w-16 h-16 text-muted-foreground/40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          <div>
            <p class="text-muted-foreground font-medium mb-1">Waiting for changes</p>
            <p class="text-muted-foreground/60 text-xs max-w-xs">Make changes to your files to see the dependency graph appear here</p>
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>
