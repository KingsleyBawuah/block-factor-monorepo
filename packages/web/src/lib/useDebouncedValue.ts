import { useEffect, useState } from "react";

/**
 * Returns `value`, but only after it's stopped changing for `delayMs`.
 * Used to debounce the Weight Customization Slider Drawer's re-score
 * request -- the sliders themselves update instantly on every drag tick,
 * but firing a POST /api/v1/score per tick would spam the API during a
 * single drag.
 */
export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return debounced;
}
