import { describe, it, expect } from "bun:test";
import { escapeHtml } from "./escapeHtml";

describe("escapeHtml", () => {
  it("escapes all five HTML-significant characters", () => {
    expect(escapeHtml(`&<>"'`)).toBe("&amp;&lt;&gt;&quot;&#39;");
  });

  it("leaves ordinary text unchanged", () => {
    expect(escapeHtml("Fullerton")).toBe("Fullerton");
  });

  it("neutralizes a script-tag injection attempt", () => {
    expect(escapeHtml('<img src=x onerror="alert(1)">')).toBe(
      "&lt;img src=x onerror=&quot;alert(1)&quot;&gt;"
    );
  });

  it("handles a real station name with an apostrophe correctly", () => {
    expect(escapeHtml("O'Hare")).toBe("O&#39;Hare");
  });
});
