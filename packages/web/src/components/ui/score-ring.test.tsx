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

function progressArc(container: HTMLElement) {
  const circles = container.querySelectorAll("circle");
  // First circle is the static background track; second is the progress arc.
  return circles[1];
}

describe("ScoreRing", () => {
  it("rounds the displayed score up and exposes it via an accessible label", () => {
    const { getByRole, getByText } = render(<ScoreRing score={81.6} />);
    expect(getByText("82")).toBeTruthy();
    expect(getByRole("img", { name: "Composite BlockFactor score: 82 out of 100" })).toBeTruthy();
  });

  it("rounds the displayed score down", () => {
    const { getByRole, getByText } = render(<ScoreRing score={81.4} />);
    expect(getByText("81")).toBeTruthy();
    expect(getByRole("img", { name: "Composite BlockFactor score: 81 out of 100" })).toBeTruthy();
  });

  it.each([
    [90, "stroke-emerald-400"],
    [75, "stroke-lime-400"],
    [55, "stroke-amber-400"],
    [20, "stroke-rose-400"],
  ])("colors the progress arc %s for score %s, matching Badge's own score-* colors", (score, expectedClass) => {
    const { container } = render(<ScoreRing score={score} />);
    expect(progressArc(container).getAttribute("class")).toContain(expectedClass);
  });

  it("colors the ring by the rounded score's tier, not the raw score's -- 79.6 rounds to 80 (emerald), not lime", () => {
    const { container, getByText } = render(<ScoreRing score={79.6} />);
    expect(getByText("80")).toBeTruthy();
    expect(progressArc(container).getAttribute("class")).toContain("stroke-emerald-400");
  });

  it("sets stroke-dashoffset on the progress arc matching ringDashOffset", () => {
    const { container } = render(<ScoreRing score={65} />);
    expect(Number(progressArc(container).getAttribute("stroke-dashoffset"))).toBeCloseTo(ringDashOffset(65), 5);
  });

  it("renders a fully empty ring at score 0 with a butt line cap (avoids a stray round-cap dot on a zero-length arc)", () => {
    const { container, getByText } = render(<ScoreRing score={0} />);
    expect(getByText("0")).toBeTruthy();
    const arc = progressArc(container);
    expect(Number(arc.getAttribute("stroke-dashoffset"))).toBeCloseTo(RING_CIRCUMFERENCE, 5);
    expect(arc.getAttribute("stroke-linecap")).toBe("butt");
  });

  it("renders a fully drawn ring at score 100 with a round line cap", () => {
    const { container, getByText } = render(<ScoreRing score={100} />);
    expect(getByText("100")).toBeTruthy();
    const arc = progressArc(container);
    expect(Number(arc.getAttribute("stroke-dashoffset"))).toBe(0);
    expect(arc.getAttribute("stroke-linecap")).toBe("round");
  });
});
