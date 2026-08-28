import { describe, it, expect, mock } from "bun:test";
import { createDivvyMarkerIcon } from "./divvyMarkerIcon";

type RecordedCall = { method: string; args: unknown[] };

/**
 * Bun's test runner shares one global/module registry across every test
 * file by default (see src/test/setup.ts's own comment on this) -- so
 * relying on happy-dom's ambient "no canvas 2D" behavior for the null-context
 * test is fragile: it silently passes or fails depending on whatever any
 * other test in the same run last left `HTMLCanvasElement.prototype.getContext`
 * set to, confirmed live via a CI-only flaky failure this assumption caused
 * (passed in isolation, failed as part of the full suite). Both tests here
 * instead explicitly patch and restore the prototype around themselves, so
 * neither depends on the environment's default or on run order.
 */
function withPatchedGetContext<T>(returnValue: T) {
  const original = HTMLCanvasElement.prototype.getContext;
  HTMLCanvasElement.prototype.getContext = mock(() => returnValue) as unknown as typeof original;
  return {
    restore: () => {
      HTMLCanvasElement.prototype.getContext = original;
    },
  };
}

function withMockCanvasContext() {
  const calls: RecordedCall[] = [];
  const state = { fillStyle: "", strokeStyle: "", lineWidth: 0 };

  const ctx = {
    get fillStyle() {
      return state.fillStyle;
    },
    set fillStyle(v: string) {
      state.fillStyle = v;
    },
    get strokeStyle() {
      return state.strokeStyle;
    },
    set strokeStyle(v: string) {
      state.strokeStyle = v;
    },
    get lineWidth() {
      return state.lineWidth;
    },
    set lineWidth(v: number) {
      state.lineWidth = v;
    },
    beginPath: () => calls.push({ method: "beginPath", args: [] }),
    moveTo: (x: number, y: number) => calls.push({ method: "moveTo", args: [x, y] }),
    arc: (...args: unknown[]) => calls.push({ method: "arc", args }),
    closePath: () => calls.push({ method: "closePath", args: [] }),
    fill: () => calls.push({ method: "fill", args: [state.fillStyle] }),
    stroke: () => calls.push({ method: "stroke", args: [state.strokeStyle, state.lineWidth] }),
    getImageData: (x: number, y: number, w: number, h: number) => {
      calls.push({ method: "getImageData", args: [x, y, w, h] });
      return { width: w, height: h, data: new Uint8ClampedArray(w * h * 4) } as ImageData;
    },
  };

  const { restore } = withPatchedGetContext(ctx);
  return { calls, restore };
}

describe("createDivvyMarkerIcon", () => {
  it("returns null when no 2D canvas context is available", () => {
    const { restore } = withPatchedGetContext(null);
    try {
      expect(createDivvyMarkerIcon()).toBeNull();
    } finally {
      restore();
    }
  });

  it("draws two semicircle halves in Divvy Blue and grey, plus a stroke outline, and reads back the full 24x24 image", () => {
    const { calls, restore } = withMockCanvasContext();
    try {
      const icon = createDivvyMarkerIcon();
      expect(icon).not.toBeNull();
      expect(icon?.width).toBe(24);
      expect(icon?.height).toBe(24);

      const fillCalls = calls.filter((c) => c.method === "fill");
      expect(fillCalls.map((c) => c.args[0])).toEqual(["#41b6e6", "#a1a1aa"]);

      const strokeCalls = calls.filter((c) => c.method === "stroke");
      expect(strokeCalls).toHaveLength(1);
      expect(strokeCalls[0].args).toEqual(["#18181b", 1.5]);

      // 2 semicircle fills + 1 full stroke circle, every one centered at
      // (12, 12) with an 11px radius (ICON_SIZE/2 - 1, leaving room for the
      // stroke).
      const arcCalls = calls.filter((c) => c.method === "arc");
      expect(arcCalls).toHaveLength(3);
      for (const call of arcCalls) {
        expect(call.args[0]).toBe(12);
        expect(call.args[1]).toBe(12);
        expect(call.args[2]).toBe(11);
      }
      // Blue half spans PI/2 -> 3PI/2 (the left semicircle); grey spans
      // -PI/2 -> PI/2 (the right semicircle); the stroke covers the full
      // circle, 0 -> 2PI.
      expect(arcCalls[0].args[3]).toBeCloseTo(Math.PI / 2, 10);
      expect(arcCalls[0].args[4]).toBeCloseTo((3 * Math.PI) / 2, 10);
      expect(arcCalls[1].args[3]).toBeCloseTo(-Math.PI / 2, 10);
      expect(arcCalls[1].args[4]).toBeCloseTo(Math.PI / 2, 10);
      expect(arcCalls[2].args[3]).toBe(0);
      expect(arcCalls[2].args[4]).toBeCloseTo(Math.PI * 2, 10);

      const getImageDataCalls = calls.filter((c) => c.method === "getImageData");
      expect(getImageDataCalls).toHaveLength(1);
      expect(getImageDataCalls[0].args).toEqual([0, 0, 24, 24]);
    } finally {
      restore();
    }
  });
});
