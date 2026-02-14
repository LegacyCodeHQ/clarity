<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Header from './components/Header.svelte';
  import GraphContainer from './components/GraphContainer.svelte';
  import Timeline from './components/Timeline.svelte';
  import { graphStore } from './lib/stores/graphStore';
  import { normalizeGraphStreamPayload } from './lib/state/viewerProtocol';

  interface Props {
    pageTitle: string;
  }

  let { pageTitle }: Props = $props();

  let connected = $state(false);
  let eventSource: EventSource | null = null;

  function connectSSE() {
    eventSource = new EventSource('/events');

    eventSource.addEventListener('graph', (event) => {
      try {
        const payload = normalizeGraphStreamPayload(JSON.parse(event.data));
        graphStore.mergePayload(payload);
      } catch (err) {
        console.error('Invalid graph payload:', err);
      }
    });

    eventSource.addEventListener('open', () => {
      connected = true;
    });

    eventSource.addEventListener('error', () => {
      connected = false;
    });
  }

  onMount(() => {
    connectSSE();
  });

  onDestroy(() => {
    if (eventSource) {
      eventSource.close();
    }
  });
</script>

<div class="app">
  <Header {pageTitle} {connected} />
  <GraphContainer />
  <Timeline />
</div>

<style>
  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: #1a1a2e;
    color: #e0e0e0;
    height: 100vh;
  }

  .app {
    height: 100vh;
    display: flex;
    flex-direction: column;
  }
</style>
