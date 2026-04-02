/**
 * IndexedDB storage for terminal scrollback buffers.
 *
 * Persists serialized xterm.js buffer state across disconnect/reconnect cycles
 * and page reloads. Each entry is keyed by `${projectId}:${worktreeId}` so it
 * survives container recreation (projectId is a deterministic hash of the host
 * path, not the container ID).
 *
 * All operations silently catch errors and post debug events to the audit log.
 * IndexedDB can fail in private browsing or when storage is full — scrollback
 * persistence is a quality-of-life feature, never a hard requirement.
 *
 * @module
 */
import { postAuditEvent } from '@/lib/api'

const DB_NAME = 'warden-scrollback'
const DB_VERSION = 1
const STORE_NAME = 'buffers'
const PROJECT_INDEX = 'by-project'

/** Shape of a persisted scrollback entry. */
export interface ScrollbackEntry {
  /** Composite key: `${projectId}:${worktreeId}`. */
  key: string
  /** Stable project identifier (12-char hex SHA-256 of host path). */
  projectId: string
  /** Worktree identifier (e.g. "main", "feature-x"). */
  worktreeId: string
  /** Serialized ANSI escape sequence string from @xterm/addon-serialize. */
  data: string
  /** Terminal column count at save time (for mismatch detection). */
  cols: number
  /** Terminal row count at save time. */
  rows: number
  /** Timestamp (Date.now()) for TTL-based eviction. */
  savedAt: number
}

/** Lazy singleton — opened once, reused for all operations. */
let dbPromise: Promise<IDBDatabase> | null = null

/** Opens (or creates) the scrollback database. */
function openDB(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise

  dbPromise = new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)

    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        const store = db.createObjectStore(STORE_NAME, { keyPath: 'key' })
        store.createIndex(PROJECT_INDEX, 'projectId', { unique: false })
      }
    }

    request.onsuccess = () => resolve(request.result)
    request.onerror = () => {
      dbPromise = null
      reject(request.error)
    }
  })

  return dbPromise
}

/** Posts an IndexedDB error to the audit log at debug level. */
function logError(operation: string, error: unknown): void {
  const message = error instanceof Error ? error.message : String(error)
  postAuditEvent({
    event: 'scrollback_db_error',
    level: 'debug',
    message: `${operation}: ${message}`,
  }).catch(() => {
    // Audit endpoint itself failed — nothing more we can do.
  })
}

/** Persists a scrollback buffer entry. */
export async function saveScrollback(
  key: string,
  projectId: string,
  worktreeId: string,
  data: string,
  cols: number,
  rows: number,
): Promise<void> {
  try {
    const db = await openDB()
    const entry: ScrollbackEntry = {
      key,
      projectId,
      worktreeId,
      data,
      cols,
      rows,
      savedAt: Date.now(),
    }
    const tx = db.transaction(STORE_NAME, 'readwrite')
    tx.objectStore(STORE_NAME).put(entry)
    await txComplete(tx)
  } catch (error) {
    logError('saveScrollback', error)
  }
}

/** Loads a scrollback buffer entry, or undefined if not found. */
export async function loadScrollback(key: string): Promise<ScrollbackEntry | undefined> {
  try {
    const db = await openDB()
    const tx = db.transaction(STORE_NAME, 'readonly')
    const request = tx.objectStore(STORE_NAME).get(key)
    return await requestResult<ScrollbackEntry | undefined>(request)
  } catch (error) {
    logError('loadScrollback', error)
    return undefined
  }
}

/** Deletes a single scrollback entry (e.g. when a worktree is killed). */
export async function deleteScrollback(key: string): Promise<void> {
  try {
    const db = await openDB()
    const tx = db.transaction(STORE_NAME, 'readwrite')
    tx.objectStore(STORE_NAME).delete(key)
    await txComplete(tx)
  } catch (error) {
    logError('deleteScrollback', error)
  }
}

/** Deletes all scrollback entries for a project (e.g. when a project is removed). */
export async function deleteProjectScrollback(projectId: string): Promise<void> {
  try {
    const db = await openDB()
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const index = tx.objectStore(STORE_NAME).index(PROJECT_INDEX)
    const request = index.openKeyCursor(IDBKeyRange.only(projectId))

    await new Promise<void>((resolve, reject) => {
      request.onsuccess = () => {
        const cursor = request.result
        if (cursor) {
          tx.objectStore(STORE_NAME).delete(cursor.primaryKey)
          cursor.continue()
        } else {
          resolve()
        }
      }
      request.onerror = () => reject(request.error)
    })

    await txComplete(tx)
  } catch (error) {
    logError('deleteProjectScrollback', error)
  }
}

/** Deletes entries older than `maxAgeMs` milliseconds. */
export async function evictStaleScrollback(maxAgeMs: number): Promise<void> {
  try {
    const cutoff = Date.now() - maxAgeMs
    const db = await openDB()
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    const request = store.openCursor()

    await new Promise<void>((resolve, reject) => {
      request.onsuccess = () => {
        const cursor = request.result
        if (cursor) {
          const entry = cursor.value as ScrollbackEntry
          if (entry.savedAt < cutoff) {
            cursor.delete()
          }
          cursor.continue()
        } else {
          resolve()
        }
      }
      request.onerror = () => reject(request.error)
    })

    await txComplete(tx)
  } catch (error) {
    logError('evictStaleScrollback', error)
  }
}

/** Wraps an IDBTransaction completion as a Promise. */
function txComplete(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

/** Wraps an IDBRequest result as a Promise. */
function requestResult<T>(request: IDBRequest): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result as T)
    request.onerror = () => reject(request.error)
  })
}
