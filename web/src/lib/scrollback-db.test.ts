import 'fake-indexeddb/auto'
import { describe, it, expect } from 'vitest'
import {
  saveScrollback,
  loadScrollback,
  deleteScrollback,
  deleteProjectScrollback,
  evictStaleScrollback,
  scrollbackKey,
} from '@/lib/scrollback-db'

// Tests share a single IndexedDB instance via the module's lazy singleton.
// Each test uses unique keys to avoid interference.

describe('scrollbackKey', () => {
  it('produces projectId:worktreeId format', () => {
    expect(scrollbackKey('abc123def456', 'main')).toBe('abc123def456:main')
  })

  it('handles worktree IDs with special characters', () => {
    expect(scrollbackKey('abc123def456', 'feature-branch.v2')).toBe(
      'abc123def456:feature-branch.v2',
    )
  })
})

describe('saveScrollback + loadScrollback', () => {
  it('round-trips a scrollback entry', async () => {
    await saveScrollback('rt', 'main', '\x1b[32mhello\x1b[0m', 80, 24)

    const entry = await loadScrollback('rt:main')
    expect(entry).toBeDefined()
    expect(entry!.key).toBe('rt:main')
    expect(entry!.projectId).toBe('rt')
    expect(entry!.worktreeId).toBe('main')
    expect(entry!.data).toBe('\x1b[32mhello\x1b[0m')
    expect(entry!.cols).toBe(80)
    expect(entry!.rows).toBe(24)
    expect(entry!.savedAt).toBeGreaterThan(0)
  })

  it('returns undefined for missing key', async () => {
    const entry = await loadScrollback('nonexistent:key')
    expect(entry).toBeUndefined()
  })

  it('overwrites existing entry with same key', async () => {
    await saveScrollback('ow', 'main', 'first', 80, 24)
    await saveScrollback('ow', 'main', 'second', 120, 40)

    const entry = await loadScrollback('ow:main')
    expect(entry!.data).toBe('second')
    expect(entry!.cols).toBe(120)
  })
})

describe('deleteScrollback', () => {
  it('removes a single entry by key', async () => {
    await saveScrollback('del', 'main', 'data', 80, 24)
    await deleteScrollback('del:main')

    const entry = await loadScrollback('del:main')
    expect(entry).toBeUndefined()
  })

  it('does not throw for missing key', async () => {
    await expect(deleteScrollback('missing:key')).resolves.toBeUndefined()
  })
})

describe('deleteProjectScrollback', () => {
  it('removes all entries for a project', async () => {
    await saveScrollback('dp', 'main', 'a', 80, 24)
    await saveScrollback('dp', 'feat', 'b', 80, 24)
    await saveScrollback('other', 'main', 'c', 80, 24)

    await deleteProjectScrollback('dp')

    expect(await loadScrollback('dp:main')).toBeUndefined()
    expect(await loadScrollback('dp:feat')).toBeUndefined()
    expect(await loadScrollback('other:main')).toBeDefined()
  })
})

describe('evictStaleScrollback', () => {
  it('removes entries older than the TTL', async () => {
    // Save an entry, then manually backdate it via direct IndexedDB access.
    await saveScrollback('ev', 'old', 'stale', 80, 24)

    // DB_VERSION bumped to 2 for the savedAt index.
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const req = indexedDB.open('warden-scrollback', 2)
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => reject(req.error)
    })
    const tx = db.transaction('buffers', 'readwrite')
    const store = tx.objectStore('buffers')
    const entry = await new Promise<Record<string, unknown>>((resolve, reject) => {
      const req = store.get('ev:old')
      req.onsuccess = () => resolve(req.result as Record<string, unknown>)
      req.onerror = () => reject(req.error)
    })
    entry.savedAt = Date.now() - 8 * 24 * 60 * 60 * 1000 // 8 days ago
    store.put(entry)
    await new Promise<void>((resolve) => {
      tx.oncomplete = () => resolve()
    })
    db.close()

    // Save a fresh entry that should survive eviction.
    await saveScrollback('ev', 'new', 'fresh', 80, 24)

    const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000
    await evictStaleScrollback(SEVEN_DAYS_MS)

    expect(await loadScrollback('ev:old')).toBeUndefined()
    expect(await loadScrollback('ev:new')).toBeDefined()
  })
})
