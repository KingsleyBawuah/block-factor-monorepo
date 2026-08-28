import { describe, it, expect } from "bun:test";
import { render } from "@testing-library/react";
import { FactorBreakdownList } from "./FactorBreakdownList";
import type { components } from "../lib/api/schema";

type ScoreFactors = components["schemas"]["ScoreResponse"]["factors"];

const BASE_FACTORS: ScoreFactors = {
  transit: { score: 80, degraded: false },
  bike: { score: 70, degraded: false },
  grocery: { score: 60, degraded: false },
  thirdPlace: { score: 50, degraded: false },
  green: { score: 40, degraded: false },
  noise: { score: 30, degraded: false },
  safety: { score: 20, degraded: false },
  flood: { score: 10, degraded: false },
  solar: { score: 90, degraded: false },
};

describe("FactorBreakdownList", () => {
  it("renders all 9 base factor cards, omitting commute when absent", () => {
    const { getByText, queryByText } = render(<FactorBreakdownList factors={BASE_FACTORS} />);
    expect(getByText("Transit")).toBeTruthy();
    expect(getByText("Solar Exposure")).toBeTruthy();
    expect(queryByText("Commute")).toBeNull();
  });

  it("renders the 10th commute card only when the API included it", () => {
    const { getByText } = render(
      <FactorBreakdownList
        factors={{ ...BASE_FACTORS, commute: { score: 55, degraded: false } }}
      />
    );
    expect(getByText("Commute")).toBeTruthy();
  });

  it("renders factors in the fixed PRD weighting-table order", () => {
    const { container } = render(
      <FactorBreakdownList
        factors={{ ...BASE_FACTORS, commute: { score: 55, degraded: false } }}
      />
    );
    const labels = Array.from(container.querySelectorAll(".uppercase")).map((el) => el.textContent);
    expect(labels).toEqual([
      "Transit",
      "Commute",
      "Bike Infrastructure",
      "Groceries",
      "Third Places",
      "Green Space",
      "Noise Environment",
      "Public Safety",
      "Flood Risk",
      "Solar Exposure",
    ]);
  });
});
