import { ImageResponse } from "next/og";

export const alt = "ModelRouter — One API for every frontier model";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OG() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%", height: "100%", display: "flex", flexDirection: "column", justifyContent: "space-between",
          padding: 72, background: "linear-gradient(135deg, #f6f8fb 0%, #dce8f8 100%)", color: "#0b1324", fontFamily: "sans-serif",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
          <div style={{ width: 56, height: 56, borderRadius: 16, background: "linear-gradient(135deg,#2f62d4,#2f8a86)" }} />
          <div style={{ fontSize: 34, fontWeight: 600, letterSpacing: -1 }}>ModelRouter</div>
        </div>
        <div style={{ display: "flex", flexDirection: "column" }}>
          <div style={{ fontSize: 84, fontWeight: 700, letterSpacing: -3, lineHeight: 1.02 }}>One API for every</div>
          <div style={{ fontSize: 84, fontWeight: 700, letterSpacing: -3, lineHeight: 1.02, color: "#2f62d4" }}>frontier model.</div>
        </div>
        <div style={{ display: "flex", gap: 28, fontSize: 26, color: "#5e6e86" }}>
          <span>OpenAI & Anthropic compatible</span><span>·</span><span>Pay as you go</span><span>·</span><span>USD & BDT</span>
        </div>
      </div>
    ),
    size,
  );
}
