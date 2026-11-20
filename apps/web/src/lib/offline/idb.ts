/** Minimal promise wrapper over one IndexedDB database with a key-value store and an outbox. */
const DB_NAME = "anejo-offline";
const VERSION = 1;

export type StoreName = "kv" | "outbox";

let opening: Promise<IDBDatabase> | null = null;

function open(): Promise<IDBDatabase> {
  if (typeof indexedDB === "undefined") return Promise.reject(new Error("IndexedDB is not available"));
  opening ??= new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, VERSION);
    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains("kv")) db.createObjectStore("kv");
      if (!db.objectStoreNames.contains("outbox")) db.createObjectStore("outbox", { keyPath: "key" });
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => {
      opening = null;
      reject(request.error);
    };
  });
  return opening;
}

async function run<T>(store: StoreName, mode: IDBTransactionMode, action: (s: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await open();
  return new Promise((resolve, reject) => {
    const request = action(db.transaction(store, mode).objectStore(store));
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

export const idb = {
  get: <T>(store: StoreName, key: string) => run<T | undefined>(store, "readonly", (s) => s.get(key)),
  all: <T>(store: StoreName) => run<T[]>(store, "readonly", (s) => s.getAll()),
  /** Stores value; the kv store needs a key, the outbox reads it from value.key. */
  put: (store: StoreName, value: unknown, key?: string) => run(store, "readwrite", (s) => s.put(value, key)),
  delete: (store: StoreName, key: string) => run(store, "readwrite", (s) => s.delete(key)),
};
