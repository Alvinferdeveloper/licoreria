import { describe, expect, it } from "vitest";
import { cn } from "./cn";

describe("cn", () => {
  it("keeps custom font sizes alongside text colors", () => {
    expect(cn("text-on-accent", "text-label")).toBe("text-on-accent text-label");
    expect(cn("text-total", "text-danger")).toBe("text-total text-danger");
  });

  it("still resolves conflicting font sizes", () => {
    expect(cn("text-label", "text-title")).toBe("text-title");
  });
});
