import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";
import App from "../App";

const rootRoute = createRootRoute({
  component: App,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: () => (
    <div className="space-y-4">
      <div className="text-xs font-mono uppercase tracking-widest text-zinc-500">
        Workspace / Split-Screen Location Intelligence
      </div>
      <p className="text-sm text-zinc-400">
        Phase 1 Foundation & Contracts initialized. Enter an address or inspect API contract endpoints.
      </p>
    </div>
  ),
});

const routeTree = rootRoute.addChildren([indexRoute]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
