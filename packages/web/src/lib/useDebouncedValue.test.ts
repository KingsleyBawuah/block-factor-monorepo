import { describe, it, expect, spyOn } from "bun:test";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useDebouncedValue } from "./useDebouncedValue";

function wait(ms: number) {
  return act(() => new Promise((resolve) => setTimeout(resolve, ms)));
}

describe("useDebouncedValue", () => {
  it("returns the initial value immediately", () => {
    const { result } = renderHook(({ value }) => useDebouncedValue(value, 30), {
      initialProps: { value: "a" },
    });
    expect(result.current).toBe("a");
  });

  it("keeps the old value until the delay elapses after the last change", async () => {
    const { result, rerender } = renderHook(({ value }) => useDebouncedValue(value, 30), {
      initialProps: { value: "a" },
    });

    rerender({ value: "b" });
    expect(result.current).toBe("a"); // not yet -- delay hasn't elapsed

    await waitFor(() => expect(result.current).toBe("b"));
  });

  it("resets the delay on every change, so only the final value in a rapid burst is ever committed", async () => {
    const { result, rerender } = renderHook(({ value }) => useDebouncedValue(value, 30), {
      initialProps: { value: 1 },
    });

    rerender({ value: 2 });
    await wait(10);
    rerender({ value: 3 });
    await wait(10);
    rerender({ value: 4 });

    expect(result.current).toBe(1); // still the initial value -- no gap >= 30ms yet

    await waitFor(() => expect(result.current).toBe(4));
  });

  it("clears its pending timer on unmount, so a stale setDebounced never fires against an unmounted component", () => {
    const clearTimeoutSpy = spyOn(globalThis, "clearTimeout");
    const { rerender, unmount } = renderHook(({ value }) => useDebouncedValue(value, 30), {
      initialProps: { value: "a" },
    });

    rerender({ value: "b" }); // schedules a pending timer
    unmount();

    expect(clearTimeoutSpy).toHaveBeenCalled();
    clearTimeoutSpy.mockRestore();
  });
});
