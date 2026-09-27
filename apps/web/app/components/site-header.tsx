import Link from "next/link";
import { ThemeToggle } from "./theme-toggle";
import { WalletButton } from "./wallet-button";
import { SiteNav } from "./site-nav";

export function SiteHeader() {
  return (
    <header className="relative border-b border-[var(--border-subtle)] bg-[var(--background)]/80 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
        <Link href="/" className="focus-ring flex items-center gap-2 rounded">
          <span
            aria-hidden="true"
            className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-brand-500 to-brand-700 text-sm font-semibold text-white"
          >
            CF
          </span>
          <span className="text-base font-semibold tracking-tight">CareFund</span>
        </Link>

        <SiteNav />

        <div className="flex items-center gap-3">
          <ThemeToggle />
          <WalletButton />
        </div>
      </div>
    </header>
  );
}
