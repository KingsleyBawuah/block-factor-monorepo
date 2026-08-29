import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";
import App from "../App";
import { validateWeightSearch } from "../lib/weight-search-params";

// validateSearch lives on the root route (not indexRoute) so App -- which
// IS the root route's own component -- can call Route.useSearch() directly
// without needing `strict: false`. This is a single-page app with one real
// route, so there's no meaningful parent/child split here.
export const rootRoute = createRootRoute({
  validateSearch: validateWeightSearch,
  component: App,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
});

const routeTree = rootRoute.addChildren([indexRoute]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
