import { Warning, XCircle } from "@phosphor-icons/react/dist/ssr";
import { Badge } from "@/components/ui/badge";
import { posCopy } from "../copy";
import type { StockLevel } from "../domain";

export function StockBadge({ level, units }: { level: StockLevel; units: number }) {
  if (level === "out") {
    return (
      <Badge tone="danger">
        <XCircle size={12} weight="bold" aria-hidden />
        {posCopy.search.outOfStock}
      </Badge>
    );
  }
  if (level === "low") {
    return (
      <Badge tone="warning">
        <Warning size={12} weight="bold" aria-hidden />
        {posCopy.search.lowStock} · {units}
      </Badge>
    );
  }
  return <span className="tabular text-caption text-ink-subtle">{posCopy.search.inStock(units)}</span>;
}
