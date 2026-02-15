<script lang="ts">
  interface Props {
    connected: boolean;
  }

  let { connected }: Props = $props();

  const statusText = $derived(
    connected ? 'Connected' : 'Reconnecting…'
  );
</script>

<div
  title={statusText}
  class="w-2 h-2 rounded-full transition-all {connected ? 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.8),0_0_12px_rgba(34,197,94,0.4)]' : 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.8),0_0_12px_rgba(239,68,68,0.4)]'}"
></div>
