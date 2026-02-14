import { describe, it, expect } from 'vitest';
import {
  applyLiveSelection,
  applySliderInput,
  applySourceSelection,
  formatSnapshotMeta,
  getViewModel,
  mergePayload,
  type ViewerState,
  type Snapshot,
  type Collection,
} from './viewerState';

const TIMESTAMP = "2026-02-12T10:00:00Z";

function snapshot(id: number, dot = `digraph ${id} {}`): Snapshot {
  return { id, timestamp: TIMESTAMP, dot };
}

function collection(id: number, snapshots: Snapshot[]): Collection {
  return {
    id,
    timestamp: TIMESTAMP,
    snapshots,
  };
}

function baseState(): ViewerState {
  return {
    workingSnapshots: [],
    pastCollections: [],
    selectedCollectionID: null,
    selectedCollectionSnapshotIndex: 0,
    liveSnapshotIndex: null,
  };
}

describe('mergePayload', () => {
  it('resets live snapshot index when no working snapshots remain', () => {
    const state: ViewerState = {
      ...baseState(),
      workingSnapshots: [snapshot(1), snapshot(2)],
      liveSnapshotIndex: 0,
    };

    const next = mergePayload(state, {
      workingSnapshots: [],
      pastCollections: [],
    });

    expect(next.liveSnapshotIndex).toBe(null);
    expect(next.workingSnapshots).toEqual([]);
  });

  it('falls back to live mode when selected collection disappears', () => {
    const state: ViewerState = {
      ...baseState(),
      selectedCollectionID: 42,
      selectedCollectionSnapshotIndex: 3,
    };

    const next = mergePayload(state, {
      workingSnapshots: [snapshot(7)],
      pastCollections: [collection(1, [snapshot(10)])],
    });

    expect(next.selectedCollectionID).toBe(null);
    expect(next.selectedCollectionSnapshotIndex).toBe(0);
  });
});

describe('applySourceSelection', () => {
  it('returns to live mode for invalid source values', () => {
    const state: ViewerState = {
      ...baseState(),
      selectedCollectionID: 5,
      selectedCollectionSnapshotIndex: 2,
      liveSnapshotIndex: 1,
    };

    const invalid = applySourceSelection(state, "bad-value");
    expect(invalid.selectedCollectionID).toBe(null);
    expect(invalid.selectedCollectionSnapshotIndex).toBe(0);
    expect(invalid.liveSnapshotIndex).toBe(null);

    const nanID = applySourceSelection(state, "collection:abc");
    expect(nanID.selectedCollectionID).toBe(null);
    expect(nanID.selectedCollectionSnapshotIndex).toBe(0);
  });

  it('selects finite collection id and resets collection index', () => {
    const state: ViewerState = {
      ...baseState(),
      selectedCollectionSnapshotIndex: 4,
      pastCollections: [collection(123, [snapshot(1), snapshot(2)])],
    };

    const next = applySourceSelection(state, "collection:123");

    expect(next.selectedCollectionID).toBe(123);
    expect(next.selectedCollectionSnapshotIndex).toBe(0);
  });
});

describe('applySliderInput', () => {
  it('in live mode clamps and maps latest index to null', () => {
    const state: ViewerState = {
      ...baseState(),
      workingSnapshots: [snapshot(1), snapshot(2), snapshot(3)],
    };

    const older = applySliderInput(state, "1");
    expect(older.liveSnapshotIndex).toBe(1);

    const latest = applySliderInput(state, "50");
    expect(latest.liveSnapshotIndex).toBe(null);

    const negative = applySliderInput(state, "-8");
    expect(negative.liveSnapshotIndex).toBe(0);
  });

  it('in collection mode clamps snapshot index', () => {
    const state: ViewerState = {
      ...baseState(),
      selectedCollectionID: 9,
      selectedCollectionSnapshotIndex: 1,
      pastCollections: [collection(9, [snapshot(1), snapshot(2), snapshot(3)])],
    };

    const high = applySliderInput(state, "20");
    expect(high.selectedCollectionSnapshotIndex).toBe(2);

    const low = applySliderInput(state, "-1");
    expect(low.selectedCollectionSnapshotIndex).toBe(0);
  });
});

describe('getViewModel', () => {
  it('returns waiting state metadata for empty live snapshots', () => {
    const vm = getViewModel(baseState(), () => "10:00:00");
    expect(vm.renderDot).toBe(null);
    expect(vm.timeline.modeText).toBe("Working directory (live)");
    expect(vm.timeline.metaText).toBe("0 working snapshots");
    expect(vm.timeline.sliderDisabled).toBe(true);
    expect(vm.sourceValue).toBe("live");
  });

  it('returns selected live snapshot dot and metadata', () => {
    const state: ViewerState = {
      ...baseState(),
      workingSnapshots: [snapshot(1, "digraph one {}"), snapshot(2, "digraph two {}")],
      liveSnapshotIndex: 0,
    };

    const vm = getViewModel(state, () => "10:00:00");
    expect(vm.renderDot).toBe("digraph one {}");
    expect(vm.timeline.modeText).toBe("Working directory snapshot");
    expect(vm.timeline.metaText).toBe(
      "2 working snapshots | #1/2 | id 1 | 10:00:00"
    );
  });
});

describe('applyLiveSelection', () => {
  it('resets to live mode baseline', () => {
    const state: ViewerState = {
      ...baseState(),
      selectedCollectionID: 8,
      selectedCollectionSnapshotIndex: 2,
      liveSnapshotIndex: 1,
    };

    const next = applyLiveSelection(state);

    expect(next.selectedCollectionID).toBe(null);
    expect(next.selectedCollectionSnapshotIndex).toBe(0);
    expect(next.liveSnapshotIndex).toBe(null);
  });
});

describe('formatSnapshotMeta', () => {
  it('renders snapshot position, id, and time', () => {
    const result = formatSnapshotMeta(snapshot(99), 1, 3, () => "11:11:11");
    expect(result).toBe("#2/3 | id 99 | 11:11:11");
  });
});
