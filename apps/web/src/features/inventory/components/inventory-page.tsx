"use client";

import { SlidersHorizontal } from "@phosphor-icons/react";
import { useState } from "react";
import { NoticeBanner, useNotice } from "@/components/data/toolbar";
import { PageHeader } from "@/components/shell/page-header";
import { Button } from "@/components/ui/button";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { can } from "@/features/auth/api";
import { useSession } from "@/features/auth/session-store";
import { inventoryCopy as copy } from "../copy";
import { AdjustmentSheet } from "./adjustment-sheet";
import { AlertsView } from "./alerts-view";
import { MovementsView, type ProductFilter } from "./movements-view";

type View = (typeof copy.views)[number]["value"];

export function InventoryPage() {
  const me = useSession((s) => s.user);
  const [view, setView] = useState<View>("alerts");
  const [product, setProduct] = useState<ProductFilter | null>(null);
  const [adjusting, setAdjusting] = useState(false);
  const { notice, success } = useNotice();

  return (
    <>
      <PageHeader
        title={copy.title}
        description={copy.description}
        actions={
          can(me, "inventory:create") && (
            <Button variant="primary" onClick={() => setAdjusting(true)}>
              <SlidersHorizontal size={16} weight="bold" aria-hidden />
              {copy.adjust}
            </Button>
          )
        }
      />
      <div className="space-y-4 px-8 pb-10">
        <SegmentedControl label={copy.viewLabel} value={view} options={copy.views} onChange={setView} className="w-64" />
        <NoticeBanner notice={notice} />
        {view === "alerts" ? (
          <AlertsView
            canBuy={can(me, "purchases:create")}
            onOpenKardex={(p) => {
              setProduct(p);
              setView("movements");
            }}
          />
        ) : (
          <MovementsView product={product} onProduct={setProduct} />
        )}
      </div>
      <AdjustmentSheet open={adjusting} onClose={() => setAdjusting(false)} onDone={(name, stock) => success(copy.adjustment.done(name, stock))} />
    </>
  );
}
