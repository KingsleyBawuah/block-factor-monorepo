import { useState } from "react";
import type { Map as MapLibreMap } from "maplibre-gl";
import { Outlet } from "@tanstack/react-router";
import { Card, CardHeader, CardTitle, CardContent } from "./components/ui/card";
import { Badge } from "./components/ui/badge";
import { MapCanvas, CtaRailLayer, ProtectedBikeLayer, PoiDensityLayer, DivvyDockLayer, DEMO_ADDRESS_LAT, DEMO_ADDRESS_LON, DEMO_ADDRESS_FACING } from "./map";
import { solarBaseScore, solarScoreTier } from "./map/layers/solarScore";

const solarScore = solarBaseScore(DEMO_ADDRESS_FACING);
const solarTier = solarScoreTier(solarScore);

export default function App() {
  const [map, setMap] = useState<MapLibreMap | null>(null);

  return (
    <div className="flex min-h-screen flex-col bg-zinc-950 font-sans text-zinc-100">
      <header className="border-b border-zinc-800 bg-zinc-900/60 px-6 py-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <span className="text-sm font-semibold tracking-wider uppercase">
              BlockFactor
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
                <div className="flex items-baseline space-x-2">
                  <span className="text-4xl font-mono font-bold text-zinc-100">--</span>
                  <span className="text-xs font-mono text-zinc-500">/ 100</span>
                </div>
                <div className="mt-4 flex flex-wrap gap-1.5">
                  <Badge variant="cta-blue">Blue Line</Badge>
                  <Badge variant="cta-red">Red Line</Badge>
                  <Badge variant="cta-pink">Pink Line</Badge>
                  <Badge variant="cta-green">Green Line</Badge>
                </div>
                <div className="mt-3 flex items-center gap-2">
                  <span className="text-xs font-mono uppercase tracking-wider text-zinc-500">
                    Solar
                  </span>
                  <Badge variant={`score-${solarTier}`}>
                    {DEMO_ADDRESS_FACING}-Facing · {solarScore}
                  </Badge>
                </div>
              </CardContent>
            </Card>
            <Outlet />
          </div>

          <div className="lg:col-span-8">
            <Card className="h-[70vh] overflow-hidden p-0 lg:sticky lg:top-6 lg:h-[calc(100vh-7rem)]">
              <MapCanvas onMapReady={setMap} />
              {map && <CtaRailLayer map={map} />}
              {map && <ProtectedBikeLayer map={map} />}
              {map && <PoiDensityLayer map={map} lat={DEMO_ADDRESS_LAT} lon={DEMO_ADDRESS_LON} />}
              {map && <DivvyDockLayer map={map} />}
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}
