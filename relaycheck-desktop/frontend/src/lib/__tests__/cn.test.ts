import { describe, it, expect } from "vitest";
import { cn } from "../cn";

describe("cn", () => {
  it("merges clsx conditional classes", () => {
    const result = cn("foo", false && "bar", "baz");
    expect(result).toBe("foo baz");
  });

  it("deduplicates tailwind classes via twMerge", () => {
    const result = cn("px-2 py-1", "px-4");
    expect(result).toBe("py-1 px-4");
  });

  it("combines clsx and twMerge", () => {
    const result = cn("text-sm font-bold", "text-lg");
    expect(result).toBe("font-bold text-lg");
  });

  it("returns empty string for no arguments", () => {
    expect(cn()).toBe("");
  });

  it("handles undefined and null inputs", () => {
    const result = cn("foo", undefined, null, "bar");
    expect(result).toBe("foo bar");
  });

  it("handles array input", () => {
    const result = cn(["foo", "bar"], "baz");
    expect(result).toBe("foo bar baz");
  });

  it("handles nested conditional array", () => {
    const result = cn(["foo", false && "bar", { baz: true, qux: false }]);
    expect(result).toBe("foo baz");
  });

  it("deduplicates conflicting tailwind margin classes", () => {
    const result = cn("m-2", "m-4");
    expect(result).toBe("m-4");
  });

  it("preserves non-conflicting tailwind classes", () => {
    const result = cn("px-2", "py-1", "bg-red-500");
    expect(result).toBe("px-2 py-1 bg-red-500");
  });
});
