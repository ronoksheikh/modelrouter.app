import { cn } from "@/lib/cn";

/** Brand mark: one input routed to many destinations. */
export function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={cn("size-7", className)} aria-hidden="true">
      <defs>
        <linearGradient id="mr-g" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="var(--accent)" />
          <stop offset="1" stopColor="var(--teal)" />
        </linearGradient>
      </defs>
      <rect x="1" y="1" width="30" height="30" rx="9" fill="url(#mr-g)" />
      <path d="M9 16h4.5c2 0 2.6-6 6-6H23M13.5 16c2 0 2.6 6 6 6H23M13 16h10" stroke="#fff" strokeWidth="2" fill="none" strokeLinecap="round" />
      <circle cx="9" cy="16" r="2.4" fill="#fff" />
    </svg>
  );
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <LogoMark />
      <span className="text-[15px] font-semibold tracking-[-0.02em]">ModelRouter</span>
    </span>
  );
}
