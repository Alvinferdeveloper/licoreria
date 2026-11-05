import { Money } from "@/components/money";
import { cn } from "@/lib/cn";
import { stockLevel, type Product } from "../domain";
import { StockBadge } from "./stock-badge";

interface ProductRowProps {
  id: string;
  product: Product;
  available: number;
  active: boolean;
  onSelect: () => void;
  onHover: () => void;
}

export function ProductRow({ id, product, available, active, onSelect, onHover }: ProductRowProps) {
  const level = stockLevel({ stock: available, minStock: product.minStock });
  return (
    <li
      id={id}
      role="option"
      aria-selected={active}
      aria-disabled={level === "out"}
      onClick={onSelect}
      onMouseMove={onHover}
      className={cn(
        "grid cursor-pointer grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-x-6 rounded-md px-3 py-2.5",
        active && "bg-amber-tint",
        level === "out" && "cursor-not-allowed opacity-55",
      )}
    >
      <div className="min-w-0">
        <p className="truncate font-medium text-ink">{product.name}</p>
        <p className="truncate text-caption text-ink-muted">
          {product.presentation}
          <span className="mx-1.5 text-ink-subtle" aria-hidden>
            ·
          </span>
          <span className="font-mono tracking-wide text-ink-subtle">{product.sku}</span>
        </p>
      </div>
      <StockBadge level={level} units={available} />
      <Money cents={product.price} className="w-28 text-right font-mono text-body text-ink" />
    </li>
  );
}
