import { describe, it, expect } from 'vitest';
import {
  applyLiveSelection,
  applySliderInput,
  applyTimelineStep,
  applySourceSelection,
  formatSnapshotMeta,
  getViewModel,
  mergePayload,
  selectRepo,
  type ViewerState,
} from './viewerState';
import type { Snapshot, Collection } from '../protocol/viewerProtocol';

const TIMESTAMP = "2026-02-12T10:00:00Z";

function snapshot(id: number, dot = `digraph ${id} {}`, repoId = "primary"): Snapshot {
  return { id, repoId, timestamp: TIMESTAMP, dot };
}

function collection(id: number, snapshots: Snapshot[], repoId = "primary"): Collection {
  return {
    id,
    repoId,
    timestamp: TIMESTAMP,
    snapshots,
    commitHistory: [],
  };
}

function baseState(): ViewerState {
  return {
    repos: [],
    selectedRepoID: "primary",
    byRepo: {},
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

describe('applyTimelineStep', () => {
  it('steps through live snapshots and maps the latest index back to live mode', () => {
    const state: ViewerState = {
      ...baseState(),
      workingSnapshots: [snapshot(1), snapshot(2), snapshot(3)],
    };

    const older = applyTimelineStep(state, -1);
    expect(older.liveSnapshotIndex).toBe(1);

    const oldest = applyTimelineStep(older, -1);
    expect(oldest.liveSnapshotIndex).toBe(0);

    const clamped = applyTimelineStep(oldest, -1);
    expect(clamped.liveSnapshotIndex).toBe(0);

    const newer = applyTimelineStep(oldest, 1);
    expect(newer.liveSnapshotIndex).toBe(1);

    const latest = applyTimelineStep(newer, 1);
    expect(latest.liveSnapshotIndex).toBe(null);
  });

  it('steps within selected collections without leaving collection mode', () => {
    const state: ViewerState = {
      ...baseState(),
      selectedCollectionID: 9,
      selectedCollectionSnapshotIndex: 1,
      pastCollections: [collection(9, [snapshot(1), snapshot(2), snapshot(3)])],
    };

    const older = applyTimelineStep(state, -1);
    expect(older.selectedCollectionID).toBe(9);
    expect(older.selectedCollectionSnapshotIndex).toBe(0);

    const clamped = applyTimelineStep(older, -1);
    expect(clamped.selectedCollectionSnapshotIndex).toBe(0);

    const newer = applyTimelineStep(older, 1);
    expect(newer.selectedCollectionSnapshotIndex).toBe(1);
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

  it('exposes the session start index in the live working set', () => {
    const state: ViewerState = {
      ...baseState(),
      workingSnapshots: [
        { ...snapshot(1, "digraph one {}"), sessionStart: true },
        snapshot(2, "digraph two {}"),
      ],
    };

    const vm = getViewModel(state, () => "10:00:00");
    expect(vm.timeline.sessionStartIndex).toBe(0);
  });

  it('reports a null session start index when no snapshot is marked', () => {
    const state: ViewerState = {
      ...baseState(),
      workingSnapshots: [snapshot(1, "digraph one {}"), snapshot(2, "digraph two {}")],
    };

    const vm = getViewModel(state, () => "10:00:00");
    expect(vm.timeline.sessionStartIndex).toBe(null);
  });

  it('exposes the session start index within a selected collection', () => {
    const sessionSnap = { ...snapshot(1, "digraph one {}"), sessionStart: true };
    const state: ViewerState = {
      ...baseState(),
      pastCollections: [collection(10, [sessionSnap, snapshot(2, "digraph two {}")])],
      selectedCollectionID: 10,
    };

    const vm = getViewModel(state, () => "10:00:00");
    expect(vm.timeline.sessionStartIndex).toBe(0);
  });

  it('labels archived source options as sessions', () => {
    const state: ViewerState = {
      ...baseState(),
      pastCollections: [collection(10, [snapshot(1), snapshot(2)])],
    };

    const vm = getViewModel(state, () => "10:00:00");
    expect(vm.sourceOptions.find((option) => option.value === "collection:10")?.text).toBe(
      "Session 1 (2 snapshots, 10:00:00)"
    );
  });

  it('omits the live source option when the selected worktree is deleted', () => {
    const state = mergePayload(baseState(), {
      repos: [
        { id: "primary", path: "/p", label: "primary", isPrimary: true, active: true },
        { id: "wt-aaaaaaaa", path: "/wt", label: "wt", isPrimary: false, active: false },
      ],
      workingSnapshots: [snapshot(1, "digraph p {}", "primary")],
      pastCollections: [collection(10, [snapshot(2, "digraph w {}", "wt-aaaaaaaa")], "wt-aaaaaaaa")],
    });

    const vm = getViewModel(selectRepo(state, "wt-aaaaaaaa"), () => "10:00:00");

    expect(vm.sourceValue).toBe("collection:10");
    expect(vm.sourceOptions.map((option) => option.value)).toEqual(["collection:10"]);
    expect(vm.timeline.liveButtonDisabled).toBe(true);
    expect(vm.renderDot).toBe("digraph w {}");
  });

  it('treats deleted worktree working snapshots as frozen instead of live', () => {
    const state = mergePayload(baseState(), {
      repos: [
        { id: "primary", path: "/p", label: "primary", isPrimary: true, active: true },
        { id: "wt-aaaaaaaa", path: "/wt", label: "wt", isPrimary: false, active: false },
      ],
      workingSnapshots: [
        snapshot(1, "digraph p {}", "primary"),
        snapshot(2, "digraph frozen {}", "wt-aaaaaaaa"),
      ],
      pastCollections: [],
    });

    const vm = getViewModel(selectRepo(state, "wt-aaaaaaaa"), () => "10:00:00");

    expect(vm.sourceValue).toBe("frozen");
    expect(vm.sourceOptions.map((option) => option.value)).toEqual(["frozen"]);
    expect(vm.sourceOptions[0].text).not.toContain("live");
    expect(vm.timeline.modeText).toBe("Removed working directory snapshot");
    expect(vm.timeline.liveButtonDisabled).toBe(true);
    expect(vm.renderDot).toBe("digraph frozen {}");
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

describe('selectRepo', () => {
  it('switches the active tab and reprojects working snapshots', () => {
    const state = mergePayload(baseState(), {
      repos: [
        { id: "primary", path: "/p", label: "primary", isPrimary: true, active: true },
        { id: "wt-aaaaaaaa", path: "/wt", label: "wt", isPrimary: false, active: true },
      ],
      workingSnapshots: [
        snapshot(1, "digraph p {}", "primary"),
        snapshot(2, "digraph w {}", "wt-aaaaaaaa"),
      ],
      pastCollections: [],
    });

    expect(state.selectedRepoID).toBe("primary");
    expect(state.workingSnapshots).toEqual([snapshot(1, "digraph p {}", "primary")]);

    const switched = selectRepo(state, "wt-aaaaaaaa");
    expect(switched.selectedRepoID).toBe("wt-aaaaaaaa");
    expect(switched.workingSnapshots).toEqual([snapshot(2, "digraph w {}", "wt-aaaaaaaa")]);
  });

  it('ignores selection for unknown repo id', () => {
    const state = mergePayload(baseState(), {
      repos: [{ id: "primary", path: "/p", label: "primary", isPrimary: true, active: true }],
      workingSnapshots: [snapshot(1)],
      pastCollections: [],
    });

    const same = selectRepo(state, "nonexistent");
    expect(same).toBe(state);
  });

  it('falls back to primary if the previously selected repo disappears', () => {
    let state = mergePayload(baseState(), {
      repos: [
        { id: "primary", path: "/p", label: "primary", isPrimary: true, active: true },
        { id: "wt-aaaaaaaa", path: "/wt", label: "wt", isPrimary: false, active: true },
      ],
      workingSnapshots: [
        snapshot(1, "digraph p {}", "primary"),
        snapshot(2, "digraph w {}", "wt-aaaaaaaa"),
      ],
      pastCollections: [],
    });
    state = selectRepo(state, "wt-aaaaaaaa");
    expect(state.selectedRepoID).toBe("wt-aaaaaaaa");

    // Simulate the worktree being removed: payload no longer lists it.
    const next = mergePayload(state, {
      repos: [{ id: "primary", path: "/p", label: "primary", isPrimary: true, active: true }],
      workingSnapshots: [snapshot(1, "digraph p {}", "primary")],
      pastCollections: [],
    });
    expect(next.selectedRepoID).toBe("primary");
    expect(next.workingSnapshots).toEqual([snapshot(1, "digraph p {}", "primary")]);
  });

  it('resets timeline selection when switching tabs', () => {
    let state = mergePayload(baseState(), {
      repos: [
        { id: "primary", path: "/p", label: "primary", isPrimary: true, active: true },
        { id: "wt-aaaaaaaa", path: "/wt", label: "wt", isPrimary: false, active: true },
      ],
      workingSnapshots: [
        snapshot(1, "digraph p1 {}", "primary"),
        snapshot(2, "digraph p2 {}", "primary"),
        snapshot(3, "digraph w {}", "wt-aaaaaaaa"),
      ],
      pastCollections: [],
    });
    state = applySliderInput(state, "0");
    expect(state.liveSnapshotIndex).toBe(0);

    const switched = selectRepo(state, "wt-aaaaaaaa");
    expect(switched.liveSnapshotIndex).toBe(null);
    expect(switched.selectedCollectionID).toBe(null);
  });
});

describe('formatSnapshotMeta', () => {
  it('renders snapshot position, id, and time', () => {
    const result = formatSnapshotMeta(snapshot(99), 1, 3, () => "11:11:11");
    expect(result).toBe("#2/3 | id 99 | 11:11:11");
  });
});
