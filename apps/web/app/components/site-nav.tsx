"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

const NAV_LINKS = [
  { href: "/provider", label: "For providers" },
  { href: "/sponsor", label: "For sponsors" },
  { href: "/admin/providers", label: "Provider directory" },
];

function isActive(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function SiteNav() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);

  return (
    <>
      <nav
        aria-label="Primary"
        className="hidden items-center gap-6 text-sm font-medium text-ink-600 sm:flex dark:text-ink-300"
      >
        {NAV_LINKS.map((link) => (
          <Link
            key={link.href}
            href={link.href}
            aria-current={isActive(pathname, link.href) ? "page" : undefined}
            className="focus-ring rounded hover:text-[var(--foreground)] aria-[current=page]:text-[var(--foreground)] aria-[current=page]:font-semibold"
          >
            {link.label}
          </Link>
        ))}
      </nav>

      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="primary-nav-mobile"
        className="focus-ring rounded-lg border border-[var(--border-subtle)] p-2 text-sm font-medium sm:hidden"
      >
        <span className="sr-only">{open ? "Close menu" : "Open menu"}</span>
        <svg aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" className="h-5 w-5">
          {open ? (
            <path strokeLinecap="round" strokeLinejoin="round" d="M5 5l10 10M15 5L5 15" />
          ) : (
            <path strokeLinecap="round" strokeLinejoin="round" d="M3 5h14M3 10h14M3 15h14" />
          )}
        </svg>
      </button>

      {open && (
        <nav
          id="primary-nav-mobile"
          aria-label="Primary"
          className="absolute inset-x-0 top-full flex flex-col gap-1 border-b border-[var(--border-subtle)] bg-[var(--background)] px-4 py-3 text-sm font-medium sm:hidden"
        >
          {NAV_LINKS.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              aria-current={isActive(pathname, link.href) ? "page" : undefined}
              onClick={() => setOpen(false)}
              className="focus-ring rounded px-2 py-2 hover:bg-[var(--surface)] aria-[current=page]:font-semibold aria-[current=page]:text-[var(--foreground)]"
            >
              {link.label}
            </Link>
          ))}
        </nav>
      )}
    </>
  );
}
