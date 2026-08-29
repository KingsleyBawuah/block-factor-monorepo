import { describe, it, expect, mock } from "bun:test";
import { render, fireEvent } from "@testing-library/react";
import { WeightDrawer } from "./WeightDrawer";
import { type Weights, weightsForPreset } from "../lib/weights";

function renderDrawer(weights = weightsForPreset("remoteWorker")) {
  const onChange = mock((_weights: Weights) => {});
  const utils = render(<WeightDrawer weights={weights} onChange={onChange} />);
  return { ...utils, onChange };
}

describe("WeightDrawer", () => {
  it("renders collapsed by default, showing the current preset name but no chips or sliders", () => {
    const { queryByText, getByRole, getByText } = renderDrawer();
    expect(getByRole("button", { name: /weight customization/i })).toBeTruthy();
    // The header badge showing the active preset name is visible even
    // collapsed -- only the chips/sliders themselves are hidden.
    expect(getByText("Remote Worker")).toBeTruthy();
    expect(queryByText("Transit")).toBeNull();
  });

  it("expands to show preset chips and one slider per active factor on click", () => {
    const { getByRole, getByText, queryByText } = renderDrawer();
    fireEvent.click(getByRole("button", { name: /weight customization/i }));

    expect(getByRole("button", { name: "Remote Worker" })).toBeTruthy();
    expect(getByRole("button", { name: "Hybrid Commuter" })).toBeTruthy();
    expect(getByText("Transit")).toBeTruthy();
    expect(getByText("Solar Exposure")).toBeTruthy();
    // Commute never renders -- no destination-entry UI exists yet to ever
    // activate it (lib/weights.ts's WEIGHT_FACTOR_ORDER).
    expect(queryByText("Commute")).toBeNull();
  });

  it("shows the matching preset name in the collapsed header, or Custom once dragged away from both", () => {
    const { getByText, rerender } = renderDrawer(weightsForPreset("remoteWorker"));
    expect(getByText("Remote Worker")).toBeTruthy();

    rerender(<WeightDrawer weights={weightsForPreset("hybridCommuter")} onChange={() => {}} />);
    expect(getByText("Hybrid Commuter")).toBeTruthy();

    rerender(<WeightDrawer weights={{ ...weightsForPreset("remoteWorker"), transit: 40, bike: 0 }} onChange={() => {}} />);
    expect(getByText("Custom")).toBeTruthy();
  });

  it("calls onChange with the preset's weights when a preset chip is clicked", () => {
    const { getByRole, onChange } = renderDrawer();
    fireEvent.click(getByRole("button", { name: /weight customization/i }));
    fireEvent.click(getByRole("button", { name: "Hybrid Commuter" }));

    expect(onChange).toHaveBeenCalledWith(weightsForPreset("hybridCommuter"));
  });

  it("renders exactly 9 sliders, one per active factor (commute excluded)", () => {
    const { getByRole, getAllByRole } = renderDrawer();
    fireEvent.click(getByRole("button", { name: /weight customization/i }));

    expect(getAllByRole("slider")).toHaveLength(9);
  });

  it("collapses again on a second click of the header, hiding the sliders", () => {
    const { getByRole, queryByText } = renderDrawer();
    const toggle = getByRole("button", { name: /weight customization/i });

    fireEvent.click(toggle);
    expect(queryByText("Transit")).toBeTruthy();

    fireEvent.click(toggle);
    expect(queryByText("Transit")).toBeNull();
  });

  it("calls onChange with a redistributed weight set when a slider moves", () => {
    const { getByRole, onChange } = renderDrawer();
    fireEvent.click(getByRole("button", { name: /weight customization/i }));

    const transitSlider = getByRole("slider", { name: /transit/i });
    fireEvent.change(transitSlider, { target: { value: "35" } });

    expect(onChange).toHaveBeenCalledTimes(1);
    const result = onChange.mock.calls[0][0];
    expect(result.transit).toBe(35);
    const total = Object.values(result).reduce((sum, v) => sum + v, 0);
    expect(total).toBe(100);
  });
});
