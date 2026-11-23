import type { Cents, CurrencyCode } from "@/lib/money";
import type { Discount } from "./discount";

/** A sellable product as the POS needs it; built from the catalog API. */
export interface PosProduct {
  id: string;
  sku: string;
  barcode: string | null;
  name: string;
  /** Size or pack shown under the name, e.g. "750 ml". */
  detail: string;
  imageUrl: string | null;
  /** Unit price before tax, in NIO cents. */
  price: Cents;
  taxPercent: number;
  /** Stock in base units. */
  stock: number;
  minStock: number;
  allowNegativeStock: boolean;
  presentations: PosPresentation[];
}

export interface PosPresentation {
  id: string;
  name: string;
  /** Base units contained, e.g. 6 for a six-pack. */
  factor: number;
  price: Cents;
}

export interface CartLine {
  /** productId plus presentationId, so a unit and a six-pack of the same beer are separate lines. */
  key: string;
  productId: string;
  presentationId: string | null;
  name: string;
  detail: string;
  unitPrice: Cents;
  taxPercent: number;
  quantity: number;
  unitsPerItem: number;
}

export interface CartTotals {
  subtotal: Cents;
  discount: Cents;
  tax: Cents;
  total: Cents;
  itemCount: number;
}

export type PaymentMethod = "cash" | "card" | "transfer";

export interface Payment {
  id: string;
  method: PaymentMethod;
  currency: CurrencyCode;
  /** Amount received in the payment's own currency. */
  amount: Cents;
  /** Voucher authorization (card) or transfer reference; cash has none. */
  reference?: string;
}

export type StockLevel = "ok" | "low" | "out";

export interface Customer {
  id: string;
  name: string;
  /** Cédula or RUC; required to issue an invoice in the customer's name. */
  taxId: string | null;
  phone: string | null;
}

export interface HeldSale {
  id: string;
  lines: CartLine[];
  /** Products as last seen, to keep checking stock after resuming. */
  products: Record<string, PosProduct>;
  discount: Discount | null;
  customer: Customer | null;
  heldAt: string;
}
