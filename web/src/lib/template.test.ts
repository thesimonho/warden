import { describe, expect, it } from 'vitest'

import {
  mergeRuntimeDomains,
  resolveRuntimeEnvVars,
  resolveRuntimeToggles,
  resolveTemplateDomains,
} from '@/lib/template'
import type { ProjectTemplate, RuntimeDefault } from '@/lib/types'

// Runtime domain/envvar fixtures — used in assertions below via domainsFor/envVarsFor.
const pythonDomains = ['pypi.org', 'files.pythonhosted.org']
const goDomains = ['proxy.golang.org', 'sum.golang.org']
const pythonEnvVars = { PIP_CACHE_DIR: '/cache/pip' }
const goEnvVars = { GOPATH: '/home/warden/go', GOMODCACHE: '/cache/go' }

/** Minimal runtime defaults for testing. */
const runtimeDefaults: RuntimeDefault[] = [
  {
    id: 'node',
    label: 'Node.js',
    description: '',
    alwaysEnabled: true,
    detected: false,
    domains: [],
    envVars: {},
  },
  {
    id: 'python',
    label: 'Python',
    description: '',
    alwaysEnabled: false,
    detected: true,
    domains: pythonDomains,
    envVars: pythonEnvVars,
  },
  {
    id: 'go',
    label: 'Go',
    description: '',
    alwaysEnabled: false,
    detected: false,
    domains: goDomains,
    envVars: goEnvVars,
  },
  {
    id: 'rust',
    label: 'Rust',
    description: '',
    alwaysEnabled: false,
    detected: false,
    domains: [],
    envVars: {},
  },
]

/** Helper: returns domains for a given runtime id from the fixture. */
const domainsFor = (id: string) => runtimeDefaults.find((r) => r.id === id)?.domains

/** Helper: returns env var entries for a given runtime id from the fixture. */
const envVarsFor = (id: string) =>
  Object.entries(runtimeDefaults.find((r) => r.id === id)?.envVars ?? {}).map(([key, value]) => ({
    key,
    value,
  }))

describe('resolveRuntimeToggles', () => {
  it('uses detection when no template is provided', () => {
    const toggles = resolveRuntimeToggles(runtimeDefaults)
    expect(toggles).toEqual({
      node: true, // alwaysEnabled
      python: true, // detected
      go: false,
      rust: false,
    })
  })

  it('uses detection when template is null', () => {
    const toggles = resolveRuntimeToggles(runtimeDefaults, null)
    expect(toggles).toEqual({
      node: true,
      python: true,
      go: false,
      rust: false,
    })
  })

  it('uses detection when template has no runtimes field', () => {
    const template: ProjectTemplate = { networkMode: 'restricted' }
    const toggles = resolveRuntimeToggles(runtimeDefaults, template)
    expect(toggles).toEqual({
      node: true,
      python: true,
      go: false,
      rust: false,
    })
  })

  it('overrides detection with template runtimes', () => {
    const template: ProjectTemplate = { runtimes: ['node', 'go', 'rust'] }
    const toggles = resolveRuntimeToggles(runtimeDefaults, template)
    expect(toggles).toEqual({
      node: true, // in template
      python: false, // detected but NOT in template
      go: true, // in template (not detected)
      rust: true, // in template (not detected)
    })
  })

  it('keeps alwaysEnabled runtimes even if not in template', () => {
    const template: ProjectTemplate = { runtimes: ['python'] }
    const toggles = resolveRuntimeToggles(runtimeDefaults, template)
    expect(toggles).toEqual({
      node: true, // alwaysEnabled overrides template
      python: true, // in template
      go: false,
      rust: false,
    })
  })

  it('handles empty template runtimes array', () => {
    const template: ProjectTemplate = { runtimes: [] }
    const toggles = resolveRuntimeToggles(runtimeDefaults, template)
    expect(toggles).toEqual({
      node: true, // alwaysEnabled
      python: false, // empty template disables all non-required
      go: false,
      rust: false,
    })
  })
})

describe('resolveTemplateDomains', () => {
  it('returns undefined when template is null', () => {
    expect(resolveTemplateDomains(null, 'claude-code')).toBeUndefined()
  })

  it('returns undefined when networkMode is not restricted', () => {
    const template: ProjectTemplate = {
      networkMode: 'none',
      agents: { 'claude-code': { allowedDomains: ['*.anthropic.com'] } },
    }
    expect(resolveTemplateDomains(template, 'claude-code')).toBeUndefined()
  })

  it('returns domains for matching agent type when restricted', () => {
    const template: ProjectTemplate = {
      networkMode: 'restricted',
      agents: {
        'claude-code': { allowedDomains: ['*.anthropic.com', '*.github.com'] },
        codex: { allowedDomains: ['*.openai.com'] },
      },
    }
    expect(resolveTemplateDomains(template, 'claude-code')).toEqual([
      '*.anthropic.com',
      '*.github.com',
    ])
    expect(resolveTemplateDomains(template, 'codex')).toEqual(['*.openai.com'])
  })

  it('returns undefined for agent type not in template', () => {
    const template: ProjectTemplate = {
      networkMode: 'restricted',
      agents: { codex: { allowedDomains: ['*.openai.com'] } },
    }
    expect(resolveTemplateDomains(template, 'claude-code')).toBeUndefined()
  })

  it('returns undefined when template has no agents', () => {
    const template: ProjectTemplate = { networkMode: 'restricted' }
    expect(resolveTemplateDomains(template, 'claude-code')).toBeUndefined()
  })
})

describe('resolveRuntimeEnvVars', () => {
  it('returns env vars for enabled runtimes', () => {
    const toggles = { node: true, python: true, go: false, rust: false }
    const envVars = resolveRuntimeEnvVars(runtimeDefaults, toggles)
    expect(envVars).toEqual(envVarsFor('python'))
  })

  it('returns env vars for multiple enabled runtimes', () => {
    const toggles = { node: true, python: true, go: true, rust: false }
    const envVars = resolveRuntimeEnvVars(runtimeDefaults, toggles)
    expect(envVars).toEqual([...envVarsFor('python'), ...envVarsFor('go')])
  })

  it('returns empty array when no runtimes have env vars', () => {
    const toggles = { node: true, python: false, go: false, rust: true }
    const envVars = resolveRuntimeEnvVars(runtimeDefaults, toggles)
    expect(envVars).toEqual([])
  })

  it('returns empty array when no runtimes are enabled', () => {
    const toggles = { node: false, python: false, go: false, rust: false }
    const envVars = resolveRuntimeEnvVars(runtimeDefaults, toggles)
    expect(envVars).toEqual([])
  })

  it('works with template-resolved toggles', () => {
    const template: ProjectTemplate = { runtimes: ['node', 'go'] }
    const toggles = resolveRuntimeToggles(runtimeDefaults, template)
    const envVars = resolveRuntimeEnvVars(runtimeDefaults, toggles)
    // python detected but not in template → disabled → no env vars
    // go in template → enabled → go env vars
    expect(envVars).toEqual(envVarsFor('go'))
  })
})

describe('mergeRuntimeDomains', () => {
  it('adds domains from enabled runtimes', () => {
    const base = ['*.anthropic.com', '*.github.com']
    const toggles = { node: true, python: true, go: false, rust: false }
    const result = mergeRuntimeDomains(base, runtimeDefaults, toggles)
    expect(result).toEqual([...base, ...(domainsFor('python') ?? [])])
  })

  it('deduplicates domains already in base list', () => {
    const base = ['*.anthropic.com', pythonDomains[0]]
    const toggles = { node: true, python: true, go: false, rust: false }
    const result = mergeRuntimeDomains(base, runtimeDefaults, toggles)
    expect(result).toEqual([...base, ...pythonDomains.slice(1)])
  })

  it('returns base domains unchanged when no runtimes have domains', () => {
    const base = ['*.anthropic.com']
    const toggles = { node: true, python: false, go: false, rust: true }
    const result = mergeRuntimeDomains(base, runtimeDefaults, toggles)
    expect(result).toEqual(base)
  })

  it('merges domains from multiple runtimes', () => {
    const base = ['*.anthropic.com']
    const toggles = { node: true, python: true, go: true, rust: false }
    const result = mergeRuntimeDomains(base, runtimeDefaults, toggles)
    expect(result).toEqual([...base, ...(domainsFor('python') ?? []), ...(domainsFor('go') ?? [])])
  })

  it('works end-to-end with template-resolved toggles', () => {
    const templateDomains = ['*.anthropic.com', '*.github.com']
    const template: ProjectTemplate = {
      networkMode: 'restricted',
      runtimes: ['node', 'go'],
      agents: { 'claude-code': { allowedDomains: templateDomains } },
    }
    const toggles = resolveRuntimeToggles(runtimeDefaults, template)
    const domains = resolveTemplateDomains(template, 'claude-code')!
    const result = mergeRuntimeDomains(domains, runtimeDefaults, toggles)
    // go in template → go domains merged; python detected but not in template → excluded
    expect(result).toEqual([...templateDomains, ...(domainsFor('go') ?? [])])
  })
})
