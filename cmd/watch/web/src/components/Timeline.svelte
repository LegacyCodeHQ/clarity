<script lang="ts">
  import { onMount } from 'svelte';
  import { viewModel, graphStore } from '../lib/stores/graphStore';
  import { shouldHandleTimelineKeydown } from '../lib/viewer/timelineKeyboard';
  import Button from '../lib/components/ui/button.svelte';

  function handleSliderInput(event: Event) {
    const target = event.target as HTMLInputElement;
    graphStore.onSliderInput(target.value);
  }

  function handleJumpToLatest() {
    graphStore.onJumpToLatest();
  }

  function handleTimelineKeydown(event: KeyboardEvent) {
    if (!shouldHandleTimelineKeydown(event)) {
      return;
    }
    if ($viewModel.timeline.sliderDisabled) {
      return;
    }

    event.preventDefault();
    graphStore.onTimelineStep(event.key === 'ArrowRight' ? 1 : -1);
  }

  onMount(() => {
    document.addEventListener('keydown', handleTimelineKeydown);
    return () => document.removeEventListener('keydown', handleTimelineKeydown);
  });

  // Calculate fill percentage for progress bar effect
  $: fillPercentage = Number($viewModel.timeline.sliderMax) > 0
    ? (Number($viewModel.timeline.sliderValue) / Number($viewModel.timeline.sliderMax)) * 100
    : 0;

  // Generate background gradient for the slider fill
  $: sliderBackground = `linear-gradient(to right, hsl(207, 61%, 59%) 0%, hsl(207, 61%, 59%) ${fillPercentage}%, hsl(0, 0%, 24%) ${fillPercentage}%, hsl(0, 0%, 24%) 100%)`;

  // Session-start marker: pin a tick to the snapshot captured when the watcher
  // attached, so the boundary where history begins is visible on the timeline.
  $: sessionStartIndex = $viewModel.timeline.sessionStartIndex;
  $: sessionStartPercentage = Number($viewModel.timeline.sliderMax) > 0 && sessionStartIndex !== null
    ? (sessionStartIndex / Number($viewModel.timeline.sliderMax)) * 100
    : 0;
  // Map the percentage onto the thumb's travel (half-thumb inset at each end).
  $: sessionStartLeft = `calc(7px + (100% - 14px) * ${sessionStartPercentage} / 100)`;
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

  .slider-track {
    position: relative;
    display: flex;
    align-items: center;
  }

  .session-start-marker {
    position: absolute;
    top: 50%;
    transform: translate(-50%, -50%);
    width: 3px;
    height: 16px;
    background: hsl(38, 92%, 50%);
    border-radius: 2px;
    cursor: help;
    box-shadow: 0 0 5px hsla(38, 92%, 50%, 0.7);
    pointer-events: auto;
  }
</style>

<div class="px-4 py-2.5 bg-card border-t border-border flex items-center gap-4">
  <div class="slider-track flex-1 min-w-[120px]">
    <input
      type="range"
      class="timeline-slider w-full cursor-pointer"
      style="background: {sliderBackground}"
      min="0"
      max={$viewModel.timeline.sliderMax}
      value={$viewModel.timeline.sliderValue}
      disabled={$viewModel.timeline.sliderDisabled}
      oninput={handleSliderInput}
    />
    {#if sessionStartIndex !== null}
      <div
        class="session-start-marker"
        style="left: {sessionStartLeft}"
        title="Session start — clarity began watching here. Changes made before this point aren't in the timeline."
      ></div>
    {/if}
  </div>
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
