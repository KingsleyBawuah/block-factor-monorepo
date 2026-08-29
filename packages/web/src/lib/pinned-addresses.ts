import type { components } from "./api/schema";

type ScoreResponse = components["schemas"]["ScoreResponse"];

export type PinnedAddress = {
  formattedAddress: string;
  lat: number;
  lon: number;
  /**
   * A frozen snapshot of the score at pin time, not live-rescored on every
   * weight change -- comparing a moving target against itself would defeat
   * the point of a side-by-side comparison, and re-fetching up to 3 scores
   * on every slider tick would add real network chatter for little benefit.
   */
  score: ScoreResponse;
};

/** Bottom Multi-Address Comparison Tray caps at 3 pinned addresses (docs/IMPLEMENTATION_PHASES.md Phase 5 deliverable 3). */
export const MAX_PINNED_ADDRESSES = 3;

export function isPinned(pinned: PinnedAddress[], lat: number, lon: number): boolean {
  return pinned.some((p) => p.lat === lat && p.lon === lon);
}

/** No-ops (returns the same array reference) if already pinned or at capacity, rather than throwing -- callers gate the pin action on a disabled button, this is the safety net. */
export function pinAddress(pinned: PinnedAddress[], next: PinnedAddress): PinnedAddress[] {
  if (isPinned(pinned, next.lat, next.lon) || pinned.length >= MAX_PINNED_ADDRESSES) {
    return pinned;
  }
  return [...pinned, next];
}

export function unpinAddress(pinned: PinnedAddress[], index: number): PinnedAddress[] {
  return pinned.filter((_, i) => i !== index);
}
