"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import { useEffect, useState } from "react";

type Mode = "system" | "light" | "dark";
const order: Mode[] = ["system", "light", "dark"];

function apply(mode: Mode) {
  const root = document.documentElement;
  if (mode === "system") root.removeAttribute("data-theme");
  else root.setAttribute("data-theme", mode);
}

export function ThemeToggle() {
  const [mode, setMode] = useState<Mode>("system");
  useEffect(() => {
    try {
      const saved = localStorage.getItem("theme") as Mode | null;
      if (saved && order.includes(saved)) setMode(saved);
    } catch {}
  }, []);
  const Icon = mode === "light" ? Sun : mode === "dark" ? Moon : Monitor;
  return (
    <button
      type="button"
      aria-label={`Theme: ${mode}. Click to change.`}
      title={`Theme: ${mode}`}
      onClick={() => {
        const next = order[(order.indexOf(mode) + 1) % order.length];
        setMode(next);
        apply(next);
        try {
          localStorage.setItem("theme", next);
        } catch {}
      }}
      className="inline-flex size-9 items-center justify-center rounded-xl text-muted transition hover:bg-bg-soft hover:text-fg"
    >
      <Icon className="size-[18px]" />
    </button>
  );
}

/** Inline script that applies the saved theme before first paint. */
export const themeScript = `try{var t=localStorage.getItem("theme");if(t==="light"||t==="dark")document.documentElement.setAttribute("data-theme",t)}catch(e){}`;
