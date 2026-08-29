import { describe, it, expect } from "bun:test";
import { searchToWeights, validateWeightSearch, weightsToSearch } from "./weight-search-params";
import { weightsForPreset } from "./weights";

describe("weightsToSearch / searchToWeights", () => {
  it("round-trips a weights object through the w_-prefixed search shape", () => {
    const weights = weightsForPreset("hybridCommuter");
    expect(searchToWeights(weightsToSearch(weights))).toEqual(weights);
  });

  it("uses flat w_-prefixed keys, matching docs/ARCHITECTURE.md's ?w_transit=35 example", () => {
    const search = weightsToSearch(weightsForPreset("remoteWorker"));
    expect(search).toEqual({
      w_transit: 25,
      w_bike: 12,
      w_grocery: 18,
      w_thirdPlace: 13,
      w_green: 9,
      w_noise: 9,
      w_safety: 9,
      w_flood: 3,
      w_solar: 2,
    });
  });
});

describe("validateWeightSearch", () => {
  it("defaults to the Remote Worker preset when the URL has no w_* params at all", () => {
    expect(validateWeightSearch({})).toEqual(weightsToSearch(weightsForPreset("remoteWorker")));
  });

  it("ignores unrelated search params when deciding whether any weight param is present", () => {
    expect(validateWeightSearch({ address: "233 S Wacker Dr" })).toEqual(
      weightsToSearch(weightsForPreset("remoteWorker"))
    );
  });

  it("reads every factor from the URL once any w_* param is present, treating a missing one as 0 rather than defaulting it from the preset", () => {
    const result = validateWeightSearch({ w_transit: "50", w_bike: "50" });
    expect(searchToWeights(result)).toEqual({
      transit: 50,
      bike: 50,
      grocery: 0,
      thirdPlace: 0,
      green: 0,
      noise: 0,
      safety: 0,
      flood: 0,
      solar: 0,
    });
  });

  it("renormalizes a URL whose weights don't sum to 100", () => {
    const result = validateWeightSearch({ w_transit: "10", w_bike: "10" });
    const weights = searchToWeights(result);
    expect(weights.transit + weights.bike).toBe(100);
  });

  it("treats a non-numeric or negative value as 0 rather than throwing", () => {
    const result = validateWeightSearch({ w_transit: "not-a-number", w_bike: "-40", w_grocery: "60" });
    const weights = searchToWeights(result);
    expect(weights.transit).toBe(0);
    expect(weights.bike).toBe(0);
    expect(weights.grocery).toBe(100);
  });

  it("always returns weights summing to exactly 100", () => {
    const result = searchToWeights(validateWeightSearch({ w_solar: "1" }));
    const total = Object.values(result).reduce((sum, v) => sum + v, 0);
    expect(total).toBe(100);
  });
});
