/** Reads JSON from localStorage, returning `fallback` on any error. */
export function readStorage<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key)
    if (raw) return JSON.parse(raw) as T
  } catch {
    /* ignore corrupt data */
  }
  return fallback
}

/** Writes a value to localStorage as JSON. */
export function writeStorage<T>(key: string, value: T): void {
  localStorage.setItem(key, JSON.stringify(value))
}

/** Reads a Set from localStorage (stored as a JSON array). */
export function readStoredSet<T extends string>(key: string, fallback: readonly T[]): Set<T> {
  const parsed = readStorage<T[]>(key, [])
  return parsed.length > 0 ? new Set(parsed) : new Set(fallback)
}

/** Writes a Set to localStorage as a JSON array. */
export function writeStoredSet<T>(key: string, set: Set<T>): void {
  writeStorage(key, [...set])
}
