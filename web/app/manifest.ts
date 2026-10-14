import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "ModelRouter",
    short_name: "ModelRouter",
    description: "One API for every frontier model.",
    start_url: "/dashboard",
    display: "standalone",
    background_color: "#f6f8fb",
    theme_color: "#2f62d4",
    icons: [{ src: "/icon.svg", sizes: "any", type: "image/svg+xml" }],
  };
}
