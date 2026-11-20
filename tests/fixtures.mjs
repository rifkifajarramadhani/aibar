import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'

export function records(now = new Date()) {
  const day = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const key = date => [date.getFullYear(), date.getMonth() + 1, date.getDate()]
    .map((part, i) => String(part).padStart(i === 0 ? 4 : 2, '0')).join('-')
  const recentDays = Array.from({ length: 7 }, (_, i) => ({
    date: key(new Date(day.getFullYear(), day.getMonth(), day.getDate() - 6 + i)),
    messageCount: [0, 900, 125000, 41000, 5000, 78900, 34219][i]
  }))
  const updatedAt = now.toISOString()
  const shortReset = new Date(+now + 120000).toISOString()
  const weekReset = new Date(+now + 3 * 86400000).toISOString()
  const record = (id, name, fields) => ({ schemaVersion: 1, id, name, updatedAt, ...fields })

  return {
    codex: record('codex', 'Codex', {
      ready: true,
      tierLabel: 'Pro',
      limits: [
        { label: '5h window', percent: 0.37, resetsAt: shortReset },
        { label: '7-day', title: 'Weekly', percent: 0.91, resetsAt: weekReset }
      ],
      limitsFetchedAt: +now,
      resetCredits: { available: 2 },
      hasLocalStats: true,
      todayTotalTokens: 34219,
      todayPrompts: 12,
      todaySessions: 3,
      recentDays,
      accounts: [{
        id: 'work',
        label: 'Work',
        email: 'work@example.invalid',
        plan: 'Business',
        active: true,
        limits: [{ label: 'Weekly', percent: 0.7, resetsAt: weekReset }]
      }]
    }),
    claude: record('claude', 'Claude Code', {
      ready: true,
      tierLabel: 'Max',
      limits: [
        { label: 'Session (5-hour)', percent: 0.26, resetsAt: shortReset },
        { label: 'Weekly', percent: 0.8, resetsAt: weekReset }
      ],
      limitsStale: true,
      limitsFetchedAt: +now - 3600000,
      usageStatusText: 'Limits paused',
      authHelpText: '<img src="https://example.invalid/test">'
    }),
    fireworks: record('fireworks', 'Fireworks', {
      tierLabel: 'Prepaid',
      balance: { remaining: 8.5, spent: 91.5, funded: 100, currency: 'USD', estimated: true }
    }),
    hermes: record('hermes', 'Hermes', {
      hasLocalStats: true,
      hasPromptStats: false,
      todayTotalTokens: 34219,
      todayPrompts: 0,
      todaySessions: 2,
      recentDays
    }),
    openrouter: record('openrouter', 'OpenRouter', {
      usageStatusText: 'Waiting for auth',
      authHelpText: 'Sign in to OpenRouter in Omarchy.'
    }),
    unused: record('unused', 'Unused', { ready: false, recentDays: [] })
  }
}

export function writeFixtures(directory) {
  mkdirSync(directory, { recursive: true })
  for (const [id, data] of Object.entries(records())) {
    writeFileSync(join(directory, id + '.json'), JSON.stringify(data))
  }
  writeFileSync(join(directory, 'malformed.json'), '{"schemaVersion":1,')
}

if (process.argv[2] && import.meta.url === pathToFileURL(process.argv[1]).href) writeFixtures(process.argv[2])
