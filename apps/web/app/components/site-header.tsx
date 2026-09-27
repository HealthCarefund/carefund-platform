import Link from "next/link";
import { ThemeToggle } from "./theme-toggle";

export function SiteHeader() {
  return (
    <header className="border-b border-[var(--border-subtle)] bg-[var(--background)]/80 backdrop-blur">
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

        <nav aria-label="Primary" className="hidden items-center gap-6 text-sm font-medium text-ink-600 sm:flex dark:text-ink-300">
          <Link href="/provider" className="focus-ring rounded hover:text-[var(--foreground)]">
            For providers
          </Link>
          <Link href="/sponsor" className="focus-ring rounded hover:text-[var(--foreground)]">
            For sponsors
          </Link>
          <Link href="/admin/providers" className="focus-ring rounded hover:text-[var(--foreground)]">
            Provider directory
          </Link>
        </nav>

        <div className="flex items-center gap-3">
          <ThemeToggle />
          <Link
            href="/connect"
            className="focus-ring rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-brand-700"
          >
            Connect wallet
          </Link>
        </div>
      </div>
    </header>
  );
}
