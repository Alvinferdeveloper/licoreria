import type { ReactNode } from "react";
import { Logo } from "@/components/brand/logo";
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
        <header className="mb-10">
          <Logo size="lg" />
          <p className="mt-3 text-body text-ink-muted">{authCopy.tagline}</p>
        </header>
        <h1 className="text-headline font-semibold tracking-headline">{title}</h1>
        {intro && <p className="mt-2 text-body text-ink-muted">{intro}</p>}
        <div className="mt-8">{children}</div>
      </div>
    </main>
  );
}
