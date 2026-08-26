import createClient from "openapi-fetch";
import type { paths } from "./schema";

// baseUrl is deliberately empty: schema.d.ts's path keys already carry the
// full "/api/v1/..." prefix (openapi.yaml declares paths that way), and
// openapi-fetch's createFinalURL does plain string concatenation of
// `${baseUrl}${pathname}` -- a non-empty "/api/v1" here would double-prefix
// every request to "/api/v1/api/v1/...". Vite's dev proxy (vite.config.ts)
// forwards the resulting relative "/api/..." URL to the Go API.
export const apiClient = createClient<paths>({
  baseUrl: "",
});
