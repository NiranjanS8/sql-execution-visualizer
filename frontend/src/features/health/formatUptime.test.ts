import { describe, expect, it } from "vitest";

import { formatUptime } from "./formatUptime";

describe("formatUptime", () => {
  it.each([
    [0, "0s"],
    [999, "0s"],
    [4_800, "4s"],
    [65_000, "1m 5s"],
    [3_725_000, "1h 2m"],
  ])("formats %i ms as %s", (input, expected) => {
    expect(formatUptime(input)).toBe(expected);
  });

  it("returns a dash for values that cannot be durations", () => {
    expect(formatUptime(Number.NaN)).toBe("—");
    expect(formatUptime(-1)).toBe("—");
  });
});
