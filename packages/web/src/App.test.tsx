import { describe, it, expect } from "bun:test";
import { render, screen } from "@testing-library/react";
import App from "./App";

describe("App component", () => {
  it("renders header and livability score card", () => {
    render(<App />);

    expect(screen.getByText("BlockFactor")).toBeTruthy();
    expect(screen.getByText("Livability Score")).toBeTruthy();
    expect(screen.getByText("Status / Online")).toBeTruthy();
  });

  it("renders authentic CTA rail badges", () => {
    render(<App />);

    expect(screen.getByText("Blue Line")).toBeTruthy();
    expect(screen.getByText("Red Line")).toBeTruthy();
    expect(screen.getByText("Pink Line")).toBeTruthy();
    expect(screen.getByText("Green Line")).toBeTruthy();
  });
});
