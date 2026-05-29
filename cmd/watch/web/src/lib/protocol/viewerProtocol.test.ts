import { describe, it, expect } from 'vitest';
import { normalizeGraphStreamPayload, type Snapshot, type Collection } from './viewerProtocol';

const TIMESTAMP = "2026-02-12T10:00:00Z";

function snapshot(id: number, repoId = "primary", dot = `digraph ${id} {}`): Snapshot {
  return { id, repoId, timestamp: TIMESTAMP, dot };
}

function collection(id: number, snapshots: Snapshot[], repoId = "primary"): Collection {
  return {
    id,
    repoId,
    timestamp: TIMESTAMP,
    snapshots,
  };
}

describe('normalizeGraphStreamPayload', () => {
  it('filters malformed snapshot and collection data', () => {
    const normalized = normalizeGraphStreamPayload({
      repos: [{ id: "primary", path: "/repo", label: "repo", isPrimary: true }],
      workingSnapshots: [
        snapshot(1),
        { id: 2, repoId: "primary", timestamp: TIMESTAMP }, // missing dot
        null,
      ],
      pastCollections: [
        collection(10, [snapshot(7), { id: 8, repoId: "primary", timestamp: TIMESTAMP }]), // inner snapshot missing dot
        { id: 11, repoId: "primary", timestamp: TIMESTAMP }, // missing snapshots array
        null,
      ],
      latestWorkingId: "bad", // not a number
      latestPastCollectionId: 22,
    });

    expect(normalized.workingSnapshots).toEqual([snapshot(1)]);
    expect(normalized.pastCollections).toEqual([collection(10, [snapshot(7)])]);
    expect(normalized.latestWorkingId).toBe(0);
    expect(normalized.latestPastCollectionId).toBe(22);
  });

  it('handles non-object input', () => {
    expect(normalizeGraphStreamPayload(null)).toEqual({
      repos: [],
      workingSnapshots: [],
      pastCollections: [],
    });
  });

  it('defaults repoId to empty string when missing', () => {
    const normalized = normalizeGraphStreamPayload({
      workingSnapshots: [{ id: 1, timestamp: TIMESTAMP, dot: "digraph {}" }],
      pastCollections: [{
        id: 5,
        timestamp: TIMESTAMP,
        snapshots: [{ id: 2, timestamp: TIMESTAMP, dot: "digraph {}" }],
      }],
    });

    expect(normalized.workingSnapshots[0].repoId).toBe("");
    expect(normalized.pastCollections[0].repoId).toBe("");
    expect(normalized.pastCollections[0].snapshots[0].repoId).toBe("");
  });

  it('normalizes the repos[] tab descriptor list', () => {
    const normalized = normalizeGraphStreamPayload({
      repos: [
        { id: "primary", path: "/repo", label: "clarity-cli", isPrimary: true },
        { id: "wt-abc12345", path: "/tmp/feat", label: "clarity-cli (feat)", isPrimary: false },
        { id: "" }, // empty id should be dropped
        null,
        "garbage",
      ],
      workingSnapshots: [],
      pastCollections: [],
    });

    expect(normalized.repos).toEqual([
      { id: "primary", path: "/repo", label: "clarity-cli", isPrimary: true },
      { id: "wt-abc12345", path: "/tmp/feat", label: "clarity-cli (feat)", isPrimary: false },
    ]);
  });

  it('falls back to repo id as label when label missing', () => {
    const normalized = normalizeGraphStreamPayload({
      repos: [{ id: "primary", path: "/repo", isPrimary: true }],
      workingSnapshots: [],
      pastCollections: [],
    });

    expect(normalized.repos[0].label).toBe("primary");
    expect(normalized.repos[0].isPrimary).toBe(true);
  });
});
