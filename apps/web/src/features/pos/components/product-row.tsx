import { Money } from "@/components/money";
import { cn } from "@/lib/cn";
import { stockLevel, type PosPresentation, type PosProduct } from "../domain";
import { StockBadge } from "./stock-badge";

interface ProductRowProps {
  id: string;
  product: PosProduct;
  available: number;
  active: boolean;
  onSelect: (presentation: PosPresentation | null) => void;
  onHover: () => void;
}

export function ProductRow({ id, product, available, active, onSelect, onHover }: ProductRowProps) {
  const level = stockLevel({ stock: available, minStock: product.minStock });
  const soldOut = level === "out" && !product.allowNegativeStock;
  return (
    <li
      id={id}
      role="option"
      aria-selected={active}
      aria-disabled={soldOut}
      onClick={() => onSelect(null)}
      onMouseMove={onHover}
      className={cn(
        "grid cursor-pointer grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-x-6 rounded-md px-3 py-2.5",
        active && "bg-amber-tint",
        soldOut && "cursor-not-allowed opacity-55",
      )}
    >
      <div className="min-w-0">
        <p className="truncate font-medium text-ink">{product.name}</p>
        <p className="flex flex-wrap items-center gap-x-1.5 gap-y-1 text-caption text-ink-muted">
          <span className="font-mono tracking-wide text-ink-subtle">{product.sku}</span>
          {product.presentations.map((pr) => (
            <button
              key={pr.id}
              type="button"
              tabIndex={-1}
              disabled={soldOut || available < pr.factor}
              onClick={(event) => {
                event.stopPropagation();
                onSelect(pr);
              }}
              className="press rounded-full bg-sunken px-2 py-0.5 font-medium text-ink-muted transition-colors hover:bg-amber-tint hover:text-amber-ink disabled:opacity-50"
            >
              {pr.name} · <Money cents={pr.price} className="font-mono" />
            </button>
          ))}
        </p>
      </div>
      <StockBadge level={level} units={available} />
      <Money cents={product.price} className="w-28 text-right font-mono text-body text-ink" />
    </li>
  );
}
