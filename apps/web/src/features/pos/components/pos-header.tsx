import { CashRegister } from "@phosphor-icons/react/dist/ssr";
import { ThemeToggle } from "@/features/theme";
import { formatRate } from "@/lib/money";
import { posCopy } from "../copy";
import type { PosSession } from "../data/session";
import { PosUserMenu } from "./pos-user-menu";

export function PosHeader({ session }: { session: PosSession }) {
  return (
    <header className="flex h-13 items-center gap-6 border-b border-line bg-chrome px-5">
      <div className="flex items-baseline gap-3">
        <span className="text-title font-semibold tracking-headline">Añejo</span>
        <span className="hidden text-label text-ink-muted sm:inline">{session.storeName}</span>
      </div>

      <span className="flex items-center gap-1.5 text-label text-ink-muted">
        <CashRegister size={16} aria-hidden />
        <span className="text-ink">{session.registerName}</span>
        <span className="flex items-center gap-1.5 text-caption">
          <span className="size-1.5 rounded-full bg-success" aria-hidden />
          {posCopy.header.shift}
        </span>
      </span>

      <div className="ml-auto flex items-center gap-3">
        <span className="hidden text-label text-ink-muted lg:inline">
          {posCopy.header.rate}{" "}
          <span className="tabular font-mono text-ink">US$ 1 = C$ {formatRate(session.exchangeRate)}</span>
        </span>
        <ThemeToggle lightLabel={posCopy.header.themeLight} darkLabel={posCopy.header.themeDark} />
        <PosUserMenu />
      </div>
    </header>
  );
}
