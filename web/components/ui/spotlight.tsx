"use client";

import { useRef, type ReactNode } from "react";

/**
 * Tracks the pointer and exposes it as CSS variables to every `.spotlight`
 * child, producing a soft light that follows the cursor (react-bits style).
 */
export function SpotlightGroup({ children, className }: { children: ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  return (
    <div
      ref={ref}
      className={className}
      onPointerMove={(e) => {
        const cards = ref.current?.querySelectorAll<HTMLElement>(".spotlight");
        cards?.forEach((c) => {
          const r = c.getBoundingClientRect();
          c.style.setProperty("--mx", `${e.clientX - r.left}px`);
          c.style.setProperty("--my", `${e.clientY - r.top}px`);
        });
      }}
    >
      {children}
    </div>
  );
}
