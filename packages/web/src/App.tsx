import { useEffect, useMemo, useRef, useState } from "react";
import type { Map as MapLibreMap } from "maplibre-gl";
import { Outlet, useNavigate, useSearch } from "@tanstack/react-router";
import { useMutation } from "@tanstack/react-query";
import { Card, CardHeader, CardTitle, CardContent } from "./components/ui/card";
import { Badge } from "./components/ui/badge";
import { ScoreRing } from "./components/ui/score-ring";
import { AddressSearchInput, type ResolvedAddress } from "./components/AddressSearchInput";
import { FactorBreakdownList } from "./components/FactorBreakdownList";
import { WeightDrawer } from "./components/WeightDrawer";
import { MapCanvas, CtaRailLayer, ProtectedBikeLayer, PoiDensityLayer, DivvyDockLayer, RESOLVED_ADDRESS_ZOOM } from "./map";
import { apiClient } from "./lib/api/client";
import dayjs from "./lib/dayjs";
import { cn } from "./lib/utils";
import { useDebouncedValue } from "./lib/useDebouncedValue";
import { searchToWeights, weightsToSearch, type WeightSearch } from "./lib/weight-search-params";
import type { Weights } from "./lib/weights";
import type { components } from "./lib/api/schema";

type ScoreResponse = components["schemas"]["ScoreResponse"];
type Origin = { lat: number; lon: number; formattedAddress: string };

const RESCORE_DEBOUNCE_MS = dayjs.duration(400, "milliseconds").asMilliseconds();

export default function App() {
  const [map, setMap] = useState<MapLibreMap | null>(null);
  const [resolved, setResolved] = useState<ResolvedAddress | null>(null);

  // validateSearch (routes/index.tsx, on the root route) always returns a
  // complete, sum-100 WeightSearch -- `strict: false` sidesteps a circular
  // import (App.tsx is the root route's own component, so App.tsx can't
  // import the root Route object back from routes/index.tsx to call its
  // typed Route.useSearch()) at the cost of this one asserted cast.
  const rawSearch = useSearch({ strict: false }) as WeightSearch;
  // Memoized on the 9 raw primitive values, not just `useMemo(..., [rawSearch])`
  // -- searchToWeights would otherwise return a brand-new object every
  // render regardless of whether any weight actually changed, which fed a
  // real infinite loop: useDebouncedValue's effect compares its input by
  // reference, so a same-content-but-new-reference `weights` every render
  // meant debouncedWeights never settled, kept re-firing the rescore
  // mutation, which called setResolved, which re-rendered, which produced
  // yet another new `weights` reference -- forever (confirmed live: ~20
  // POST /api/v1/score calls/sec with no slider ever touched).
  // biome-ignore lint/correctness/useExhaustiveDependencies: depending on rawSearch itself (Biome's suggested fix) is exactly what caused the loop above -- rawSearch is a new object every render regardless of content, so it can never be a valid memo dependency here. The 9 listed fields are the complete, exact set searchToWeights reads (weight-search-params.ts) -- nothing is actually missing.
  const weights = useMemo(
    () => searchToWeights(rawSearch),
    [
      rawSearch.w_transit,
      rawSearch.w_bike,
      rawSearch.w_grocery,
      rawSearch.w_thirdPlace,
      rawSearch.w_green,
      rawSearch.w_noise,
      rawSearch.w_safety,
      rawSearch.w_flood,
      rawSearch.w_solar,
    ]
  );
  const navigate = useNavigate({ from: "/" });
  const debouncedWeights = useDebouncedValue(weights, RESCORE_DEBOUNCE_MS);

  // Holds the scored origin outside React state: writing it must never
  // itself re-trigger the rescore effect below (only a real weight change
  // should), and a ref read isn't a reactive dependency. Resolving a new
  // address writes this without touching `weights`/`debouncedWeights` at
  // all, so the rescore effect below (keyed only on debouncedWeights)
  // never fires just because a search happened -- no separate "skip the
  // next one" flag needed.
  const originRef = useRef<Origin | null>(null);

  const rescoreMutation = useMutation({
    mutationFn: async (vars: { origin: Origin; weights: Weights }): Promise<ScoreResponse> => {
      const { data, error } = await apiClient.POST("/api/v1/score", {
        body: {
          origin: { lat: vars.origin.lat, lon: vars.origin.lon },
          addressHint: vars.origin.formattedAddress,
          weights: vars.weights,
        },
      });
      if (error) throw error;
      return data;
    },
  });

  function handleResolved(next: ResolvedAddress) {
    originRef.current = { lat: next.lat, lon: next.lon, formattedAddress: next.formattedAddress };
    setResolved(next);
  }

  function handleReset() {
    originRef.current = null;
    setResolved(null);
  }

  useEffect(() => {
    const origin = originRef.current;
    if (!origin) return;
    rescoreMutation.mutate(
      { origin, weights: debouncedWeights },
      {
        onSuccess: (score) => {
          // Guards against a slow rescore for an address the user has
          // since navigated away from landing after a newer search already
          // resolved -- rescoreMutation isn't itself cancelled, so this
          // keeps a stale response from overwriting a newer one.
          if (originRef.current?.lat === origin.lat && originRef.current?.lon === origin.lon) {
            setResolved((prev) => (prev ? { ...prev, score } : prev));
          }
        },
      }
    );
    // rescoreMutation.mutate is referentially stable across renders
    // (TanStack Query), so including it here doesn't cause extra re-runs.
  }, [debouncedWeights, rescoreMutation.mutate]);

  function handleWeightsChange(next: Weights) {
    navigate({ search: (prev) => ({ ...prev, ...weightsToSearch(next) }) });
  }

  useEffect(() => {
    if (!map || !resolved) return;
    map.flyTo({ center: [resolved.lon, resolved.lat], zoom: RESOLVED_ADDRESS_ZOOM });
  }, [map, resolved]);

  return (
    <div className="flex min-h-screen flex-col bg-zinc-950 font-sans text-zinc-100">
      <header className="border-b border-zinc-800 bg-zinc-900/60 px-6 py-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <span className="text-sm font-semibold tracking-wider uppercase">
              <span className="text-flag-blue">Block</span>
              <span className="text-flag-red">Factor</span>
            </span>
            <span className="text-xs font-mono text-zinc-500">v0.1.0-alpha</span>
          </div>
          <div className="flex items-center space-x-2">
            <Badge variant="score-emerald">Status / Online</Badge>
          </div>
        </div>
      </header>

      <main className="flex flex-1 p-6">
        <div className="grid w-full grid-cols-1 gap-6 lg:grid-cols-12">
          <div className="lg:col-span-4 space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Livability Score</CardTitle>
              </CardHeader>
              <CardContent>
                <AddressSearchInput onResolved={handleResolved} onReset={handleReset} weights={weights} />

                {resolved ? (
                  <>
                    <div className="mt-4 truncate font-mono text-xs text-zinc-500">
                      {resolved.formattedAddress}
                    </div>
                    <div
                      className={cn(
                        "mt-3 flex justify-center transition-opacity motion-reduce:transition-none",
                        rescoreMutation.isPending && "opacity-60"
                      )}
                    >
                      <ScoreRing score={resolved.score.compositeScore} />
                    </div>
                  </>
                ) : (
                  <div className="mt-4 py-2 font-mono text-xs text-zinc-500">
                    Search an address above to see its BlockFactor score.
                  </div>
                )}

                <div className="mt-4 flex flex-wrap gap-1.5">
                  <Badge variant="cta-blue">Blue Line</Badge>
                  <Badge variant="cta-red">Red Line</Badge>
                  <Badge variant="cta-pink">Pink Line</Badge>
                  <Badge variant="cta-green">Green Line</Badge>
                </div>
              </CardContent>
            </Card>

            {resolved && (
              <>
                <WeightDrawer weights={weights} onChange={handleWeightsChange} />
                <div
                  className={cn(
                    "transition-opacity motion-reduce:transition-none",
                    rescoreMutation.isPending && "opacity-60"
                  )}
                >
                  <FactorBreakdownList factors={resolved.score.factors} />
                </div>
              </>
            )}

            <Outlet />
          </div>

          <div className="min-w-0 lg:col-span-8">
            {/*
              clip-path (not just overflow-hidden) is required here: Firefox
              renders the MapLibre WebGL canvas on its own GPU-composited
              layer, which its overflow:hidden + border-radius clipping
              doesn't reach -- a long-standing Gecko bug (e.g. Mozilla
              Bugzilla #915492, #537433), confirmed live (rounded in Chrome,
              square in Firefox/Zen at the same commit). clip-path is a real
              compositor-level clip Firefox does honor on canvas layers.
              0.375rem matches index.css's --radius-lg override (rounded-lg's
              Tailwind default of 0.5rem is NOT what actually renders here).
            */}
            <Card className="h-[70vh] overflow-hidden p-0 [clip-path:inset(0_round_0.375rem)] lg:sticky lg:top-6 lg:h-[calc(100vh-7rem)]">
              <MapCanvas onMapReady={setMap} />
              {map && <CtaRailLayer map={map} />}
              {map && <ProtectedBikeLayer map={map} />}
              {map && resolved && <PoiDensityLayer map={map} lat={resolved.lat} lon={resolved.lon} />}
              {map && <DivvyDockLayer map={map} />}
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}
