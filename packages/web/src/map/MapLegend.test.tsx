import { describe, it, expect, mock } from "bun:test";
import { render, fireEvent } from "@testing-library/react";
import { MapLegend } from "./MapLegend";
import type { MapLayerKey } from "./layers/legend-entries";

const ALL_VISIBLE: Record<MapLayerKey, boolean> = {
  ctaRail: true,
  bikeTrail: true,
  poiDensity: true,
  divvyDocks: true,
};

function renderLegend(visibility = ALL_VISIBLE) {
  const onToggle = mock((_key: MapLayerKey) => {});
  const utils = render(<MapLegend visibility={visibility} onToggle={onToggle} />);
  return { ...utils, onToggle };
}

describe("MapLegend", () => {
  it("renders collapsed by default, showing no layer entries", () => {
    const { queryByText, getByRole } = renderLegend();
    expect(getByRole("button", { name: /layers/i })).toBeTruthy();
    expect(queryByText("CTA Rail")).toBeNull();
  });

  it("expands to show one entry per layer on click", () => {
    const { getByRole, getByText } = renderLegend();
    fireEvent.click(getByRole("button", { name: /layers/i }));

    expect(getByText("CTA Rail")).toBeTruthy();
    expect(getByText("Bike / Trail")).toBeTruthy();
    expect(getByText("Cafes / Fitness / Groceries")).toBeTruthy();
    expect(getByText("Divvy Docks")).toBeTruthy();
  });

  it("checks a layer's checkbox according to the current visibility prop", () => {
    const { getByRole } = renderLegend({ ...ALL_VISIBLE, divvyDocks: false });
    fireEvent.click(getByRole("button", { name: /layers/i }));

    expect((getByRole("checkbox", { name: "Divvy Docks" }) as HTMLInputElement).checked).toBe(false);
    expect((getByRole("checkbox", { name: "CTA Rail" }) as HTMLInputElement).checked).toBe(true);
  });

  it("calls onToggle with the layer's key when its checkbox is clicked", () => {
    const { getByRole, onToggle } = renderLegend();
    fireEvent.click(getByRole("button", { name: /layers/i }));

    fireEvent.click(getByRole("checkbox", { name: "Bike / Trail" }));

    expect(onToggle).toHaveBeenCalledWith("bikeTrail");
  });

  it("shows the bike lane protection mini-key (Protected / Shared lane) under the Bike / Trail row", () => {
    const { getByRole, getByText } = renderLegend();
    fireEvent.click(getByRole("button", { name: /layers/i }));

    expect(getByText("Protected")).toBeTruthy();
    expect(getByText("Shared lane")).toBeTruthy();
  });

  it("renders Divvy Docks' swatch as a split blue/grey circle, matching its real two-tone marker", () => {
    const { getByRole, container } = renderLegend();
    fireEvent.click(getByRole("button", { name: /layers/i }));

    const divvyRow = getByRole("checkbox", { name: "Divvy Docks" }).closest("div");
    const swatch = divvyRow?.querySelector("span.rounded-full") as HTMLElement | null;
    expect(swatch?.style.background).toContain("linear-gradient");
    expect(swatch?.style.background).toContain("#41b6e6");
    expect(swatch?.style.background).toContain("#a1a1aa");
    // Sanity: this is the one swatch in the whole panel that's a single
    // split circle rather than a square or a small dot cluster.
    expect(container.querySelectorAll("span[style*='linear-gradient']")).toHaveLength(1);
  });

  it("collapses again on a second click of the header", () => {
    const { getByRole, queryByText } = renderLegend();
    const toggle = getByRole("button", { name: /layers/i });

    fireEvent.click(toggle);
    expect(queryByText("CTA Rail")).toBeTruthy();

    fireEvent.click(toggle);
    expect(queryByText("CTA Rail")).toBeNull();
  });
});
