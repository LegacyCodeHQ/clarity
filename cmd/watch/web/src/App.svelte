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

<div class="h-screen flex flex-col">
  <Header {pageTitle} {connected} />
  <GraphContainer />
  <Timeline />
</div>
