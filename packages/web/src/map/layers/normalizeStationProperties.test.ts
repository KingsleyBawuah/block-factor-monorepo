import { describe, it, expect } from "bun:test";
import { normalizeStationProperties } from "./normalizeStationProperties";

describe("normalizeStationProperties", () => {
  it("passes through already-correct types unchanged", () => {
    expect(normalizeStationProperties({ name: "Fullerton", lines: ["Red", "Brown"], ada: true })).toEqual({
      name: "Fullerton",
      lines: ["Red", "Brown"],
      ada: true,
    });
  });

  it("parses a JSON-stringified lines array (pinned maplibre-gl v5 behavior)", () => {
    expect(
      normalizeStationProperties({ name: "Fullerton", lines: '["Red","Brown"]', ada: true })
    ).toEqual({ name: "Fullerton", lines: ["Red", "Brown"], ada: true });
  });

  it("parses a stringified boolean for ada", () => {
    expect(normalizeStationProperties({ name: "Fullerton", lines: [], ada: "true" }).ada).toBe(true);
    expect(normalizeStationProperties({ name: "Fullerton", lines: [], ada: "false" }).ada).toBe(false);
  });

  it("falls back to an empty array for unparseable lines rather than throwing", () => {
    expect(normalizeStationProperties({ name: "Fullerton", lines: "not json", ada: false }).lines).toEqual([]);
  });

  it("falls back to an empty string for a missing/non-string name", () => {
    expect(normalizeStationProperties({ lines: [], ada: false }).name).toBe("");
  });
});
