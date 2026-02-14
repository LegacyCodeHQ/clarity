<script lang="ts">
  import { viewModel } from '../lib/stores/graphStore';
  import SourceSelector from './SourceSelector.svelte';

  interface Props {
    pageTitle: string;
    connected: boolean;
  }

  let { pageTitle, connected }: Props = $props();

  const statusText = $derived(
    connected ? 'Connected' : 'Reconnecting...'
  );
</script>

<div class="header">
  <span class="title">{pageTitle}</span>
  <span class="status" class:disconnected={!connected}>
    <span class="dot"></span>
    <span class="status-text">{statusText}</span>
  </span>
  <span class="spacer"></span>
  <label for="snapshot-source">View:</label>
  <SourceSelector />
</div>

<style>
  .header {
    padding: 8px 16px;
    background: #16213e;
    border-bottom: 1px solid #0f3460;
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 13px;
  }

  .title {
    font-weight: 600;
  }

  .spacer {
    flex: 1;
  }

  .status {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #4ade80;
    transition: background 0.3s;
  }

  .status.disconnected .dot {
    background: #f87171;
  }

  label {
    font-size: 12px;
  }
</style>
