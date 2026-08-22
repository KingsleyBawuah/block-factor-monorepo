import { describe, it, expect } from "bun:test";
import { render, screen } from "@testing-library/react";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import App from "./App";

function createTestRouter() {
  const rootRoute = createRootRoute({ component: App });
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <div>Workspace / Split-Screen Location Intelligence</div>
    ),
  });
  const routeTree = rootRoute.addChildren([indexRoute]);
  const history = createMemoryHistory({ initialEntries: ["/"] });
  return createRouter({ routeTree, history });
}

describe("App component", () => {
  it("renders header and livability score card", async () => {
    const testRouter = createTestRouter();
    render(<RouterProvider router={testRouter} />);

    expect(await screen.findByText("BlockFactor")).toBeTruthy();
    expect(await screen.findByText("Livability Score")).toBeTruthy();
    expect(await screen.findByText("Status / Online")).toBeTruthy();
  });

  it("renders authentic CTA rail badges", async () => {
    const testRouter = createTestRouter();
    render(<RouterProvider router={testRouter} />);

    expect(await screen.findByText("Blue Line")).toBeTruthy();
    expect(await screen.findByText("Red Line")).toBeTruthy();
    expect(await screen.findByText("Pink Line")).toBeTruthy();
    expect(await screen.findByText("Green Line")).toBeTruthy();
  });

  it("renders outlet child route content", async () => {
    const testRouter = createTestRouter();
    render(<RouterProvider router={testRouter} />);

    expect(
      await screen.findByText("Workspace / Split-Screen Location Intelligence")
    ).toBeTruthy();
  });
});
