<script lang="ts">
  import { viewModel, graphStore } from '../lib/stores/graphStore';
  import Button from '../lib/components/ui/button.svelte';

  function handleSliderInput(event: Event) {
    const target = event.target as HTMLInputElement;
    graphStore.onSliderInput(target.value);
  }

  function handleJumpToLatest() {
    graphStore.onJumpToLatest();
  }
</script>

<div class="px-4 py-2.5 bg-card border-t border-border flex items-center gap-3 text-xs">
  <span class="text-muted-foreground">{$viewModel.timeline.modeText}</span>
  <input
    type="range"
    class="flex-1 min-w-[120px] accent-primary"
    min="0"
    max={$viewModel.timeline.sliderMax}
    value={$viewModel.timeline.sliderValue}
    disabled={$viewModel.timeline.sliderDisabled}
    oninput={handleSliderInput}
  />
  <Button
    size="sm"
    variant="secondary"
    disabled={$viewModel.timeline.liveButtonDisabled}
    onclick={handleJumpToLatest}
  >
    Jump to latest
  </Button>
  <span class="min-w-[120px] text-right text-muted-foreground">{$viewModel.timeline.metaText}</span>
</div>
