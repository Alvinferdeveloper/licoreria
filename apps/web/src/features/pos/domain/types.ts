import type { Cents, CurrencyCode } from "@/lib/money";

export type CategoryId = "rum" | "beer" | "whisky" | "vodka" | "wine" | "mixers";

export interface Category {
  id: CategoryId;
  label: string;
}

export interface Product {
  id: string;
  sku: string;
  barcode: string;
  name: string;
  brand: string;
  categoryId: CategoryId;
  presentation: string;
  /** Unit price before tax, in NIO cents. */
  price: Cents;
  taxPercent: number;
  stock: number;
  minStock: number;
}

export interface CartLine {
  productId: string;
  name: string;
  presentation: string;
  unitPrice: Cents;
  taxPercent: number;
  quantity: number;
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
  customer: Customer | null;
  heldAt: string;
}
