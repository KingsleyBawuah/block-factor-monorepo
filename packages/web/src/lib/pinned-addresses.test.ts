import { describe, it, expect } from "bun:test";
import { MAX_PINNED_ADDRESSES, isPinned, pinAddress, unpinAddress, type PinnedAddress } from "./pinned-addresses";

function fakePin(overrides: Partial<PinnedAddress> = {}): PinnedAddress {
  return {
    formattedAddress: "233 S Wacker Dr, Chicago, IL",
    lat: 41.8788,
    lon: -87.6359,
    score: { compositeScore: 81, factors: {} } as PinnedAddress["score"],
    ...overrides,
  };
}

describe("pinAddress", () => {
  it("appends a new address to an empty list", () => {
    const result = pinAddress([], fakePin());
    expect(result).toHaveLength(1);
    expect(result[0].formattedAddress).toBe("233 S Wacker Dr, Chicago, IL");
  });

  it("appends up to MAX_PINNED_ADDRESSES distinct addresses", () => {
    let pinned: PinnedAddress[] = [];
    for (let i = 0; i < MAX_PINNED_ADDRESSES; i++) {
      pinned = pinAddress(pinned, fakePin({ lat: 41.8 + i, lon: -87.6 + i }));
    }
    expect(pinned).toHaveLength(MAX_PINNED_ADDRESSES);
  });

  it("no-ops (same reference) once already at MAX_PINNED_ADDRESSES", () => {
    let pinned: PinnedAddress[] = [];
    for (let i = 0; i < MAX_PINNED_ADDRESSES; i++) {
      pinned = pinAddress(pinned, fakePin({ lat: 41.8 + i, lon: -87.6 + i }));
    }
    const result = pinAddress(pinned, fakePin({ lat: 42.5, lon: -88.5 }));
    expect(result).toBe(pinned);
    expect(result).toHaveLength(MAX_PINNED_ADDRESSES);
  });

  it("no-ops (same reference) when the exact coordinate is already pinned, rather than adding a duplicate", () => {
    const pinned = pinAddress([], fakePin());
    const result = pinAddress(pinned, fakePin({ formattedAddress: "a different label, same coordinate" }));
    expect(result).toBe(pinned);
    expect(result).toHaveLength(1);
  });
});

describe("unpinAddress", () => {
  it("removes only the address at the given index", () => {
    const pinned = [fakePin({ lat: 1, lon: 1 }), fakePin({ lat: 2, lon: 2 }), fakePin({ lat: 3, lon: 3 })];
    const result = unpinAddress(pinned, 1);
    expect(result.map((p) => p.lat)).toEqual([1, 3]);
  });

  it("returns an empty array when unpinning the only pinned address", () => {
    const pinned = [fakePin()];
    expect(unpinAddress(pinned, 0)).toEqual([]);
  });
});

describe("isPinned", () => {
  it("returns true for a coordinate already in the list", () => {
    const pinned = [fakePin({ lat: 41.8788, lon: -87.6359 })];
    expect(isPinned(pinned, 41.8788, -87.6359)).toBe(true);
  });

  it("returns false for a coordinate not in the list", () => {
    const pinned = [fakePin({ lat: 41.8788, lon: -87.6359 })];
    expect(isPinned(pinned, 41.88, -87.63)).toBe(false);
  });

  it("returns false for an empty list", () => {
    expect(isPinned([], 41.8788, -87.6359)).toBe(false);
  });
});
