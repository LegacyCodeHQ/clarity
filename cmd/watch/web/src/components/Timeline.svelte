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

  // Calculate fill percentage for progress bar effect
  $: fillPercentage = Number($viewModel.timeline.sliderMax) > 0
    ? (Number($viewModel.timeline.sliderValue) / Number($viewModel.timeline.sliderMax)) * 100
    : 0;

  // Generate background gradient for the slider fill
  $: sliderBackground = `linear-gradient(to right, hsl(207, 61%, 59%) 0%, hsl(207, 61%, 59%) ${fillPercentage}%, hsl(0, 0%, 24%) ${fillPercentage}%, hsl(0, 0%, 24%) 100%)`;
</script>

<style>
  .timeline-slider {
    height: 6px;
    -webkit-appearance: none;
    appearance: none;
    border-radius: 3px;
    outline: none;
    border: 1px solid rgba(255, 255, 255, 0.1);
  }

  .timeline-slider::-webkit-slider-track {
    height: 6px;
    background: transparent;
    border-radius: 3px;
  }

  .timeline-slider::-moz-range-track {
    height: 6px;
    background: transparent;
    border-radius: 3px;
  }

  .timeline-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 14px;
    height: 14px;
    background: hsl(207, 61%, 59%);
    border-radius: 50%;
    cursor: grab;
    transition: all 0.2s ease;
    box-shadow: 0 0 0 0 rgba(86, 156, 214, 0);
  }

  .timeline-slider::-moz-range-thumb {
    width: 14px;
    height: 14px;
    background: hsl(207, 61%, 59%);
    border-radius: 50%;
    border: none;
    cursor: grab;
    transition: all 0.2s ease;
    box-shadow: 0 0 0 0 rgba(86, 156, 214, 0);
  }

  .timeline-slider:not(:disabled):hover::-webkit-slider-thumb {
    transform: scale(1.15);
    box-shadow: 0 0 0 4px rgba(86, 156, 214, 0.15);
  }

  .timeline-slider:not(:disabled):hover::-moz-range-thumb {
    transform: scale(1.15);
    box-shadow: 0 0 0 4px rgba(86, 156, 214, 0.15);
  }

  .timeline-slider:not(:disabled):active::-webkit-slider-thumb {
    cursor: grabbing;
    transform: scale(1.1);
    box-shadow: 0 0 0 6px rgba(86, 156, 214, 0.2);
  }

  .timeline-slider:not(:disabled):active::-moz-range-thumb {
    cursor: grabbing;
    transform: scale(1.1);
    box-shadow: 0 0 0 6px rgba(86, 156, 214, 0.2);
  }

  .timeline-slider:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .timeline-slider:disabled::-webkit-slider-thumb {
    cursor: not-allowed;
    background: hsl(0, 0%, 52%);
  }

  .timeline-slider:disabled::-moz-range-thumb {
    cursor: not-allowed;
    background: hsl(0, 0%, 52%);
  }
</style>

<div class="px-4 py-2.5 bg-card border-t border-border flex items-center gap-4">
  <input
    type="range"
    class="timeline-slider flex-1 min-w-[120px] cursor-pointer"
    style="background: {sliderBackground}"
    min="0"
    max={$viewModel.timeline.sliderMax}
    value={$viewModel.timeline.sliderValue}
    disabled={$viewModel.timeline.sliderDisabled}
    oninput={handleSliderInput}
  />
  <Button
    variant="ghost"
    size="sm"
    disabled={$viewModel.timeline.liveButtonDisabled}
    onclick={handleJumpToLatest}
    class="text-xs"
  >
    Live
  </Button>
  <span class="min-w-[100px] text-right text-xs text-muted-foreground">{$viewModel.timeline.metaText}</span>
</div>
