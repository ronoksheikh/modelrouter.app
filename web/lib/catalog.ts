import { backend } from "./backend";
import type { PublicModel } from "./types";

/** Public model catalog, cached and revalidated every 5 minutes (ISR). */
export async function getModels(): Promise<PublicModel[]> {
  try {
    const res = await backend<{ data: PublicModel[] }>("/api/public/models", { revalidate: 300, tags: ["models"] });
    return res.data ?? [];
  } catch {
    // Keep pages rendering if the API is briefly unavailable; ISR retries later.
    return [];
  }
}

export async function getModel(id: string): Promise<PublicModel | null> {
  const all = await getModels();
  return all.find((m) => m.id === id) ?? null;
}

export { modelHref } from "./links";
