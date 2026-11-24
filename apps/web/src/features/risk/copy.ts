import type { Flag, Level, RiskEvent } from "./api";

const pct = (value: number) => `${(value * 100).toLocaleString("es-NI", { maximumFractionDigits: 1 })} %`;

export const riskCopy = {
  title: "Riesgos de caja",
  description: "Cada cajero comparado con el resto de la tienda. Una marca no acusa a nadie: señala qué revisar primero.",
  period: "Periodo",
  periods: [
    { value: "7", label: "7 días" },
    { value: "30", label: "30 días" },
    { value: "90", label: "90 días" },
  ] as const,
  pct,
  summary: {
    voids: "Ventas anuladas",
    discounts: (limit: number) => `Descuentos cerca o sobre el ${limit} %`,
    shortages: "Cierres con faltante",
    flagged: "Cajeros a revisar",
    flaggedValue: (n: number, total: number) => `${n} de ${total}`,
  },
  columns: {
    cashier: "Cajero",
    sales: "Ventas",
    voids: "Anulaciones",
    discounts: "Descuentos al límite",
    shortages: "Cierres con faltante",
    other: "PIN rechazados · fuera de horario",
  },
  shiftsOf: (short: number, shifts: number) => `${short} de ${shifts}`,
  level: { high: "Revisar ya", medium: "Revisar", low: "Normal" } satisfies Record<Level, string>,
  flags: {
    voids: "Anula más que el resto",
    discounts: "Descuenta al límite más que el resto",
    shortages: "Faltantes frecuentes al cerrar",
    pin: "Varios PIN rechazados",
    outside_hours: "Entró fuera de horario",
  } satisfies Record<Flag, string>,
  storeRate: (rate: string) => `Tienda: ${rate}`,
  empty: { title: "Sin actividad en el periodo", body: "No hay ventas ni cierres de caja en estos días." },
  events: {
    title: "Hechos a revisar",
    subtitle: "Anulaciones, faltantes de C$ 100 o más, descuentos autorizados con PIN, PIN rechazados y entradas fuera de horario.",
    empty: "Nada que revisar en el periodo.",
    more: (n: number) => `Ver los ${n} más recientes`,
    fewer: "Ver menos",
    kinds: {
      void: "Anuló una venta",
      shortage: "Faltante al cerrar caja",
      discount: "Descuento sobre el límite",
      pin: "PIN rechazado",
      outside_hours: "Entró fuera de horario",
    } satisfies Record<RiskEvent["kind"], string>,
    authorizedBy: (detail: string) => {
      const [invoice, who] = detail.split(" · ");
      return `${invoice} · autorizó ${who}`;
    },
    pinOf: (owner: string) => (owner ? `Intentó con el PIN de ${owner}` : "Intentó autorizar con un PIN"),
  },
  how: {
    title: "Cómo se marca",
    items: [
      "Anulaciones: al menos 3 y una tasa 1,5 veces mayor que la de la tienda.",
      "Descuentos: al menos 3 cerca del límite (80 % o más) o autorizados con PIN, 1,5 veces más que la tienda.",
      "Faltantes: al menos 3 cierres con faltante, 1,25 veces más que la tienda.",
      "PIN: 3 o más rechazados. Horario: cualquier entrada fuera del horario de uso.",
      "Dos marcas o más: Revisar ya. Una: Revisar.",
    ],
  },
};
