import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { validateWeightSearch } from "./lib/weight-search-params";

// maplibre-gl is mocked globally in src/test/setup.ts (the bun:test
// preload) — happy-dom has no WebGL context, so the real Map constructor
// (invoked by MapCanvas, rendered inside App) would throw.

// Every map layer (CtaRailLayer, ProtectedBikeLayer, DivvyDockLayer, and
// PoiDensityLayer once an address resolves) fires its own GET on mount --
// this catch-all keeps them all satisfied with a harmless empty
// FeatureCollection so only the paths this file actually cares about
// (geocode/score) need specific handling.
const getMock = mock(async (path: string, options?: { params?: { query?: Record<string, unknown> } }) => {
  if (path === "/api/v1/geocode") {
    const q = options?.params?.query?.q as string;
    if (q === "233 S Wacker Dr") {
      return {
        data: {
          candidates: [
            { formattedAddress: "233 S Wacker Dr, Chicago, IL", coordinates: { lat: 41.8781, lon: -87.6298 }, rank: 0 },
          ],
        },
        error: undefined,
      };
    }
  }
  return { data: { type: "FeatureCollection", features: [] }, error: undefined };
});

type MockFactor = { degraded: boolean; score: number | null };
type MockScoreData = {
  compositeScore: number;
  factors: Record<
    "transit" | "bike" | "grocery" | "thirdPlace" | "green" | "noise" | "safety" | "flood" | "solar",
    MockFactor
  >;
  benchmarkCommutes: never[];
};

type ScoreRequestOptions = {
  body?: { origin?: { lat: number; lon: number }; addressHint?: string; weights?: Record<string, number> };
};

const postMock = mock(
  async (path: string, _options?: ScoreRequestOptions): Promise<{ data?: MockScoreData; error?: unknown }> => {
    if (path === "/api/v1/score") {
      return {
        data: {
          compositeScore: 81.1,
          factors: {
            transit: { degraded: false, score: 90 },
            bike: { degraded: false, score: 80 },
            grocery: { degraded: false, score: 70 },
            thirdPlace: { degraded: false, score: 60 },
            green: { degraded: false, score: 75 },
            noise: { degraded: false, score: 65 },
            safety: { degraded: false, score: 85 },
            flood: { degraded: false, score: 95 },
            solar: { degraded: false, score: 75 },
          },
          benchmarkCommutes: [],
        },
        error: undefined,
      };
    }
    throw new Error(`unexpected POST path: ${path}`);
  }
);

mock.module("./lib/api/client", () => ({
  apiClient: { GET: getMock, POST: postMock },
}));

const { default: App } = await import("./App");

function createTestRouter() {
  // validateSearch mirrors the real app's root route (routes/index.tsx) --
  // without it, App's useSearch({ strict: false }) reads an unvalidated
  // empty object under test, and every weight comes back undefined.
  const rootRoute = createRootRoute({ component: App, validateSearch: validateWeightSearch });
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
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const testRouter = createTestRouter();
  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={testRouter} />
    </QueryClientProvider>
  );
}

async function searchFor233Wacker() {
  const input = await screen.findByLabelText("Address");
  fireEvent.change(input, { target: { value: "233 S Wacker Dr" } });
  fireEvent.click(screen.getByRole("button", { name: /search/i }));
}

describe("App component", () => {
  beforeEach(() => {
    getMock.mockClear();
    postMock.mockClear();
  });

  it("renders header and livability score card", async () => {
    renderApp();

    expect(await screen.findByText("Livability Score")).toBeTruthy();
    expect(await screen.findByText("Status / Online")).toBeTruthy();
  });

  it("renders the BlockFactor wordmark split into Chicago flag blue/red halves", async () => {
    renderApp();

    const block = await screen.findByText("Block");
    const factor = await screen.findByText("Factor");
    expect(block.className).toContain("text-flag-blue");
    expect(factor.className).toContain("text-flag-red");
  });

  it("renders authentic CTA rail badges", async () => {
    renderApp();

    expect(await screen.findByText("Blue Line")).toBeTruthy();
    expect(await screen.findByText("Red Line")).toBeTruthy();
    expect(await screen.findByText("Pink Line")).toBeTruthy();
    expect(await screen.findByText("Green Line")).toBeTruthy();
  });

  it("prompts to search before any address has resolved, with no factor breakdown list", async () => {
    renderApp();

    expect(
      await screen.findByText("Search an address above to see its BlockFactor score.")
    ).toBeTruthy();
    expect(screen.queryByText("Solar Exposure")).toBeNull();
  });

  it("updates the score card and factor breakdown once an address resolves", async () => {
    renderApp();

    await searchFor233Wacker();

    expect(await screen.findByText("233 S Wacker Dr, Chicago, IL")).toBeTruthy();
    expect(await screen.findByText("81")).toBeTruthy(); // Math.round(81.1)
    // 75 falls in the "lime" score tier (65-79) -- asserts the Solar
    // Exposure card's progress bar fill carries that tier's color, not just
    // that the number rendered somewhere.
    // Solar has no contributingPois in this fixture, so its card header
    // renders as a plain <div> rather than a <button> -- scope by the card's
    // own rounded-lg wrapper instead, which is stable either way.
    const solarLabel = await screen.findByText("Solar Exposure");
    const solarRow = solarLabel.closest("div.rounded-lg") as HTMLElement;
    expect(within(solarRow).getByText("75")).toBeTruthy();
    expect(solarRow.querySelector(".bg-lime-400")).toBeTruthy();
    expect(screen.queryByText("Search an address above to see its BlockFactor score.")).toBeNull();
  });

  it("shows an Unavailable badge instead of a score when the solar factor is degraded", async () => {
    postMock.mockImplementationOnce(async () => ({
      data: {
        compositeScore: 70,
        factors: {
          transit: { degraded: false, score: 90 },
          bike: { degraded: false, score: 80 },
          grocery: { degraded: false, score: 70 },
          thirdPlace: { degraded: false, score: 60 },
          green: { degraded: false, score: 75 },
          noise: { degraded: false, score: 65 },
          safety: { degraded: false, score: 85 },
          flood: { degraded: false, score: 95 },
          solar: { degraded: true, score: null },
        },
        benchmarkCommutes: [],
      },
      error: undefined,
    }));
    renderApp();

    await searchFor233Wacker();

    expect(await screen.findByText("Unavailable")).toBeTruthy();
  });

  it("resets back to the search prompt when a new search starts", async () => {
    renderApp();

    await searchFor233Wacker();
    await screen.findByText("233 S Wacker Dr, Chicago, IL");

    fireEvent.change(screen.getByLabelText("Address"), { target: { value: "another address" } });
    fireEvent.click(screen.getByRole("button", { name: /search/i }));

    await waitFor(() =>
      expect(screen.queryByText("Search an address above to see its BlockFactor score.")).toBeTruthy()
    );
    expect(screen.queryByText("233 S Wacker Dr, Chicago, IL")).toBeNull();
  });

  it("re-scores the same address (no re-geocode) once a dragged weight slider settles, debounced", async () => {
    renderApp();

    await searchFor233Wacker();
    await screen.findByText("233 S Wacker Dr, Chicago, IL");

    const scoreCallsBefore = postMock.mock.calls.filter((c) => c[0] === "/api/v1/score").length;
    const geocodeCallsBefore = getMock.mock.calls.filter((c) => c[0] === "/api/v1/geocode").length;

    fireEvent.click(screen.getByRole("button", { name: /weight customization/i }));
    fireEvent.change(screen.getByRole("slider", { name: /transit/i }), { target: { value: "40" } });

    // App.tsx debounces the actual re-score request (RESCORE_DEBOUNCE_MS)
    // -- it shouldn't fire on every drag tick.
    expect(postMock.mock.calls.filter((c) => c[0] === "/api/v1/score").length).toBe(scoreCallsBefore);

    await waitFor(
      () => expect(postMock.mock.calls.filter((c) => c[0] === "/api/v1/score").length).toBe(scoreCallsBefore + 1),
      { timeout: 1000 }
    );

    const rescoreCall = postMock.mock.calls.filter((c) => c[0] === "/api/v1/score").at(-1);
    expect(rescoreCall?.[1]?.body?.origin).toEqual({ lat: 41.8781, lon: -87.6298 });
    expect(rescoreCall?.[1]?.body?.weights?.transit).toBe(40);

    // Re-scores the already-resolved coordinates directly -- no redundant geocode.
    expect(getMock.mock.calls.filter((c) => c[0] === "/api/v1/geocode").length).toBe(geocodeCallsBefore);
  });

  it("renders outlet child route content", async () => {
    renderApp();

    expect(
      await screen.findByText("Workspace / Split-Screen Location Intelligence")
    ).toBeTruthy();
  });
});
