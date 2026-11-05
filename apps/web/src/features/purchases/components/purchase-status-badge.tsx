import { Badge } from "@/components/ui/badge";
import type { PurchaseStatus } from "../api";
import { purchasesCopy as copy } from "../copy";

const tones = { draft: "amber", received: "success", voided: "neutral" } as const;

export function PurchaseStatusBadge({ status }: { status: PurchaseStatus }) {
  return <Badge tone={tones[status]}>{copy.status[status]}</Badge>;
}
