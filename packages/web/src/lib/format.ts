const METERS_PER_MILE = 1609.344;

/**
 * Formats a straight-line distance in meters as miles for display (e.g.
 * "0.28 mi"), matching docs/ARCHITECTURE.md §7.2's distance formatting
 * convention. Distances render in Geist Mono at the call site, not here.
 */
export function formatDistanceMiles(meters: number): string {
  return `${(meters / METERS_PER_MILE).toFixed(2)} mi`;
}
