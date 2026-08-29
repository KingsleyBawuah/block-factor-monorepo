import { describe, it, expect } from "bun:test";
import { ctaLineBadgeVariant, type BadgeProps } from "./badge";

describe("ctaLineBadgeVariant", () => {
  const cases: [string, BadgeProps["variant"]][] = [
    ["Red", "cta-red"],
    ["Blue", "cta-blue"],
    ["Brown", "cta-brown"],
    ["Green", "cta-green"],
    ["Pink", "cta-pink"],
    ["Orange", "cta-orange"],
    ["Purple", "cta-purple"],
    ["Yellow", "cta-yellow"],
  ];

  it.each(cases)("maps %s to %s", (line, expectedVariant) => {
    expect(ctaLineBadgeVariant(line)).toBe(expectedVariant);
  });

  it("is case-insensitive", () => {
    expect(ctaLineBadgeVariant("red")).toBe("cta-red");
    expect(ctaLineBadgeVariant("RED")).toBe("cta-red");
  });

  it("falls back to outline for an unrecognized line name", () => {
    expect(ctaLineBadgeVariant("Silver")).toBe("outline");
    expect(ctaLineBadgeVariant("")).toBe("outline");
  });
});
