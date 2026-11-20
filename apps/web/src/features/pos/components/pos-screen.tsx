"use client";

import { LockSimple } from "@phosphor-icons/react";
import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect } from "react";
import { Button, buttonVariants } from "@/components/ui/button";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { cashKeys } from "@/features/cash/api";
import { catalogKeys } from "@/features/catalog/api";
import { useHotkeys } from "@/hooks/use-hotkeys";
import { posCopy } from "../copy";
import { usePosSession, type PosBlocker, type PosSession } from "../data/session";
import { focusSearch } from "../hooks/search-focus";
import { usePosStore } from "../store/pos-store";
import { useOfflineSync } from "../offline/use-offline-sync";
import { useTenderSummary } from "../store/selectors";
import { BillPanel } from "./bill-panel";
import { OfflineBanner } from "./offline-status";
import { PosHeader } from "./pos-header";
import { PosUserMenu } from "./pos-user-menu";
import { ProductSearch } from "./product-search";

export function PosScreen() {
  const { session, blocker } = usePosSession();

  useEffect(() => {
    usePosStore.persist.rehydrate();
  }, []);

  if (!session) return <PosBlocked blocker={blocker ?? "loading"} />;
  return <PosWorkspace session={session} />;
}

function PosWorkspace({ session }: { session: PosSession }) {
  const queryClient = useQueryClient();
  const mode = usePosStore((state) => state.mode);
  const lastSaleId = usePosStore((state) => state.lastSale?.id);
  const setRate = usePosStore((state) => state.setRate);
  const startCheckout = usePosStore((state) => state.startCheckout);
  const backToCart = usePosStore((state) => state.backToCart);
  const confirmSale = usePosStore((state) => state.confirmSale);
  const startNewSale = usePosStore((state) => state.startNewSale);
  const holdSale = usePosStore((state) => state.holdSale);
  const setNotice = usePosStore((state) => state.setNotice);
  const needsAuthorization = usePosStore((state) => state.needsAuthorization);
  const setOfflineContext = usePosStore((state) => state.setOfflineContext);
  const me = useSession((s) => s.user);
  const { isCovered } = useTenderSummary();

  useOfflineSync(me?.id);
  useEffect(() => setRate(session.exchangeRate), [session.exchangeRate, setRate]);
  useEffect(() => {
    if (!me) return;
    setOfflineContext({
      userId: me.id,
      cashierName: session.cashierName || me.fullName,
      registerName: session.registerName,
      discountLimitPercent: session.discountLimitPercent,
      canAuthorizeDiscounts: can(me, "discounts:authorize"),
    });
  }, [me, session, setOfflineContext]);

  // Stock and the shift summary changed with the sale.
  useEffect(() => {
    if (!lastSaleId) return;
    void queryClient.invalidateQueries({ queryKey: catalogKeys.products });
    void queryClient.invalidateQueries({ queryKey: cashKeys.current });
  }, [lastSaleId, queryClient]);

  useHotkeys({
    F2: () => {
      if (mode === "cart") focusSearch();
    },
    F4: () => {
      if (mode === "cart") startCheckout();
    },
    F8: () => {
      if (holdSale()) {
        setNotice(posCopy.bill.heldNotice);
        focusSearch();
      }
    },
    Escape: () => {
      if (mode !== "tender") return;
      backToCart();
      requestAnimationFrame(focusSearch);
    },
    ...(mode === "tender" && isCovered && !needsAuthorization && { Enter: () => void confirmSale() }),
    ...(mode === "complete" && {
      Enter: () => {
        startNewSale();
        requestAnimationFrame(focusSearch);
      },
    }),
  });

  return (
    <div className="grid h-dvh grid-rows-[auto_auto_minmax(0,1fr)] print:hidden">
      <PosHeader session={session} />
      <OfflineBanner />
      <main className="grid min-h-0 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_400px]">
        <ProductSearch locked={mode !== "cart"} />
        <BillPanel />
      </main>
    </div>
  );
}

/** Full-screen stop when the POS cannot sell yet, with the way to fix it. */
function PosBlocked({ blocker }: { blocker: PosBlocker }) {
  const me = useSession((s) => s.user);
  const queryClient = useQueryClient();
  const copy = posCopy.blocked;

  if (blocker === "loading") {
    return <div className="grid h-dvh place-items-center text-ink-muted">{copy.loading}</div>;
  }
  const content =
    blocker === "error"
      ? { ...copy.error, href: null }
      : blocker === "no-shift"
        ? { ...copy.noShift, href: "/caja" }
        : { ...copy.noRate, href: can(me, "exchange_rates:create") ? "/tipo-cambio" : null, body: can(me, "exchange_rates:create") ? copy.noRate.body : copy.noRate.ask };

  return (
    <div className="relative grid h-dvh place-items-center px-6">
      <div className="absolute top-3 right-5">
        <PosUserMenu />
      </div>
      <div className="max-w-md space-y-4 text-center">
        <LockSimple size={32} className="mx-auto text-ink-subtle" aria-hidden />
        <h1 className="text-headline font-semibold tracking-headline">{content.title}</h1>
        <p className="text-ink-muted">{content.body}</p>
        {blocker === "error" ? (
          <Button onClick={() => void queryClient.invalidateQueries()}>{copy.error.action}</Button>
        ) : (
          content.href && (
            <Link href={content.href} className={buttonVariants({ variant: "primary", size: "lg" })}>
              {content.action}
            </Link>
          )
        )}
      </div>
    </div>
  );
}
