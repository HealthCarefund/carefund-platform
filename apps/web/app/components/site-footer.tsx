function currentNetworkLabel(): string {
  const network = process.env.NEXT_PUBLIC_STELLAR_NETWORK;
  return network ? network : "network not configured";
}

export function SiteFooter() {
  return (
    <footer className="border-t border-[var(--border-subtle)] py-8 text-sm text-ink-500 dark:text-ink-400">
      <div className="mx-auto flex max-w-6xl flex-col gap-3 px-4 sm:flex-row sm:items-center sm:justify-between sm:px-6">
        <p>&copy; {new Date().getFullYear()} CareFund. Care agreements, funded transparently.</p>
        <p className="inline-flex items-center gap-2">
          <span
            aria-hidden="true"
            className="h-1.5 w-1.5 rounded-full bg-brand-500"
          />
          Stellar {currentNetworkLabel()}
        </p>
      </div>
    </footer>
  );
}
