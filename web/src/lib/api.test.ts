import { describe, it, expect, vi, afterEach } from 'vitest'
import {
  createWorktree,
  connectTerminal,
  disconnectTerminal,
  removeProject,
  deleteContainer,
  validateContainer,
  fetchWorktreeDiff,
  worktreeHostPath,
} from '@/lib/api'

/** Captures the most recent fetch call's arguments. */
function mockFetchOk(body: unknown) {
  return vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(body),
  })
}

function mockFetchError(status: number, statusText: string) {
  return vi.fn().mockResolvedValue({
    ok: false,
    status,
    statusText,
    json: () => Promise.reject(new Error('no body')),
  })
}

describe('worktreeHostPath', () => {
  it('maps workspace root to host dir', () => {
    const result = worktreeHostPath('/home/simon/myapp', '/home/dev/myapp', '/home/dev/myapp')
    expect(result).toBe('/home/simon/myapp')
  })

  it('maps worktree subpath to host dir', () => {
    const result = worktreeHostPath(
      '/home/simon/myapp',
      '/home/dev/myapp/.claude/worktrees/feature-x',
      '/home/dev/myapp',
    )
    expect(result).toBe('/home/simon/myapp/.claude/worktrees/feature-x')
  })

  it('works with legacy /project workspace dir', () => {
    const result = worktreeHostPath('/home/simon/myapp', '/project', '/project')
    expect(result).toBe('/home/simon/myapp')
  })

  it('maps legacy /project worktree to host dir', () => {
    const result = worktreeHostPath(
      '/home/simon/myapp',
      '/project/.claude/worktrees/feat',
      '/project',
    )
    expect(result).toBe('/home/simon/myapp/.claude/worktrees/feat')
  })

  it('falls back to raw path when no prefix matches', () => {
    const result = worktreeHostPath('/home/simon/myapp', '/some/other/path', '/home/dev/myapp')
    expect(result).toBe('/home/simon/myapp/some/other/path')
  })
})

describe('createWorktree', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends POST to the correct endpoint', async () => {
    const fetchMock = mockFetchOk({ worktreeId: 'feature-x', projectId: 'proj-1' })
    vi.stubGlobal('fetch', fetchMock)

    await createWorktree('proj-1', 'feature-x')

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/projects/proj-1/worktrees')
    expect(options.method).toBe('POST')
  })

  it('sends name in request body', async () => {
    const fetchMock = mockFetchOk({ worktreeId: 'fix-bug', projectId: 'proj-1' })
    vi.stubGlobal('fetch', fetchMock)

    await createWorktree('proj-1', 'fix-bug')

    const body = JSON.parse(fetchMock.mock.calls[0][1].body)
    expect(body).toEqual({ name: 'fix-bug' })
  })

  it('returns worktree result from response', async () => {
    const fetchMock = mockFetchOk({ worktreeId: 'feature-x', projectId: 'proj-1' })
    vi.stubGlobal('fetch', fetchMock)

    const result = await createWorktree('proj-1', 'feature-x')

    expect(result).toEqual({ worktreeId: 'feature-x', projectId: 'proj-1' })
  })

  it('throws on non-ok response', async () => {
    const fetchMock = mockFetchError(500, 'Internal Server Error')
    vi.stubGlobal('fetch', fetchMock)

    await expect(createWorktree('proj-1', 'feature-x')).rejects.toThrow()
  })

  it('sets Content-Type to application/json', async () => {
    const fetchMock = mockFetchOk({ worktreeId: 'feature-x', projectId: 'proj-1' })
    vi.stubGlobal('fetch', fetchMock)

    await createWorktree('proj-1', 'feature-x')

    const headers = fetchMock.mock.calls[0][1].headers
    expect(headers['Content-Type']).toBe('application/json')
  })
})

describe('connectTerminal', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends POST to the correct endpoint', async () => {
    const fetchMock = mockFetchOk({ worktreeId: 'main', projectId: 'proj-1' })
    vi.stubGlobal('fetch', fetchMock)

    await connectTerminal('proj-1', 'main')

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/projects/proj-1/worktrees/main/connect')
    expect(options.method).toBe('POST')
  })

  it('returns worktree result from response', async () => {
    const fetchMock = mockFetchOk({ worktreeId: 'main', projectId: 'proj-1' })
    vi.stubGlobal('fetch', fetchMock)

    const result = await connectTerminal('proj-1', 'main')

    expect(result).toEqual({ worktreeId: 'main', projectId: 'proj-1' })
  })

  it('throws on non-ok response', async () => {
    const fetchMock = mockFetchError(500, 'Internal Server Error')
    vi.stubGlobal('fetch', fetchMock)

    await expect(connectTerminal('proj-1', 'main')).rejects.toThrow()
  })
})

describe('disconnectTerminal', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends POST to the correct endpoint', async () => {
    const fetchMock = mockFetchOk({ worktreeId: 'main', projectId: 'proj-1' })
    vi.stubGlobal('fetch', fetchMock)

    await disconnectTerminal('proj-1', 'main')

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/projects/proj-1/worktrees/main/disconnect')
    expect(options.method).toBe('POST')
  })

  it('returns worktree result from response', async () => {
    const fetchMock = mockFetchOk({ worktreeId: 'main', projectId: 'proj-1' })
    vi.stubGlobal('fetch', fetchMock)

    const result = await disconnectTerminal('proj-1', 'main')

    expect(result).toEqual({ worktreeId: 'main', projectId: 'proj-1' })
  })

  it('throws on non-ok response', async () => {
    const fetchMock = mockFetchError(404, 'Not Found')
    vi.stubGlobal('fetch', fetchMock)

    await expect(disconnectTerminal('proj-1', 'main')).rejects.toThrow()
  })
})

describe('removeProject', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends DELETE to the correct endpoint', async () => {
    const fetchMock = mockFetchOk({ name: 'my-project' })
    vi.stubGlobal('fetch', fetchMock)

    await removeProject('abc123def456')

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/projects/abc123def456')
    expect(options.method).toBe('DELETE')
  })

  it('returns project result from response', async () => {
    const fetchMock = mockFetchOk({ name: 'my-project' })
    vi.stubGlobal('fetch', fetchMock)

    const result = await removeProject('abc123def456')

    expect(result).toEqual({ name: 'my-project' })
  })

  it('throws on non-ok response', async () => {
    const fetchMock = mockFetchError(500, 'Internal Server Error')
    vi.stubGlobal('fetch', fetchMock)

    await expect(removeProject('abc123def456')).rejects.toThrow()
  })
})

describe('deleteContainer', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends DELETE to the correct endpoint', async () => {
    const fetchMock = mockFetchOk({ containerId: 'abc123def456', name: 'my-container' })
    vi.stubGlobal('fetch', fetchMock)

    await deleteContainer('proj-1')

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/projects/proj-1/container')
    expect(options.method).toBe('DELETE')
  })

  it('returns container result from response', async () => {
    const fetchMock = mockFetchOk({ containerId: 'abc123def456', name: 'my-container' })
    vi.stubGlobal('fetch', fetchMock)

    const result = await deleteContainer('proj-1')

    expect(result).toEqual({ containerId: 'abc123def456', name: 'my-container' })
  })

  it('throws on non-ok response', async () => {
    const fetchMock = mockFetchError(500, 'Internal Server Error')
    vi.stubGlobal('fetch', fetchMock)

    await expect(deleteContainer('proj-1')).rejects.toThrow()
  })
})

describe('validateContainer', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends GET to the correct endpoint', async () => {
    const fetchMock = mockFetchOk({ valid: true, missing: null })
    vi.stubGlobal('fetch', fetchMock)

    await validateContainer('proj-1')

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/projects/proj-1/container/validate')
  })

  it('returns validation result with valid container', async () => {
    const fetchMock = mockFetchOk({ valid: true, missing: null })
    vi.stubGlobal('fetch', fetchMock)

    const result = await validateContainer('proj-1')

    expect(result).toEqual({ valid: true, missing: null })
  })

  it('returns missing binaries for invalid container', async () => {
    const fetchMock = mockFetchOk({
      valid: false,
      missing: ['/usr/local/bin/ttyd', '/usr/local/bin/create-terminal.sh'],
    })
    vi.stubGlobal('fetch', fetchMock)

    const result = await validateContainer('proj-1')

    expect(result.valid).toBe(false)
    expect(result.missing).toHaveLength(2)
  })

  it('throws on non-ok response', async () => {
    const fetchMock = mockFetchError(500, 'Internal Server Error')
    vi.stubGlobal('fetch', fetchMock)

    await expect(validateContainer('proj-1')).rejects.toThrow()
  })
})

describe('fetchWorktreeDiff', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends GET to the correct endpoint', async () => {
    const fetchMock = mockFetchOk({
      files: [],
      rawDiff: '',
      totalAdditions: 0,
      totalDeletions: 0,
      truncated: false,
    })
    vi.stubGlobal('fetch', fetchMock)

    await fetchWorktreeDiff('proj-1', 'main')

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/projects/proj-1/worktrees/main/diff')
  })

  it('returns diff response from response', async () => {
    const body = {
      files: [
        { path: 'main.go', additions: 10, deletions: 2, isBinary: false, status: 'modified' },
      ],
      rawDiff: 'diff --git a/main.go b/main.go\n',
      totalAdditions: 10,
      totalDeletions: 2,
      truncated: false,
    }
    const fetchMock = mockFetchOk(body)
    vi.stubGlobal('fetch', fetchMock)

    const result = await fetchWorktreeDiff('proj-1', 'main')

    expect(result.files).toHaveLength(1)
    expect(result.totalAdditions).toBe(10)
  })

  it('throws on non-ok response', async () => {
    const fetchMock = mockFetchError(500, 'Internal Server Error')
    vi.stubGlobal('fetch', fetchMock)

    await expect(fetchWorktreeDiff('proj-1', 'main')).rejects.toThrow()
  })
})
