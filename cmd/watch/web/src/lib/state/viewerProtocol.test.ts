import { describe, it, expect } from 'vitest';
import { normalizeGraphStreamPayload, type Snapshot, type Collection } from './viewerProtocol';

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

describe('normalizeGraphStreamPayload', () => {
  it('filters malformed snapshot and collection data', () => {
    const normalized = normalizeGraphStreamPayload({
      workingSnapshots: [
        snapshot(1),
        { id: 2, timestamp: TIMESTAMP }, // missing dot
        null,
      ],
      pastCollections: [
        collection(10, [snapshot(7), { id: 8, timestamp: TIMESTAMP }]), // inner snapshot missing dot
        { id: 11, timestamp: TIMESTAMP }, // missing snapshots array
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
      workingSnapshots: [],
      pastCollections: [],
    });
  });
});
