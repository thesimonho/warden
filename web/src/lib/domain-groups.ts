/**
 * Default allowed domains for the restricted network mode.
 *
 * This is the minimum useful set for AI coding agent development:
 * Anthropic API, OpenAI API, GitHub, and common package registries.
 * Wildcard patterns (e.g. *.github.com) are resolved at container start.
 */
export const restrictedDomains: readonly string[] = [
  '*.anthropic.com',
  '*.openai.com',
  '*.github.com',
  '*.githubusercontent.com',
  'pypi.org',
  'files.pythonhosted.org',
  'registry.npmjs.org',
  'registry.yarnpkg.com',
  'go.dev',
  'proxy.golang.org',
  'sum.golang.org',
] as const
