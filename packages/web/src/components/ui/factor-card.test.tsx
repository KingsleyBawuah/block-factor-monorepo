import { describe, it, expect } from "bun:test";
import { render, fireEvent } from "@testing-library/react";
import { FactorCard } from "./factor-card";

describe("FactorCard", () => {
  it("renders the rounded score and a tier-colored progress bar", () => {
    const { getByText, getByRole } = render(
      <FactorCard label="Transit" icon="lni-train-1" factor={{ score: 81.6, degraded: false }} />
    );
    expect(getByText("82")).toBeTruthy();
    const bar = getByRole("progressbar", { name: "Transit sub-score" });
    expect(bar.getAttribute("aria-valuenow")).toBe("82");
  });

  it.each([
    [90, "bg-emerald-400"],
    [75, "bg-lime-400"],
    [55, "bg-amber-400"],
    [20, "bg-rose-400"],
  ])("colors the progress bar fill %s for score %s, matching ScoreRing's own tier colors", (score, expectedClass) => {
    const { getByRole } = render(
      <FactorCard label="Transit" icon="lni-train-1" factor={{ score, degraded: false }} />
    );
    const bar = getByRole("progressbar", { name: "Transit sub-score" });
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.className).toContain(expectedClass);
    expect(fill.style.width).toBe(`${score}%`);
  });

  it("clamps an out-of-range score before using it as aria-valuenow and the bar width", () => {
    const { getByRole } = render(
      <FactorCard label="Transit" icon="lni-train-1" factor={{ score: 142, degraded: false }} />
    );
    const bar = getByRole("progressbar", { name: "Transit sub-score" });
    expect(bar.getAttribute("aria-valuenow")).toBe("100");
    expect((bar.firstElementChild as HTMLElement).style.width).toBe("100%");
  });

  it("shows an Unavailable badge and no progress bar when degraded", () => {
    const { getByText, queryByRole } = render(
      <FactorCard label="Noise Environment" icon="lni-volume-high" factor={{ score: null, degraded: true }} />
    );
    expect(getByText("Unavailable")).toBeTruthy();
    expect(queryByRole("progressbar")).toBeNull();
  });

  it("renders a straight-line-qualified distance pill in miles when distanceMeters is set", () => {
    const { getByText } = render(
      <FactorCard
        label="Groceries"
        icon="lni-basket-shopping-3"
        factor={{ score: 90, degraded: false, distanceMeters: 450.6 }}
      />
    );
    expect(getByText("0.28 mi")).toBeTruthy();
    expect(getByText("straight-line")).toBeTruthy();
  });

  it("omits the distance pill when distanceMeters is absent", () => {
    const { queryByText } = render(
      <FactorCard label="Solar Exposure" icon="lni-sun-1" factor={{ score: 75, degraded: false }} />
    );
    expect(queryByText("straight-line")).toBeNull();
  });

  it("expands to reveal contributing POIs on click only when POIs exist", () => {
    const { getByRole, getByText, queryByText } = render(
      <FactorCard
        label="Third Places"
        icon="lni-coffee-cup-2"
        factor={{
          score: 70,
          degraded: false,
          contributingPois: [{ name: "Sip Coffee House", distanceMeters: 200 }],
        }}
      />
    );
    expect(queryByText("Sip Coffee House")).toBeNull();
    fireEvent.click(getByRole("button"));
    expect(getByText("Sip Coffee House")).toBeTruthy();
  });

  it("renders a plain, non-interactive row (not a disabled button) when there are no contributing POIs", () => {
    const { queryByRole, getByText } = render(
      <FactorCard label="Flood Risk" icon="lni-water-drop-1" factor={{ score: 60, degraded: false }} />
    );
    // A disabled <button> would drop out of the tab order and read as
    // "disabled" to screen readers, implying a broken factor rather than
    // simply non-expandable content -- so no button role should exist at all.
    expect(queryByRole("button")).toBeNull();
    expect(getByText("Flood Risk")).toBeTruthy();
  });

  it("treats an empty contributingPois array the same as no POIs at all", () => {
    const { queryByRole } = render(
      <FactorCard label="Green Space" icon="lni-tree-2" factor={{ score: 60, degraded: false, contributingPois: [] }} />
    );
    expect(queryByRole("button")).toBeNull();
  });

  it("renders every contributing POI even when two share the same name and distance", () => {
    const { getAllByText, getByRole } = render(
      <FactorCard
        label="Groceries"
        icon="lni-basket-shopping-3"
        factor={{
          score: 80,
          degraded: false,
          contributingPois: [
            { name: "Walgreens", distanceMeters: 200 },
            { name: "Walgreens", distanceMeters: 200 },
          ],
        }}
      />
    );
    fireEvent.click(getByRole("button"));
    expect(getAllByText("Walgreens")).toHaveLength(2);
  });

  it("renders one colored dot per line next to a station's name", () => {
    const { getByRole, getByText } = render(
      <FactorCard
        label="Transit"
        icon="lni-train-1"
        factor={{
          score: 100,
          degraded: false,
          contributingPois: [{ name: "Fullerton ('L' Station)", distanceMeters: 250, lines: ["Red", "Brown", "Purple"] }],
        }}
      />
    );
    fireEvent.click(getByRole("button"));

    const nameRow = getByText("Fullerton ('L' Station)").parentElement;
    if (!nameRow) throw new Error("expected the station name to have a parent element");
    const dots = nameRow.querySelectorAll('span[aria-hidden="true"]');
    expect(dots).toHaveLength(3);
    expect((dots[0] as HTMLElement).style.backgroundColor).toBe("#c60c30"); // Red
    expect((dots[1] as HTMLElement).style.backgroundColor).toBe("#62361b"); // Brown
    expect((dots[2] as HTMLElement).style.backgroundColor).toBe("#522398"); // Purple
  });

  it("labels the dot cluster with the real line names for screen readers, since the dots alone are color-only", () => {
    const { getByRole, getByLabelText } = render(
      <FactorCard
        label="Transit"
        icon="lni-train-1"
        factor={{
          score: 100,
          degraded: false,
          contributingPois: [{ name: "Fullerton ('L' Station)", distanceMeters: 250, lines: ["Red", "Brown"] }],
        }}
      />
    );
    fireEvent.click(getByRole("button"));

    expect(getByLabelText("Red, Brown lines")).toBeTruthy();
  });

  it("falls back to the muted line color for an unrecognized line name rather than omitting the dot", () => {
    const { getByRole, getByText } = render(
      <FactorCard
        label="Transit"
        icon="lni-train-1"
        factor={{
          score: 100,
          degraded: false,
          contributingPois: [{ name: "Future Stop", distanceMeters: 250, lines: ["Silver"] }],
        }}
      />
    );
    fireEvent.click(getByRole("button"));

    const dot = getByText("Future Stop").parentElement?.querySelector('span[aria-hidden="true"]') as HTMLElement;
    expect(dot.style.backgroundColor).toBe("#71717a"); // the shared unrecognized-line fallback
  });

  it("renders no line dots for a contributing POI without lines (e.g. a bus corridor)", () => {
    const { getByRole, getByText } = render(
      <FactorCard
        label="Transit"
        icon="lni-train-1"
        factor={{
          score: 100,
          degraded: false,
          contributingPois: [{ name: "Route 156 - LaSalle (Bus)", distanceMeters: 80 }],
        }}
      />
    );
    fireEvent.click(getByRole("button"));

    const nameRow = getByText("Route 156 - LaSalle (Bus)").parentElement;
    expect(nameRow?.querySelector('span[aria-hidden="true"]')).toBeNull();
  });
});
