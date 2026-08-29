import { describe, it, expect, mock } from "bun:test";
import { render, fireEvent } from "@testing-library/react";
import { ComparisonTray } from "./ComparisonTray";
import type { PinnedAddress } from "../lib/pinned-addresses";

function fakePin(overrides: Partial<PinnedAddress> = {}): PinnedAddress {
  return {
    formattedAddress: "233 S Wacker Dr, Chicago, IL",
    lat: 41.8788,
    lon: -87.6359,
    score: {
      compositeScore: 81.6,
      factors: {
        transit: { score: 92, degraded: false },
        grocery: { score: 60, degraded: false },
        solar: { score: null, degraded: true },
      },
    } as PinnedAddress["score"],
    ...overrides,
  };
}

describe("ComparisonTray", () => {
  it("renders nothing when no addresses are pinned", () => {
    const { container } = render(<ComparisonTray pinned={[]} onUnpin={mock(() => {})} />);
    expect(container.firstChild).toBeNull();
  });

  it("shows the pin count and each pinned address's rounded composite score", () => {
    const { getByText } = render(
      <ComparisonTray pinned={[fakePin()]} onUnpin={mock(() => {})} />
    );
    expect(getByText("Comparing 1/3")).toBeTruthy();
    expect(getByText("233 S Wacker Dr, Chicago, IL")).toBeTruthy();
    expect(getByText("82")).toBeTruthy();
  });

  it("calls onUnpin with the clicked address's index", () => {
    const onUnpin = mock((_index: number) => {});
    const { getByLabelText } = render(
      <ComparisonTray
        pinned={[fakePin({ lat: 1, lon: 1 }), fakePin({ lat: 2, lon: 2, formattedAddress: "875 N Michigan Ave" })]}
        onUnpin={onUnpin}
      />
    );

    fireEvent.click(getByLabelText("Unpin 875 N Michigan Ave"));

    expect(onUnpin).toHaveBeenCalledWith(1);
  });

  it("does not show a Compare toggle with only one address pinned", () => {
    const { queryByRole } = render(<ComparisonTray pinned={[fakePin()]} onUnpin={mock(() => {})} />);
    expect(queryByRole("button", { name: /compare/i })).toBeNull();
  });

  it("expands to a factor-by-factor comparison table on Compare, and collapses again on a second click", () => {
    const { getByRole, getByText, getAllByText, queryByText } = render(
      <ComparisonTray
        pinned={[fakePin({ lat: 1, lon: 1 }), fakePin({ lat: 2, lon: 2, formattedAddress: "875 N Michigan Ave" })]}
        onUnpin={mock(() => {})}
      />
    );

    expect(queryByText("Transit")).toBeNull();
    fireEvent.click(getByRole("button", { name: /compare/i }));

    expect(getByText("Transit")).toBeTruthy();
    expect(getByText("Groceries")).toBeTruthy();
    // Both pinned fixtures share the same transit/grocery scores (92/60) --
    // asserts the actual cell values and their tier-color classes, not just
    // that the factor labels rendered somewhere in the table.
    const transitCells = getAllByText("92");
    expect(transitCells).toHaveLength(2);
    for (const cell of transitCells) {
      expect(cell.className).toContain("text-emerald-400");
    }
    const groceryCells = getAllByText("60");
    expect(groceryCells).toHaveLength(2);
    for (const cell of groceryCells) {
      expect(cell.className).toContain("text-amber-400");
    }

    fireEvent.click(getByRole("button", { name: /hide comparison/i }));
    expect(queryByText("Transit")).toBeNull();
  });

  it("shows Unavailable for a degraded or missing factor in the comparison table, not a blank or NaN cell", () => {
    const { getByRole, getAllByText } = render(
      <ComparisonTray
        pinned={[fakePin({ lat: 1, lon: 1 }), fakePin({ lat: 2, lon: 2 })]}
        onUnpin={mock(() => {})}
      />
    );

    fireEvent.click(getByRole("button", { name: /compare/i }));

    // Both pinned fixtures share the same degraded solar factor.
    expect(getAllByText("Unavailable")).toHaveLength(2);
  });

  it("omits a factor row entirely when none of the pinned addresses scored it", () => {
    const { getByRole, queryByText } = render(
      <ComparisonTray
        pinned={[
          fakePin({ lat: 1, lon: 1, score: { compositeScore: 81, factors: { transit: { score: 90, degraded: false } } } as PinnedAddress["score"] }),
          fakePin({ lat: 2, lon: 2, score: { compositeScore: 70, factors: { transit: { score: 50, degraded: false } } } as PinnedAddress["score"] }),
        ]}
        onUnpin={mock(() => {})}
      />
    );

    fireEvent.click(getByRole("button", { name: /compare/i }));

    expect(queryByText("Groceries")).toBeNull();
  });
});
