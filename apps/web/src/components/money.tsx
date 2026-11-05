import { formatMoney, type Cents, type CurrencyCode } from "@/lib/money";
import { cn } from "@/lib/cn";

interface MoneyProps {
  cents: Cents;
  currency?: CurrencyCode;
  className?: string;
}

export function Money({ cents, currency = "NIO", className }: MoneyProps) {
  return <span className={cn("tabular whitespace-nowrap", className)}>{formatMoney(cents, currency)}</span>;
}
