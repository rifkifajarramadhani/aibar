import assert from 'node:assert/strict'
import { test } from 'node:test'
import { parse, history, countdown } from '../widget/Records.mjs'
import { listing } from '../widget/OmarchyFiles.mjs'
import { records } from './fixtures.mjs'

const now = new Date(2026, 10, 20, 23, 58)
const fixtures = records(now)
const provider = id => parse(JSON.stringify(fixtures[id]), id).record

test('reads Codex and Claude windows, stale timestamps, and read-only account fields', () => {
  const codex = provider('codex')
  const claude = provider('claude')
  assert.deepEqual(codex.limits.map(limit => limit.percent), [0.37, 0.91])
  assert.equal(codex.limits[1].label, 'Weekly')
  assert.equal(codex.plan, 'Pro')
  assert.equal(codex.resetCredits, 2)
  assert.equal(codex.accounts[0].plan, 'Business')
  assert.equal(codex.accounts[0].email, 'work@example.invalid')
  assert.equal(codex.accounts[0].active, true)
  assert.deepEqual(claude.limits.map(limit => limit.percent), [0.26, 0.8])
  assert.equal(claude.stale, true)
  assert.equal(claude.fetched, +now - 3600000)
  assert.equal(claude.help, '<img src="https://example.invalid/test">')
})

test('discovers prepaid, activity-only, and signed-out providers', () => {
  assert.deepEqual(provider('fireworks').balance, {
    remaining: 8.5, funded: 100, spent: 91.5, currency: 'USD', estimated: true
  })
  assert.equal(provider('hermes').hasActivity, true)
  assert.equal(provider('hermes').todayPrompts, null)
  assert.equal(provider('hermes').todaySessions, null)
  assert.deepEqual(provider('hermes').limits, [])
  assert.equal(provider('openrouter').status, 'Waiting for auth')
  assert.equal(provider('unused'), null)
})

test('keeps seven local calendar days and distinguishes zero from missing data', () => {
  const record = provider('codex')
  assert.deepEqual(history(record, +now).map(day => day.tokens), [0, 900, 125000, 41000, 5000, 78900, 34219])
  const tomorrow = new Date(2026, 10, 21, 0, 1)
  assert.deepEqual(history(record, +tomorrow).map(day => day.tokens), [900, 125000, 41000, 5000, 78900, 34219, null])
})

test('advances the reset countdown without predicting usage', () => {
  const reset = provider('codex').limits[0].reset
  assert.deepEqual(countdown(reset, +now), { seconds: 120, minutes: 2, hours: 0, days: 0 })
  assert.deepEqual(countdown(reset, +now + 1000), { seconds: 119, minutes: 1, hours: 0, days: 0 })
  assert.equal(countdown(reset, +now + 120001).seconds, 0)
  assert.equal(countdown(null, +now), null)
})

test('rejects invalid records and fields while preserving valid siblings', () => {
  assert.equal(parse('{"schemaVersion":1,', 'broken'), null)
  assert.equal(parse('{"schemaVersion":2,"id":"broken"}', 'broken'), null)
  assert.equal(parse('{"schemaVersion":1,"id":"different"}', 'broken'), null)
  assert.equal(parse('x'.repeat(2 * 1024 * 1024 + 1), 'large'), null)
  const record = parse(JSON.stringify({
    ...fixtures.codex,
    limits: [null, [], { percent: null }, { percent: '0.5' }, fixtures.codex.limits[0]],
    accounts: [{ label: {} }],
    balance: { remaining: -1 }
  }), 'codex').record
  assert.equal(record.limits.length, 1)
  assert.equal(record.balance, null)
  assert.deepEqual(record.accounts, [])
})

test('decodes Omarchy file discovery output', () => {
  assert.deepEqual(Object.entries(listing(['codex.json', '10', '../invalid.json', '12', 'large.json', '2097153', ''].join('\0'))),
    [['codex', true], ['large', false]])
})
