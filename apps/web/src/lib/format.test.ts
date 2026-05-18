import { describe, expect, it } from "vitest";

import { formatBytes, formatTransferRate, toBytes } from "./format";

describe("format helpers", () => {
  it("formats zero storage quota as unlimited", () => {
    expect(formatBytes(0)).toBe("Unlimited");
  });

  it("parses human storage quotas", () => {
    expect(toBytes("1 GB")).toBe(1024 ** 3);
    expect(toBytes("1.5 TB")).toBe(Math.round(1.5 * 1024 ** 4));
  });

  it("formats transfer rates without treating zero as unlimited", () => {
    expect(formatTransferRate(0)).toBe("0 B/s");
    expect(formatTransferRate(1536)).toBe("1.5 KB/s");
  });

  it("rejects invalid storage quotas", () => {
    expect(Number.isNaN(toBytes("many bytes"))).toBe(true);
  });
});
