import Link from "next/link";
import { Logo } from "@/components/ui/logo";

const cols = [
  { title: "Product", links: [["Models", "/models"], ["Pricing", "/pricing"], ["Dashboard", "/dashboard"], ["Status", "/status"]] },
  { title: "Developers", links: [["Documentation", "/docs"], ["Quickstart", "/docs/quickstart"], ["OpenAI SDK", "/docs/openai"], ["Anthropic SDK", "/docs/anthropic"]] },
  { title: "Company", links: [["Terms", "/terms"], ["Privacy", "/privacy"], ["Refunds", "/refunds"], ["Contact", "mailto:support@modelrouter.app"]] },
] as const;

export function SiteFooter() {
  return (
    <footer className="border-t border-border">
      <div className="container-page grid gap-10 py-14 md:grid-cols-[1.4fr_repeat(3,1fr)]">
        <div className="max-w-xs">
          <Logo />
          <p className="mt-4 text-sm leading-6 text-muted">
            One API for every frontier model. Built in Bangladesh, for developers everywhere.
          </p>
        </div>
        {cols.map((c) => (
          <div key={c.title}>
            <p className="text-[13px] font-medium text-fg">{c.title}</p>
            <ul className="mt-3 space-y-2.5">
              {c.links.map(([label, href]) => (
                <li key={href}>
                  <Link href={href} className="text-sm text-muted transition hover:text-fg">
                    {label}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <div className="container-page flex flex-col gap-2 border-t border-border py-6 text-xs text-subtle sm:flex-row sm:items-center sm:justify-between">
        <p>© {new Date().getFullYear()} ModelRouter. All rights reserved.</p>
        <p>Payments in USD worldwide · BDT for Bangladesh</p>
      </div>
    </footer>
  );
}
