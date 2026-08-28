import { describe, it, expect } from "bun:test";
import { render } from "@testing-library/react";
import { RING_CIRCUMFERENCE, ringDashOffset, ScoreRing } from "./score-ring";

describe("ringDashOffset", () => {
  it("returns 0 (fully drawn ring) at score 100", () => {
    expect(ringDashOffset(100)).toBe(0);
  });

  it("returns the full circumference (empty ring) at score 0", () => {
    expect(ringDashOffset(0)).toBeCloseTo(RING_CIRCUMFERENCE, 5);
  });

  it("returns half the circumference at score 50", () => {
    expect(ringDashOffset(50)).toBeCloseTo(RING_CIRCUMFERENCE / 2, 5);
  });

  it("clamps a score above 100 to a fully drawn ring", () => {
    expect(ringDashOffset(150)).toBe(0);
  });

  it("clamps a negative score to a fully empty ring", () => {
    expect(ringDashOffset(-10)).toBeCloseTo(RING_CIRCUMFERENCE, 5);
  });
});

describe("ScoreRing", () => {
  it("rounds the displayed score and exposes it via an accessible label", () => {
    const { getByRole, getByText } = render(<ScoreRing score={81.6} />);
    expect(getByText("82")).toBeTruthy();
    expect(getByRole("img", { name: "Composite BlockFactor score: 82 out of 100" })).toBeTruthy();
  });

  it("colors the progress arc by score tier, matching Badge's own score-* colors", () => {
    const { container } = render(<ScoreRing score={90} />);
    const circles = container.querySelectorAll("circle");
    // First circle is the static background track; second is the progress arc.
    expect(circles[1].getAttribute("class")).toContain("stroke-emerald-400");
  });

  it("uses the rose tier for a low score", () => {
    const { container } = render(<ScoreRing score={20} />);
    const circles = container.querySelectorAll("circle");
    expect(circles[1].getAttribute("class")).toContain("stroke-rose-400");
  });

  it("sets stroke-dashoffset on the progress arc matching ringDashOffset", () => {
    const { container } = render(<ScoreRing score={65} />);
    const circles = container.querySelectorAll("circle");
    expect(Number(circles[1].getAttribute("stroke-dashoffset"))).toBeCloseTo(ringDashOffset(65), 5);
  });
});
