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

  it("does not render an expand toggle when there are no contributing POIs", () => {
    const { getByRole } = render(
      <FactorCard label="Flood Risk" icon="lni-water-drop-1" factor={{ score: 60, degraded: false }} />
    );
    expect(getByRole("button").hasAttribute("aria-expanded")).toBe(false);
  });
});
