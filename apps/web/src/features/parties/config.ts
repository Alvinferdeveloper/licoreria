import type { PartyKind } from "./api";

export interface PartyField {
  name: string;
  label: string;
  type?: "text" | "email" | "tel" | "date" | "number";
  required?: boolean;
  hint?: string;
  placeholder?: string;
  mono?: boolean;
  wide?: boolean;
  /** Sent as null when left empty (the API stores no value instead of an empty string). */
  nullable?: boolean;
}

export interface PartyConfig {
  kind: PartyKind;
  permission: string;
  title: string;
  description: string;
  singular: string;
  create: string;
  search: string;
  nameField: string;
  fields: PartyField[];
  /** Secondary line under the name in the table. */
  detail: (row: Record<string, unknown>) => string;
}

export const customersConfig: PartyConfig = {
  kind: "customers",
  permission: "customers",
  title: "Clientes",
  description: "Para facturar a nombre de alguien y ver su historial. En el punto de venta, sin cliente se usa Consumidor final.",
  singular: "cliente",
  create: "Nuevo cliente",
  search: "Buscar por nombre, cédula, RUC o teléfono",
  nameField: "fullName",
  fields: [
    { name: "fullName", label: "Nombre o razón social", required: true, wide: true, placeholder: "Ej. María José Gutiérrez" },
    {
      name: "taxId",
      nullable: true,
      label: "Cédula o RUC",
      mono: true,
      hint: "Necesario para emitir factura a su nombre.",
      placeholder: "001-150390-0012K",
    },
    { name: "phone", nullable: true, label: "Teléfono", type: "tel", placeholder: "8812 3456" },
    { name: "email", nullable: true, label: "Correo", type: "email", hint: "Para enviarle sus facturas.", placeholder: "maria.gutierrez@correo.com" },
    { name: "birthDate", nullable: true, label: "Fecha de nacimiento", type: "date" },
    { name: "address", label: "Dirección", wide: true, placeholder: "Ej. Bolonia, de la Óptica Nicaragüense 1 c. al sur, Managua" },
  ],
  detail: (row) => [row.taxId, row.phone].filter(Boolean).join(" · "),
};

export const suppliersConfig: PartyConfig = {
  kind: "suppliers",
  permission: "suppliers",
  title: "Proveedores",
  description: "Empresas a las que se compra mercadería y sus condiciones de pago.",
  singular: "proveedor",
  create: "Nuevo proveedor",
  search: "Buscar por nombre, RUC o contacto",
  nameField: "name",
  fields: [
    { name: "name", label: "Nombre", required: true, wide: true, placeholder: "Ej. Distribuidora La Fama, S.A." },
    { name: "taxId", nullable: true, label: "RUC", mono: true, placeholder: "J0310000012345" },
    { name: "contactName", label: "Persona de contacto", placeholder: "Ej. Roberto Mendoza" },
    { name: "phone", nullable: true, label: "Teléfono", type: "tel", placeholder: "2250 1234" },
    { name: "email", nullable: true, label: "Correo", type: "email", placeholder: "pedidos@distribuidora.com.ni" },
    { name: "creditDays", label: "Días de crédito", type: "number", hint: "0 si se paga al contado.", placeholder: "Ej. 30" },
    { name: "address", label: "Dirección", wide: true, placeholder: "Ej. Km 7 Carretera Norte, Managua" },
  ],
  detail: (row) => [row.contactName, row.phone].filter(Boolean).join(" · "),
};

export const partyConfigs: Record<PartyKind, PartyConfig> = { customers: customersConfig, suppliers: suppliersConfig };
