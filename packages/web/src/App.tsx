import { Card, CardHeader, CardTitle, CardContent } from "./components/ui/card";
import { Badge } from "./components/ui/badge";

export default function App() {
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
              </CardContent>
            </Card>
          </div>

          <div className="lg:col-span-8">
            <Card className="h-full flex items-center justify-center border-dashed border-zinc-800 bg-zinc-900/20">
              <div className="text-center">
                <span className="text-xs font-mono tracking-widest text-zinc-500 uppercase">
                  MapLibre WebGL Canvas Container
                </span>
                <p className="mt-1 text-xs text-zinc-600">
                  Vector tiles & isochrone layers mount here in Phase 4.
                </p>
              </div>
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}
