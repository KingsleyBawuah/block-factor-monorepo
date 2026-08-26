import { GlobalRegistrator } from "@happy-dom/global-registrator";
import { afterEach, mock } from "bun:test";

GlobalRegistrator.register();

// Dynamic import, evaluated only after GlobalRegistrator.register() above:
// @testing-library/react pulls in react-dom, which caches its DOM
// availability check at module-load time. A static `import { cleanup }
// from "@testing-library/react"` is hoisted above GlobalRegistrator.register()
// regardless of source order, so react-dom would see no `document` yet and
// stay stuck thinking there's no DOM for the rest of the process — this was
// confirmed live: it broke every `screen.*` query (which reads
// `document.body`) while queries bound to a render()'s own return value
// kept working, since those hold a direct element reference instead of
// re-reading the global `document`.
const { cleanup } = await import("@testing-library/react");

// @testing-library/react's own auto-cleanup detects a global `afterEach`,
// but bun:test doesn't put one on `globalThis` even though it's available
// as an import — registering it explicitly here (the shared preload, see
// bunfig.toml) is what actually unmounts each rendered tree between tests,
// confirmed empirically: without this, two `render()` calls in the same
// file leave both DOM trees mounted rather than the second replacing the
// first.
afterEach(cleanup);

// happy-dom has no WebGL context, so the real maplibre-gl Map constructor
// (used by MapCanvas) would throw. Centralized here rather than per test
// file: mock.module() shares one module registry across all test files in
// a run (bun test doesn't isolate files unless run with --isolate), so a
// future test file that renders a map-bearing component without its own
// mock.module call would otherwise get whichever module state a prior file
// happened to leave behind.
mock.module("maplibre-gl", () => ({
  Map: mock(() => ({
    addControl: mock(() => {}),
    remove: mock(() => {}),
  })),
  NavigationControl: mock(() => ({})),
}));
