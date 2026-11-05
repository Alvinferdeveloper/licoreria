import type { ReactNode } from "react";
import { authCopy } from "../copy";

interface AuthFrameProps {
  title: string;
  intro?: ReactNode;
  children: ReactNode;
}

/** Shared frame for the signed-out screens: a narrow, left-aligned column with the wordmark. */
export function AuthFrame({ title, intro, children }: AuthFrameProps) {
  return (
    <main className="grid min-h-dvh place-items-center bg-paper px-5 py-12">
      <div className="w-full max-w-[22rem]">
        <p className="mb-12 flex items-baseline gap-2">
          <span className="text-title font-semibold tracking-headline">{authCopy.brand}</span>
          <span className="text-label text-ink-subtle">{authCopy.tagline}</span>
        </p>
        <h1 className="text-headline font-semibold tracking-headline">{title}</h1>
        {intro && <p className="mt-2 text-body text-ink-muted">{intro}</p>}
        <div className="mt-8">{children}</div>
      </div>
    </main>
  );
}
