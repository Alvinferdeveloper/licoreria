import { toCents } from "@/lib/money";
import type { Category, Product } from "../domain";
import { DEMO_SESSION } from "./session";

export const CATEGORIES: readonly Category[] = [
  { id: "rum", label: "Ron" },
  { id: "beer", label: "Cerveza" },
  { id: "whisky", label: "Whisky" },
  { id: "vodka", label: "Vodka y ginebra" },
  { id: "wine", label: "Vino" },
  { id: "mixers", label: "Mezcladores y más" },
];

type Seed = [sku: string, barcode: string, name: string, brand: string, Product["categoryId"], presentation: string, price: number, stock: number, minStock: number];

const SEEDS: readonly Seed[] = [
  ["RON-FC4-750", "7430011001042", "Flor de Caña 4 años Extra Lite", "Flor de Caña", "rum", "750 ml", 218.5, 46, 12],
  ["RON-FC7-750", "7430011001073", "Flor de Caña 7 años Gran Reserva", "Flor de Caña", "rum", "750 ml", 421.75, 31, 10],
  ["RON-FC12-750", "7430011001127", "Flor de Caña 12 años Centenario", "Flor de Caña", "rum", "750 ml", 856, 9, 6],
  ["RON-FC18-750", "7430011001189", "Flor de Caña 18 años Centenario", "Flor de Caña", "rum", "750 ml", 1647.8, 4, 3],
  ["RON-PLA-1L", "7430019002011", "Ron Plata Blanco", "Ron Plata", "rum", "1 L", 179.15, 58, 15],
  ["RON-FC4-1750", "7430011001745", "Flor de Caña 4 años Extra Seco", "Flor de Caña", "rum", "1.75 L", 468.7, 0, 4],
  ["CER-TON-355", "7430020003550", "Toña lata", "Toña", "beer", "355 ml", 31.3, 312, 96],
  ["CER-TON-6P", "7430020003567", "Toña six-pack", "Toña", "beer", "6 × 355 ml", 182.6, 41, 12],
  ["CER-TON-24", "7430020003574", "Toña caja", "Toña", "beer", "24 × 355 ml", 709.5, 8, 6],
  ["CER-VIC-355", "7430020104359", "Victoria Clásica lata", "Victoria", "beer", "355 ml", 31.3, 287, 96],
  ["CER-VFR-355", "7430020114358", "Victoria Frost lata", "Victoria", "beer", "355 ml", 33.9, 64, 72],
  ["CER-HEI-355", "8712000003552", "Heineken botella", "Heineken", "beer", "355 ml", 47.85, 118, 48],
  ["WHI-JWR-750", "5000267014005", "Johnnie Walker Red Label", "Johnnie Walker", "whisky", "750 ml", 782.6, 17, 6],
  ["WHI-JWB-750", "5000267024004", "Johnnie Walker Black Label 12 años", "Johnnie Walker", "whisky", "750 ml", 1452.2, 7, 4],
  ["WHI-BUC-750", "5000196001107", "Buchanan's Deluxe 12 años", "Buchanan's", "whisky", "750 ml", 1496.5, 2, 4],
  ["WHI-OPA-750", "5000281005416", "Old Parr 12 años", "Old Parr", "whisky", "750 ml", 1348.7, 11, 4],
  ["VOD-ABS-750", "7312040017034", "Absolut Vodka", "Absolut", "vodka", "750 ml", 716.5, 13, 5],
  ["VOD-SMI-750", "0082000000051", "Smirnoff No. 21", "Smirnoff", "vodka", "750 ml", 428.7, 22, 8],
  ["GIN-TAN-750", "5000291020706", "Tanqueray London Dry", "Tanqueray", "vodka", "750 ml", 893.9, 6, 3],
  ["VIN-CDD-750", "7804320559001", "Casillero del Diablo Cabernet Sauvignon", "Concha y Toro", "wine", "750 ml", 478.25, 19, 6],
  ["VIN-GAT-750", "7804300120160", "Gato Negro Merlot", "San Pedro", "wine", "750 ml", 261.7, 27, 8],
  ["MIX-COC-2L", "7441029500028", "Coca-Cola", "Coca-Cola", "mixers", "2 L", 64.35, 73, 24],
  ["MIX-AFP-600", "7441008100607", "Agua Fuente Pura", "Fuente Pura", "mixers", "600 ml", 17.4, 140, 48],
  ["MIX-HIE-5LB", "7430099000051", "Hielo en bolsa", "Hielo Polar", "mixers", "5 lb", 39.15, 35, 20],
];

export const PRODUCTS: readonly Product[] = SEEDS.map(
  ([sku, barcode, name, brand, categoryId, presentation, price, stock, minStock], index) => ({
    id: `prd_${String(index + 1).padStart(3, "0")}`,
    sku,
    barcode,
    name,
    brand,
    categoryId,
    presentation,
    price: toCents(price),
    taxPercent: DEMO_SESSION.taxPercent,
    stock,
    minStock,
  }),
);
