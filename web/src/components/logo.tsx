import Link from "next/link";

export function Logo() {
  return (
    <Link href="/" className="flex items-center gap-2 font-semibold tracking-tight">
      <svg viewBox="0 0 32 32" className="size-6" aria-hidden>
        <rect width="32" height="32" rx="7" className="fill-accent" />
        <path
          d="M9 22V14M14 22V10M19 22V16M24 22V12"
          className="stroke-accent-foreground"
          strokeWidth="3"
          strokeLinecap="round"
        />
      </svg>
      tally
    </Link>
  );
}
