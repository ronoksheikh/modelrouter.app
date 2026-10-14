import { describe, expect, it } from "vitest";
import { authorName, centsToBDT, formatBps, formatPriceString, formatTokens, formatUSD, timeAgo } from "./format";

describe("formatUSD", () => {
  it("formats regular amounts", () => {
    expect(formatUSD(5_000_000_000)).toBe("$5.00");
    expect(formatUSD(1_234_567_000_000)).toBe("$1,234.57");
    expect(formatUSD(-2_500_000_000)).toBe("-$2.50");
    expect(formatUSD(0)).toBe("$0.00");
  });
  it("keeps precision for tiny amounts", () => {
    expect(formatUSD(10)).toBe("$0.00000001");
    expect(formatUSD(3_000_010)).toBe("$0.00300001");
  });
});

describe("formatPriceString", () => {
  it("handles free and small prices", () => {
    expect(formatPriceString("0")).toBe("Free");
    expect(formatPriceString("0.165")).toBe("$0.165");
    expect(formatPriceString("15")).toBe("$15");
    expect(formatPriceString("2.5")).toBe("$2.5");
    expect(formatPriceString("0.00000001")).toBe("$0.00000001");
  });
});

describe("misc", () => {
  it("formats tokens", () => {
    expect(formatTokens(128000)).toBe("128K");
    expect(formatTokens(1_048_576)).toBe("1.05M");
    expect(formatTokens(8192)).toBe("8.2K");
    expect(formatTokens(0)).toBe("—");
  });
  it("converts to BDT", () => expect(centsToBDT(1000, 122)).toBe(1220));
  it("formats bps", () => expect(formatBps(1000)).toBe("10%"));
  it("names authors", () => {
    expect(authorName("openai")).toBe("OpenAI");
    expect(authorName("acme")).toBe("Acme");
    expect(authorName("")).toBe("ModelRouter");
  });
  it("relative time", () => {
    const now = Date.parse("2026-01-01T12:00:00Z");
    expect(timeAgo("2026-01-01T11:00:00Z", now)).toBe("1h ago");
    expect(timeAgo(null, now)).toBe("Never");
  });
});
