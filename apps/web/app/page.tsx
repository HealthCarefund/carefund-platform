import Link from "next/link";

const pillars = [
  {
    title: "Care agreement lifecycle",
    description:
      "Every agreement moves through an explicit, on-chain lifecycle — requested, funded, care confirmed, settled — so every party can see exactly where it stands.",
  },
  {
    title: "Transparent funding",
    description:
      "Sponsors fund agreements directly on Stellar. Funds are held by the contract, not by CareFund, until the conditions for settlement are met.",
  },
  {
    title: "Authorized verification",
    description:
      "Only providers and attesters registered and verified on-chain can confirm care was delivered — authorization the contracts enforce, not a claim this site makes.",
  },
  {
    title: "Settlement",
    description:
      "Once care is confirmed, settlement follows the agreement's own terms automatically, with disputes resolved through an explicit on-chain process.",
  },
];

export default function HomePage() {
  return (
    <div>
      <section className="relative overflow-hidden">
        <div
          aria-hidden="true"
          className="absolute inset-0 -z-10 bg-gradient-to-b from-brand-50 via-transparent to-transparent dark:from-brand-900/40"
        />
        <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 sm:py-28">
          <p className="text-sm font-semibold tracking-wide text-brand-600 uppercase dark:text-brand-300">
            Care funding, settled on-chain
          </p>
          <h1 className="mt-4 max-w-2xl text-4xl font-semibold tracking-tight text-balance sm:text-5xl">
            Fund care agreements with a transparent, verifiable settlement path.
          </h1>
          <p className="mt-6 max-w-xl text-lg text-ink-600 dark:text-ink-300">
            CareFund coordinates sponsors, providers, and attesters around a single
            on-chain care agreement — from funding through verified care to
            settlement.
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Link
              href="/sponsor"
              className="focus-ring rounded-lg bg-brand-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700"
            >
              Sponsor a care agreement
            </Link>
            <Link
              href="/provider"
              className="focus-ring rounded-lg border border-[var(--border-subtle)] px-5 py-3 text-sm font-medium transition-colors hover:bg-[var(--surface)]"
            >
              I&rsquo;m a provider
            </Link>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-4 pb-24 sm:px-6">
        <h2 className="text-sm font-semibold tracking-wide text-ink-500 uppercase dark:text-ink-400">
          How it works
        </h2>
        <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {pillars.map((pillar) => (
            <div
              key={pillar.title}
              className="rounded-xl border border-[var(--border-subtle)] bg-[var(--surface)] p-6"
            >
              <h3 className="text-base font-semibold">{pillar.title}</h3>
              <p className="mt-2 text-sm text-ink-600 dark:text-ink-300">{pillar.description}</p>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
