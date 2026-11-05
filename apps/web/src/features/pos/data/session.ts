export interface PosSession {
  storeName: string;
  registerName: string;
  cashierName: string;
  /** NIO per USD for today. */
  exchangeRate: number;
  /** Default VAT percent applied to products. */
  taxPercent: number;
  /** Highest discount a cashier can apply without an administrator PIN. */
  discountLimitPercent: number;
  invoiceSeries: string;
  nextInvoiceNumber: number;
}

export const DEMO_SESSION: PosSession = {
  storeName: "Licorería El Barril",
  registerName: "Caja 1",
  cashierName: "Marvin Téllez",
  exchangeRate: 36.6243,
  taxPercent: 15,
  discountLimitPercent: 5,
  invoiceSeries: "F001",
  nextInvoiceNumber: 482,
};
