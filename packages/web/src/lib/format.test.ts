import { describe, it, expect } from "bun:test";
import { formatDistanceMiles } from "./format";

describe("formatDistanceMiles", () => {
  it("formats 0 meters as 0.00 mi", () => {
    expect(formatDistanceMiles(0)).toBe("0.00 mi");
  });

  it("formats exactly one mile (1609.344 m) as 1.00 mi", () => {
    expect(formatDistanceMiles(1609.344)).toBe("1.00 mi");
  });

  it("formats a sub-mile distance rounded to 2 decimal places", () => {
    expect(formatDistanceMiles(450.6)).toBe("0.28 mi");
  });

  it("formats a multi-mile distance", () => {
    expect(formatDistanceMiles(8046.72)).toBe("5.00 mi");
  });
});
