import { describe, it, expect, mock, beforeEach } from "bun:test";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const getMock = mock(async (path: string, options?: { params?: { query?: Record<string, unknown> } }) => {
  if (path === "/api/v1/geocode") {
    const q = options?.params?.query?.q as string;
    if (q === "nowhere") return { data: { candidates: [] }, error: undefined };
    if (q === "network down") throw new Error("network down");
    if (q === "ambiguous street") {
      return {
        data: {
          candidates: [
            { formattedAddress: "100 Ambiguous St, Chicago, IL", coordinates: { lat: 41.9, lon: -87.65 }, rank: 0 },
            { formattedAddress: "200 Ambiguous St, Chicago, IL", coordinates: { lat: 41.91, lon: -87.66 }, rank: 1 },
          ],
        },
        error: undefined,
      };
    }
    if (q === "out of coverage") {
      return {
        data: { candidates: [{ formattedAddress: "1 Rural Rd, Downstate IL", coordinates: { lat: 39, lon: -89 }, rank: 0 }] },
        error: undefined,
      };
    }
    if (q === "score error") {
      return {
        data: { candidates: [{ formattedAddress: "500 Broken Ave, Chicago, IL", coordinates: { lat: 41.85, lon: -87.6 }, rank: 0 }] },
        error: undefined,
      };
    }
    return {
      data: {
        candidates: [
          { formattedAddress: "233 S Wacker Dr, Chicago, IL", coordinates: { lat: 41.8781, lon: -87.6298 }, rank: 0 },
        ],
      },
      error: undefined,
    };
  }
  throw new Error(`unexpected GET path: ${path}`);
});

const postMock = mock(async (path: string, options?: { body?: { address?: string } }) => {
  if (path === "/api/v1/score") {
    const address = options?.body?.address;
    if (address === "1 Rural Rd, Downstate IL") {
      return { data: undefined, error: { code: "OUT_OF_COVERAGE", message: "outside Chicago" } };
    }
    if (address === "500 Broken Ave, Chicago, IL") throw { code: "SERVER_ERROR", message: "boom" };
    return {
      data: {
        compositeScore: 82,
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
});

mock.module("../lib/api/client", () => ({
  apiClient: { GET: getMock, POST: postMock },
}));

const { AddressSearchInput } = await import("./AddressSearchInput");

function renderInput() {
  const onResolved = mock(() => {});
  const onReset = mock(() => {});
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <AddressSearchInput onResolved={onResolved} onReset={onReset} />
    </QueryClientProvider>
  );
  return { onResolved, onReset };
}

function search(query: string) {
  fireEvent.change(screen.getByLabelText("Address"), { target: { value: query } });
  fireEvent.click(screen.getByRole("button", { name: /search/i }));
}

describe("AddressSearchInput", () => {
  beforeEach(() => {
    getMock.mockClear();
    postMock.mockClear();
  });

  it("auto-selects a single geocode candidate and resolves it via POST /api/v1/score", async () => {
    const { onResolved } = renderInput();

    search("233 S Wacker Dr");

    await waitFor(() => expect(onResolved).toHaveBeenCalledTimes(1));
    expect(onResolved).toHaveBeenCalledWith({
      formattedAddress: "233 S Wacker Dr, Chicago, IL",
      lat: 41.8781,
      lon: -87.6298,
      score: expect.objectContaining({ compositeScore: 82 }),
    });
    // Scores by address text, not origin coordinates -- S_solar's street-
    // facing inference only runs when the request carries an address
    // string (factor_solar.go), so sending origin here would silently
    // default every score to South (100/100).
    expect(postMock).toHaveBeenCalledWith("/api/v1/score", { body: { address: "233 S Wacker Dr, Chicago, IL" } });
  });

  it("shows a not-found notice for zero geocode candidates", async () => {
    renderInput();

    search("nowhere");

    expect(await screen.findByText('No address matched "nowhere".')).toBeTruthy();
  });

  it("shows a picker for multiple geocode candidates and resolves the one selected", async () => {
    const { onResolved } = renderInput();

    search("ambiguous street");

    const option = await screen.findByText("200 Ambiguous St, Chicago, IL");
    fireEvent.click(option);

    await waitFor(() => expect(onResolved).toHaveBeenCalledTimes(1));
    expect(onResolved).toHaveBeenCalledWith(
      expect.objectContaining({ formattedAddress: "200 Ambiguous St, Chicago, IL", lat: 41.91, lon: -87.66 })
    );
  });

  it("shows an out-of-coverage notice on a 422 OUT_OF_COVERAGE score response", async () => {
    renderInput();

    search("out of coverage");

    expect(await screen.findByText(/falls outside the City of Chicago boundary/)).toBeTruthy();
  });

  it("shows a generic error notice when scoring fails for a reason other than OUT_OF_COVERAGE", async () => {
    renderInput();

    search("score error");

    expect(await screen.findByText(/Couldn't score that address/)).toBeTruthy();
  });

  it("shows a generic error notice when the geocode request itself throws", async () => {
    renderInput();

    search("network down");

    expect(await screen.findByText(/Couldn't reach the geocoder/)).toBeTruthy();
  });

  it("clears the previous result via onReset when a new search starts", async () => {
    const { onReset } = renderInput();

    search("233 S Wacker Dr");

    await waitFor(() => expect(onReset).toHaveBeenCalledTimes(1));
  });
});
