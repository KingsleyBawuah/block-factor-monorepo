import { describe, it, expect } from "bun:test";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import App from "./App";

// maplibre-gl is mocked globally in src/test/setup.ts (the bun:test
// preload) — happy-dom has no WebGL context, so the real Map constructor
// (invoked by MapCanvas, rendered inside App) would throw.

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

// App renders map layer components (e.g. CtaRailLayer) that call useQuery,
// which throws without a QueryClientProvider ancestor -- unlike production
// (main.tsx), retry is disabled here: happy-dom's fetch has nothing real to
// reach, and TanStack Query's default 3-attempt exponential backoff would
// otherwise add real delay to every test that renders App.
function renderApp() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const testRouter = createTestRouter();
  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={testRouter} />
    </QueryClientProvider>
  );
}

describe("App component", () => {
  it("renders header and livability score card", async () => {
    renderApp();

    expect(await screen.findByText("BlockFactor")).toBeTruthy();
    expect(await screen.findByText("Livability Score")).toBeTruthy();
    expect(await screen.findByText("Status / Online")).toBeTruthy();
  });

  it("renders authentic CTA rail badges", async () => {
    renderApp();

    expect(await screen.findByText("Blue Line")).toBeTruthy();
    expect(await screen.findByText("Red Line")).toBeTruthy();
    expect(await screen.findByText("Pink Line")).toBeTruthy();
    expect(await screen.findByText("Green Line")).toBeTruthy();
  });

  it("renders the solar exposure badge for the demo address's facing and score", async () => {
    renderApp();

    // 233 S Wacker Dr infers West-facing (verified against
    // InferStreetFacingOrientation, see map/constants.ts), which is a 75/100
    // S_solar base score -- shown here as a plain sidebar badge rather than
    // requiring a viewer to interpret the 3D ray's color/height on the map.
    expect(await screen.findByText("W-Facing · 75")).toBeTruthy();
  });

  it("renders outlet child route content", async () => {
    renderApp();

    expect(
      await screen.findByText("Workspace / Split-Screen Location Intelligence")
    ).toBeTruthy();
  });
});
