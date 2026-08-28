import { useEffect, useState } from "react";
import type { Map as MapLibreMap } from "maplibre-gl";
import { Outlet } from "@tanstack/react-router";
import { Card, CardHeader, CardTitle, CardContent } from "./components/ui/card";
import { Badge } from "./components/ui/badge";
import { AddressSearchInput, type ResolvedAddress } from "./components/AddressSearchInput";
import { MapCanvas, CtaRailLayer, ProtectedBikeLayer, PoiDensityLayer, DivvyDockLayer, RESOLVED_ADDRESS_ZOOM, scoreTier } from "./map";

export default function App() {
  const [map, setMap] = useState<MapLibreMap | null>(null);
  const [resolved, setResolved] = useState<ResolvedAddress | null>(null);

  useEffect(() => {
    if (!map || !resolved) return;
    map.flyTo({ center: [resolved.lon, resolved.lat], zoom: RESOLVED_ADDRESS_ZOOM });
  }, [map, resolved]);

  const solar = resolved?.score.factors.solar;

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
                <AddressSearchInput onResolved={setResolved} onReset={() => setResolved(null)} />

                {resolved ? (
                  <>
                    <div className="mt-4 truncate font-mono text-xs text-zinc-500">
                      {resolved.formattedAddress}
                    </div>
                    <div className="mt-1 flex items-baseline space-x-2">
                      <span className="text-4xl font-mono font-bold text-zinc-100">
                        {Math.round(resolved.score.compositeScore)}
                      </span>
                      <span className="text-xs font-mono text-zinc-500">/ 100</span>
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

                {solar && (
                  <div className="mt-3 flex items-center gap-2">
                    <span className="text-xs font-mono uppercase tracking-wider text-zinc-500">
                      Solar
                    </span>
                    {solar.degraded || solar.score == null ? (
                      <Badge variant="outline">Unavailable</Badge>
                    ) : (
                      <Badge variant={`score-${scoreTier(solar.score)}`}>{Math.round(solar.score)}</Badge>
                    )}
                  </div>
                )}
              </CardContent>
            </Card>
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
