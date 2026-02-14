/**
 * Svelte store wrapper for viewer state management.
 * Provides reactive state updates and view model derivation.
 */

import { writable, derived } from 'svelte/store';
import {
  normalizeState,
  mergePayload,
  applySliderInput,
  applyLiveSelection,
  applySourceSelection,
  getViewModel,
  type ViewerState,
  type ViewModel,
} from '../state/viewerState';
import type { GraphStreamPayload } from '../state/viewerProtocol';

function createGraphStore() {
  const initialState: ViewerState = normalizeState({
    workingSnapshots: [],
    pastCollections: [],
    selectedCollectionID: null,
    selectedCollectionSnapshotIndex: 0,
    liveSnapshotIndex: null,
  });

  const { subscribe, update } = writable<ViewerState>(initialState);

  return {
    subscribe,

    /**
     * Merge a new payload from the SSE stream
     */
    mergePayload: (payload: GraphStreamPayload) => {
      update(state => mergePayload(state, payload));
    },

    /**
     * Handle slider input change
     */
    onSliderInput: (rawValue: string) => {
      update(state => applySliderInput(state, rawValue));
    },

    /**
     * Jump to latest snapshot (live mode)
     */
    onJumpToLatest: () => {
      update(state => applyLiveSelection(state));
    },

    /**
     * Handle source selection change (live or collection)
     */
    onSourceChange: (selected: string) => {
      update(state => applySourceSelection(state, selected));
    },

    /**
     * Reset to initial state
     */
    reset: () => {
      update(() => initialState);
    },
  };
}

export const graphStore = createGraphStore();

/**
 * Derived store that computes the view model from the current state
 */
export const viewModel = derived<typeof graphStore, ViewModel>(
  graphStore,
  ($state) => getViewModel($state)
);
