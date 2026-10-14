import type { MetadataRoute } from "next";
import { getModels } from "@/lib/catalog";
import { docsFlat } from "@/lib/docs";
import { site } from "@/lib/site";

export const revalidate = 3600;

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const now = new Date();
  const statics = ["", "/models", "/pricing", "/terms", "/privacy", "/refunds"].map((p) => ({
    url: site.url + p,
    lastModified: now,
    changeFrequency: p === "" || p === "/models" ? ("daily" as const) : ("monthly" as const),
    priority: p === "" ? 1 : p === "/models" ? 0.9 : 0.5,
  }));
  const docs = docsFlat.map((d) => ({ url: site.url + d.href, lastModified: now, changeFrequency: "weekly" as const, priority: 0.7 }));
  const models = (await getModels()).map((m) => ({
    url: `${site.url}/models/${m.id}`,
    lastModified: new Date(m.created_at),
    changeFrequency: "weekly" as const,
    priority: m.featured ? 0.8 : 0.6,
  }));
  return [...statics, ...docs, ...models];
}
