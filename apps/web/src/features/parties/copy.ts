/** Texts shared by customers and suppliers; each kind's own texts live in config.ts. */
export const partiesCopy = {
  columns: { name: "Nombre", email: "Correo", status: "Estado", actions: "Acciones" },
  noContact: "Sin datos de contacto",
  status: { walkIn: "Predeterminado", active: "Activo", inactive: "Inactivo" },
  statusFilter: "Estado",
  actions: {
    edit: (name: string) => `Editar ${name}`,
    deactivate: "Desactivar",
    activate: "Activar",
    confirm: "¿Confirmar?",
  },
  empty: (singular: string) => ({ title: "Sin resultados", body: `Ningún ${singular} coincide con la búsqueda.` }),
  editTitle: (singular: string) => `Editar ${singular}`,
  notices: {
    toggled: (name: string, active: boolean) => `${name} ${active ? "se activó" : "se desactivó"}.`,
    saved: (name: string) => `${name} se guardó.`,
  },
  form: { required: "Este campo es obligatorio.", cancel: "Cancelar", save: "Guardar" },
} as const;
