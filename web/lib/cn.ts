type Value = string | false | null | undefined | 0;

/** Joins truthy class names. */
export function cn(...v: Value[]): string {
  return v.filter(Boolean).join(" ");
}
