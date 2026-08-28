import { describe, it, expect } from "bun:test";
import {
  PRESETS,
  WEIGHT_FACTOR_ORDER,
  type Weights,
  matchPreset,
  normalizeToSum100,
  redistributeWeights,
  weightsForPreset,
} from "./weights";

function sum(weights: Weights): number {
  return WEIGHT_FACTOR_ORDER.reduce((total, k) => total + weights[k], 0);
}

describe("weightsForPreset", () => {
  it("returns the Remote Worker preset's 9 active factors unchanged -- they already sum to 100 with commute excluded", () => {
    expect(weightsForPreset("remoteWorker")).toEqual({
      transit: 25,
      bike: 12,
      grocery: 18,
      thirdPlace: 13,
      green: 9,
      noise: 9,
      safety: 9,
      flood: 3,
      solar: 2,
    });
  });

  it("renormalizes the Hybrid Commuter preset's 9 active factors to sum to 100 once commute (17%) is dropped", () => {
    // docs/PRD.md §3.4's raw values (17/10/13/9/9/10/10/3/2) sum to 83
    // without commute -- scaled by 100/83 and apportioned by largest
    // remainder, hand-verified against that same PRD table.
    expect(weightsForPreset("hybridCommuter")).toEqual({
      transit: 20,
      bike: 12,
      grocery: 16,
      thirdPlace: 11,
      green: 11,
      noise: 12,
      safety: 12,
      flood: 4,
      solar: 2,
    });
  });

  it("sums to exactly 100 for both presets", () => {
    expect(sum(weightsForPreset("remoteWorker"))).toBe(100);
    expect(sum(weightsForPreset("hybridCommuter"))).toBe(100);
  });

  it("PRESETS itself still carries commute for both presets, per the PRD table verbatim", () => {
    expect(PRESETS.remoteWorker.commute).toBe(0);
    expect(PRESETS.hybridCommuter.commute).toBe(17);
  });
});

describe("normalizeToSum100", () => {
  it("leaves an already-100 distribution unchanged", () => {
    const preset = weightsForPreset("remoteWorker");
    expect(normalizeToSum100(preset)).toEqual(preset);
  });

  it("scales a distribution that sums to less than 100 up to exactly 100", () => {
    const raw = { transit: 5, bike: 5, grocery: 0, thirdPlace: 0, green: 0, noise: 0, safety: 0, flood: 0, solar: 0 };
    const result = normalizeToSum100(raw);
    expect(sum(result)).toBe(100);
    expect(result.transit).toBe(50);
    expect(result.bike).toBe(50);
  });

  it("splits an all-zero distribution evenly and still sums to 100", () => {
    const raw = { transit: 0, bike: 0, grocery: 0, thirdPlace: 0, green: 0, noise: 0, safety: 0, flood: 0, solar: 0 };
    const result = normalizeToSum100(raw);
    expect(sum(result)).toBe(100);
    // 100 / 9 factors isn't even -- some get 12, some get 11, none get 0.
    for (const k of WEIGHT_FACTOR_ORDER) {
      expect(result[k]).toBeGreaterThanOrEqual(11);
      expect(result[k]).toBeLessThanOrEqual(12);
    }
  });

  it("gives a single nonzero factor the entire 100 when every other factor is 0", () => {
    const raw = { transit: 7, bike: 0, grocery: 0, thirdPlace: 0, green: 0, noise: 0, safety: 0, flood: 0, solar: 0 };
    const result = normalizeToSum100(raw);
    expect(result.transit).toBe(100);
    expect(sum(result)).toBe(100);
  });
});

describe("redistributeWeights", () => {
  const remoteWorker = weightsForPreset("remoteWorker");

  it("is a no-op when the dragged value equals the current value", () => {
    expect(redistributeWeights(remoteWorker, "transit", 25)).toBe(remoteWorker);
  });

  it("sets the dragged key to exactly the new value and keeps the total at 100", () => {
    const result = redistributeWeights(remoteWorker, "transit", 35);
    expect(result.transit).toBe(35);
    expect(sum(result)).toBe(100);
  });

  it("matches a hand-computed redistribution when grocery is dragged from 18 to 30", () => {
    // Others (transit 25, bike 12, thirdPlace 13, green 9, noise 9, safety
    // 9, flood 3, solar 2; sum 82) scale down to fit a new total of 70,
    // apportioned by largest remainder -- hand-verified.
    expect(redistributeWeights(remoteWorker, "grocery", 30)).toEqual({
      transit: 21,
      bike: 10,
      grocery: 30,
      thirdPlace: 11,
      green: 8,
      noise: 8,
      safety: 8,
      flood: 2,
      solar: 2,
    });
  });

  it("gives every other factor to 0 when one slider is dragged to 100", () => {
    const result = redistributeWeights(remoteWorker, "flood", 100);
    expect(result.flood).toBe(100);
    for (const k of WEIGHT_FACTOR_ORDER) {
      if (k !== "flood") expect(result[k]).toBe(0);
    }
    expect(sum(result)).toBe(100);
  });

  it("evenly splits freed weight across the other 8 when they all start at 0 (scaleToSum's zero-fallback branch)", () => {
    const allFlood = redistributeWeights(remoteWorker, "flood", 100); // flood:100, every other factor:0
    const result = redistributeWeights(allFlood, "flood", 60);
    expect(result.flood).toBe(60);
    for (const k of WEIGHT_FACTOR_ORDER) {
      if (k !== "flood") expect(result[k]).toBe(5); // 40 freed / 8 factors, evenly
    }
    expect(sum(result)).toBe(100);
  });

  it("hands a dragged-to-0 slider's freed weight back to the others proportionally", () => {
    const result = redistributeWeights(remoteWorker, "transit", 0);
    expect(result.transit).toBe(0);
    expect(sum(result)).toBe(100);
    // The others (which held 75) should still be in roughly the same
    // relative proportion to each other -- grocery (18) should still
    // outweigh solar (2) after absorbing transit's freed 25 points.
    expect(result.grocery).toBeGreaterThan(result.solar);
  });

  it("clamps a raw value above 100 down to 100 before redistributing", () => {
    const result = redistributeWeights(remoteWorker, "grocery", 250);
    expect(result.grocery).toBe(100);
    expect(sum(result)).toBe(100);
  });

  it("clamps a raw value below 0 up to 0 before redistributing", () => {
    const result = redistributeWeights(remoteWorker, "solar", -20);
    expect(result.solar).toBe(0);
    expect(sum(result)).toBe(100);
  });

  it("always sums to 100 across a sequence of drags on different sliders", () => {
    let weights = remoteWorker;
    weights = redistributeWeights(weights, "transit", 40);
    weights = redistributeWeights(weights, "bike", 5);
    weights = redistributeWeights(weights, "safety", 20);
    weights = redistributeWeights(weights, "solar", 0);
    expect(sum(weights)).toBe(100);
  });
});

describe("matchPreset", () => {
  it("matches the Remote Worker preset exactly", () => {
    expect(matchPreset(weightsForPreset("remoteWorker"))).toBe("remoteWorker");
  });

  it("matches the Hybrid Commuter preset exactly", () => {
    expect(matchPreset(weightsForPreset("hybridCommuter"))).toBe("hybridCommuter");
  });

  it("returns null once a slider has been dragged away from both presets", () => {
    const dragged = redistributeWeights(weightsForPreset("remoteWorker"), "transit", 40);
    expect(matchPreset(dragged)).toBeNull();
  });
});
